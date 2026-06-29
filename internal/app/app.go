// Package app adapte le cœur MO5 au desktop via Ebitengine.
// Il est le seul package autorisé à importer Ebitengine.
//
// L'émulation tourne dans une goroutine dédiée (internal/emu.Host) ; l'UI ne
// fait que publier les entrées (Update), lire un instantané du framebuffer
// (Draw) et envoyer des commandes média. Le cœur n'est jamais touché
// directement depuis l'UI : pas de verrou partagé, UI réactive.
package app

import (
	"errors"
	"fmt"
	"image/color"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/atotto/clipboard"

	"github.com/Lesur-ai/dcmo5/internal/emu"
	"github.com/Lesur-ai/dcmo5/internal/keyboard"
	"github.com/Lesur-ai/dcmo5/internal/machine"
	"github.com/Lesur-ai/dcmo5/internal/media"
	"github.com/Lesur-ai/dcmo5/internal/media/impl"
	"github.com/Lesur-ai/dcmo5/internal/overlay"
	"github.com/Lesur-ai/dcmo5/internal/uimodel"
	"github.com/hajimehoshi/ebiten/v2"
	ebaudio "github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// ErrUserQuit est retourné par Run quand l'utilisateur ferme la fenêtre.
var ErrUserQuit = errors.New("quit")

// liveKey associe une touche physique à une touche MO5 apprise (avec son besoin
// de SHIFT MO5 et le caractère source, déduits du caractère décodé par l'OS).
type liveKey struct {
	mo5   int
	shift bool
	r     rune // caractère appris (sert à exclure les répétitions OS des touches tenues)
}

// mo5KeyACC est la touche ACC (accent, AltGr) du MO5. Référencée par les tests ;
// la résolution des touches s'appuie sur le modèle clavier (Model.IsModifier).
const mo5KeyACC = 0x36

const windowTitle = "DCMO5 Moderne"

// App implémente ebiten.Game et orchestre l'UI autour d'un emu.Host.
type App struct {
	host     *emu.Host
	fb       *ebiten.Image
	fbPixels []uint32       // tampon framebuffer réutilisé (anti-alloc/GC)
	fbBytes  []byte         // tampon RGBA réutilisé pour WritePixels
	fw, fh   int            // dimensions du framebuffer (fixées par la machine via FrameSize())
	family   machine.Family // famille de la machine attachée : pilote la géométrie d'affichage (Layout/fenêtre/curseur) via uimodel.DisplayGeometry

	// currentProfile : profil STATIQUE de la machine réellement attachée (schéma de
	// Params). Source unique pour DescribeLive/DiffLive de l'overlay (lot #117 Inc 3b).
	// family en est dérivée (currentProfile.Family) → pas de divergence possible. La
	// config live n'est PAS stockée : CurrentConfig() la DÉRIVE de l'état monté (closers/
	// noms), seule source de vérité vivante, pour ne jamais afficher de média fantôme.
	currentProfile machine.MachineProfile

	// Saisie clavier
	keys       *keyboard.Injector
	kbModel    *keyboard.Model // modèle clavier de la machine (data-driven)
	inputChars []rune

	// Touches-caractères « tenues » en live : on apprend l'association touche
	// physique → touche MO5 depuis le caractère décodé par l'OS (layout-safe),
	// puis on tient la touche MO5 tant que la physique est enfoncée (jeux +
	// répétition). L'injecteur (keys) ne sert plus qu'à --exec/collage.
	liveKeys    map[ebiten.Key]liveKey
	justPressed []ebiten.Key

	// Saisie programmée (--exec, coller). execSeq attend la fin du délai de boot,
	// puis alimente typeAhead, lui-même vidé progressivement vers l'injecteur
	// (sans dépasser sa file bornée, donc sans perdre le début d'un long script).
	execSeq         string
	execDelayFrames int
	typeAhead       []rune

	// Launcher (lot #117, PR-C2) : écran de sélection machine + paramètres rendu
	// avec ebitenui. Non-nil ⇒ mode launcher (host==nil, aucune émulation). À
	// l'action « Démarrer », l'App instancie la machine, monte les médias, démarre
	// le Host puis repasse launcher=nil (mode émulateur).
	launcher           *launcher
	onStart            func(profileID string, cfg machine.Config) // hook de persistance config à l'action « Démarrer » (mode launcher)
	onJoystickKBChange func(enabled bool)                         // hook de persistance du toggle joystick clavier (B9)
	hostStarted        bool                                       // host.Start() a été appelé (garde le Stop différé)

	// overlay : machine d'état PURE de l'overlay Échap (zéro-value = fermé, aucun
	// constructeur). Ouvert/fermé par Échap (cf. Update). overlayUI est l'arbre ebitenui,
	// construit paresseusement à la 1ʳᵉ ouverture (besoin du profil) ; nil tant que
	// l'overlay n'a jamais été ouvert.
	overlay   overlay.Model
	overlayUI *overlayUI
	mediaDir  string // répertoire de départ du navigateur de fichiers

	// Inc J3a : joystick au clavier activable à la demande (F12). Désactivé par
	// défaut (false) → WASD/AltGr tapent normalement en BASIC. Activé → mapping
	// joystick (J1 = flèches+AltGr, J2 = WASD+LeftShift) exclut WASD et AltGr
	// du clavier émulation (cf. joystick.go isJoystickExclusiveKey conditionnel).
	joystickKBEnabled bool

	// Inc J4b : gamepads matériels en standard layout, max 2 simultanés (J1 / J2).
	// Slot management par ordre de connexion (D8 plan workflow joystick). Le
	// buffer connectBuf est réutilisé entre frames pour éviter les allocs (le
	// nombre de gamepads connectés par frame est typiquement 0 ou 1).
	gamepadSlots      gamepadSlots
	gamepadConnectBuf []ebiten.GamepadID

	// Médias montés : Closer des fichiers ouverts (fermeture à l'éjection/remplacement)
	tapeCloser io.Closer
	diskCloser io.Closer

	// Audio (le lecteur consomme la ring du Host ; il ne touche jamais le cœur).
	// audioCtx est créé UNE SEULE FOIS (ebiten n'autorise qu'un contexte par process :
	// un 2e ebaudio.NewContext panique). Au changement de machine, on recrée le Player
	// (lié à la ring d'un Host précis) mais PAS le contexte (cf. teardownAudio/initAudio).
	audioCtx      *ebaudio.Context
	audioPlayer   *ebaudio.Player
	audioDisabled bool

	// romResolver résout le chemin de la ROM système d'une machine par son identifiant
	// (lecture de la config persistée, injectée depuis cmd via SetROMResolver). Sert au
	// changement de machine à chaud (Inc 5) : construire la config de la cible. nil = pas
	// de résolution (le switch échouera proprement sur ROM requise).
	romResolver func(machineID string) string

	// État desktop
	paused     bool
	romMissing bool
	romName    string
	tapeName   string
	diskName   string
	cartName   string
}

// New crée une application pilotant la machine donnée via un emu.Host (mode
// émulateur, chemin CLI à boot direct). Les tampons d'affichage sont dimensionnés
// selon FrameSize() de la machine (fixe par instance) : l'App est agnostique du
// modèle émulé. profile est le profil STATIQUE de la machine (résolu par l'appelant
// via machine.ByID) : il porte la famille (géométrie d'affichage) ET le schéma de
// Params consommé par l'overlay — l'interface machine.Machine runtime ne porte pas
// cette identité statique, d'où le passage explicite du profil.
func New(m machine.Machine, profile machine.MachineProfile) *App {
	a := &App{mediaDir: startMediaDir(os.Getwd, os.UserHomeDir)}
	a.attachMachine(m, profile)
	return a
}

// NewLauncher crée une application en MODE LAUNCHER (host==nil) : elle affiche
// l'écran de sélection de machine + paramètres (rendu ebitenui, data-driven via
// uimodel) au lieu d'émuler. La machine est instanciée à l'action « Démarrer »
// (cf. updateLauncher). profiles est la liste proposée (machine.Profiles(), plus
// éventuellement un profil de démonstration) ; initial pré-remplit les valeurs du
// profil présélectionné selected (cf. --machine, résolu par launch.SelectIndex ; ex.
// chemin ROM mémorisé en config). noAudio diffère/inhibe l'audio.
func NewLauncher(profiles []machine.MachineProfile, mediaDir string, noAudio bool, initial machine.Config, selected int) *App {
	a := &App{mediaDir: mediaDir, audioDisabled: noAudio}
	a.launcher = newLauncher(profiles, mediaDir, osListerUI, initial, selected)
	return a
}

// SetOnStart enregistre un hook appelé avec l'ID du profil sélectionné et la config
// validée au moment où l'utilisateur lance une machine depuis le launcher (transition
// launcher→émulateur). L'ID permet à la couche cmd de persister le choix PAR machine
// (ex. chemin ROM) sans coupler l'App au package config. Sans effet en mode émulateur
// (chemin CLI à boot direct).
func (a *App) SetOnStart(fn func(profileID string, cfg machine.Config)) { a.onStart = fn }

// SetROMResolver injecte le résolveur de ROM système par machine (lecture de la config
// persistée, côté cmd). Consommé par le changement de machine à chaud (Inc 5) pour
// construire la config de la cible. nil (défaut) → aucune résolution.
func (a *App) SetROMResolver(fn func(machineID string) string) { a.romResolver = fn }

// SetOnJoystickKBChange injecte un callback appelé chaque fois que le toggle
// joystick clavier change d'état (B9 : persistance globale). Le callback reçoit
// le nouvel état (true = activé). nil (défaut) → pas de persistance.
func (a *App) SetOnJoystickKBChange(fn func(enabled bool)) { a.onJoystickKBChange = fn }

// SetJoystickKBEnabled fixe l'état initial du toggle joystick clavier, typiquement
// depuis la config persistante au démarrage (B9). Sans appel, le toggle est false
// (désactivé par défaut).
func (a *App) SetJoystickKBEnabled(b bool) { a.joystickKBEnabled = b }

// attachMachine câble une machine sur l'App (tampons d'affichage, Host, modèle
// clavier). Partagé par New (CLI direct) et par la transition launcher→émulateur :
// c'est le SITE UNIQUE qui mémorise le couple (profil, famille), si bien que la famille
// ne peut jamais contredire le profil (elle en est dérivée). N'appelle PAS host.Start() :
// le démarrage (et le montage des médias) reste à la charge de l'appelant, qui doit
// monter les médias AVANT Start().
func (a *App) attachMachine(m machine.Machine, profile machine.MachineProfile) {
	fw, fh := m.FrameSize()
	kbModel := m.KeyboardModel()
	fb := ebiten.NewImage(fw, fh)
	fb.Fill(color.RGBA{R: 0, G: 0, B: 0, A: 0xFF})
	a.host = emu.New(m, defaultAudioGain)
	a.fb = fb
	a.fw, a.fh = fw, fh
	a.currentProfile = profile
	a.family = profile.Family // famille DÉRIVÉE du profil : source unique, pas de divergence
	a.fbPixels = make([]uint32, fw*fh)
	a.fbBytes = make([]byte, fw*fh*4)
	a.keys = keyboard.NewInjector(kbModel, keyboard.DefaultHoldFrames, keyboard.DefaultGapFrames)
	a.kbModel = kbModel
	a.liveKeys = make(map[ebiten.Key]liveKey)
}

// startMediaDir choisit le répertoire de départ du navigateur de fichiers (overlay) : le
// répertoire de travail courant en priorité (intuitif quand on lance le binaire
// depuis le dossier du projet, où vivent rom/ et software/), avec repli sur le
// répertoire personnel puis « . ». Les sources (getwd/home) sont injectées pour
// rester testable sans dépendre de l'environnement réel.
func startMediaDir(getwd, home func() (string, error)) string {
	if wd, err := getwd(); err == nil && wd != "" {
		return wd
	}
	if h, err := home(); err == nil && h != "" {
		return h
	}
	return "."
}

// SetROMStatus indique si la ROM est absente (affichage d'avertissement).
func (a *App) SetROMStatus(missing bool) { a.romMissing = missing }

// SetMediaNames configure les noms de médias montés pour le titre fenêtre.
func (a *App) SetMediaNames(rom, tape, disk, cart string) {
	a.romName = filepath.Base(rom)
	a.tapeName = filepath.Base(tape)
	a.diskName = filepath.Base(disk)
	a.cartName = filepath.Base(cart)
}

// SetExec programme une séquence de touches tapée automatiquement après
// delaySeconds (le temps que la ROM atteigne l'invite BASIC). Les « \n » de la
// séquence ont déjà été convertis en retours-chariot par l'appelant.
func (a *App) SetExec(seq string, delaySeconds float64) {
	a.execSeq = seq
	a.execDelayFrames = int(delaySeconds * 60) // 60 ticks/s
}

// SetStartupMediaClosers confie à l'App les descripteurs des médias ouverts au
// démarrage (CLI), pour qu'ils soient fermés proprement si on les remplace
// depuis l'overlay (évite une fuite du descripteur initial). nil est accepté.
func (a *App) SetStartupMediaClosers(tape, disk io.Closer) {
	a.tapeCloser = tape
	a.diskCloser = disk
}

// Update est appelé à chaque tick (60 Hz) : il publie les entrées vers le Host
// et pilote l'overlay. L'émulation, elle, avance dans la goroutine du Host.
func (a *App) Update() error {
	// Mode launcher : aucune émulation (host==nil). On rend/anime l'UI ebitenui et,
	// si l'utilisateur a validé « Démarrer », on instancie la machine. Branché TOUT
	// EN HAUT, avant tout accès à overlay/host/keys (qui sont nil/inactifs ici).
	if a.launcher != nil {
		return a.updateLauncher()
	}

	// Inc J4b (codex P3) : réconciliation des slots gamepad EN HAUT de chaque
	// Update, AVANT toute lecture des boutons (Start/Menu pour overlay) et la
	// composition joystick. Sans cette position en tête, un gamepad branché
	// au même tick où l'utilisateur presse Start ne serait pas reconnu (slot
	// pas encore attribué). Couvre aussi le cas overlay ouvert : si une manette
	// est branchée pendant que l'overlay est affiché, son Start sera détecté
	// dès cette frame.
	a.gamepadConnectBuf = a.updateGamepadSlots(a.gamepadConnectBuf)

	// OVERLAY OUVERT : capture STRICTE. Branché TOUT EN HAUT, return immédiat → aucune
	// entrée (Échap, F3, F5, collage, touches live, crayon) n'atteint le cœur tant
	// que l'overlay est ouvert. La contrainte #3 (revue Codex) est satisfaite par la
	// STRUCTURE (ce return), pas par des gardes éparpillés.
	if a.overlay.IsOpen() {
		if err := a.updateOverlay(); err != nil {
			return err
		}
		if !a.overlay.IsOpen() {
			// L'overlay vient de se fermer (Échap/Reset/Init prog/Appliquer) : PURGER l'état
			// d'entrée avant de reprendre. Sinon le Host garderait l'InputState d'AVANT
			// l'ouverture pendant ~1 frame (une touche relâchée pendant l'overlay resterait
			// « pressée »). Purge = tout relâché : jamais de touche fantôme ; une touche
			// réellement tenue se ré-affirme au tick suivant (republication normale).
			a.host.SetInput(emu.InputState{Joystick: machine.NeutralJoystick})
		}
		a.syncPause() // une action (fermeture) a pu changer l'état
		return nil
	}

	// ÉCHAP OUVRE l'overlay. L'overlay est forcément FERMÉ ici : s'il était ouvert, le
	// court-circuit en haut de Update aurait rendu la main (c'est updateOverlay qui gère
	// Échap pour remonter/fermer). On construit/rafraîchit l'arbre ebitenui sur l'état média
	// courant et on gèle l'émulation AU MÊME TICK que l'ouverture (sinon une frame avance).
	// Inc J4b (B6 plan workflow joystick) : le bouton Start/Menu du gamepad
	// (StandardGamepadButtonCenterRight) ouvre AUSSI l'overlay — sinon un user
	// en gamepad seul ne pourrait pas accéder à Reset/Init prog/Quitter/Changer
	// machine/Joystick. On scrute les deux slots gamepad attribués.
	if inputJustPressed(ebiten.KeyEscape) || a.gamepadStartJustPressed() {
		a.overlay.Open()
		a.openOverlayUI()
		a.syncPause()
		a.updateTitle()
		return nil // overlay ouvert : capture STRICTE dès ce tick (aucune entrée vers le cœur)
	}

	// F5 = reset machine
	if inputJustPressed(ebiten.KeyF5) {
		a.host.Reset()
	}

	// F3 = pause / resume (KeyP est la touche MO5 P=0x1C, on évite le conflit)
	if inputJustPressed(ebiten.KeyF3) {
		a.paused = !a.paused
		a.syncPause()
		a.updateTitle()
	}
	if a.paused {
		return nil
	}

	// Saisie programmée : après le délai de boot, la séquence --exec rejoint le
	// tampon typeAhead, vidé progressivement vers l'injecteur ci-dessous.
	if a.execSeq != "" {
		if a.execDelayFrames > 0 {
			a.execDelayFrames--
		} else {
			a.queueTypeAhead(a.execSeq)
			a.execSeq = ""
		}
	}
	// Coller : Cmd+V (macOS) ou Ctrl+V → taper le presse-papier dans le MO5.
	if pasteRequested() {
		if text, err := clipboard.ReadAll(); err == nil && text != "" {
			a.queueTypeAhead(text)
		}
	}
	a.feedTypeAhead()

	// Saisie clavier MO5. Les touches-caractères sont « tenues » en live :
	// apprentissage layout-safe (touche physique → touche MO5 via le caractère
	// décodé par l'OS), puis maintien tant que la physique est enfoncée → jeux
	// utilisables + répétition gérée par la ROM MO5. L'injecteur ne sert plus
	// qu'à la saisie scriptée (--exec, collage).
	a.inputChars = ebiten.AppendInputChars(a.inputChars[:0])
	a.justPressed = inpututil.AppendJustPressedKeys(a.justPressed[:0])
	// Inc J3a : le toggle joystick au clavier vit dans l'overlay (rangée Système,
	// bouton « Joystick : ON/OFF »). OFF par défaut → WASD tapent en BASIC
	// normalement. ON → mapping joystick (J1=flèches+RightShift, J2=WASD+
	// LeftShift) avec exclusion WASD du clavier émulation. Le toggle ne
	// déclenche AUCUN effet de bord côté machine — juste un changement de
	// routage des touches dans les frames suivantes.

	learnLiveKeys(a.kbModel, a.liveKeys, a.justPressed, a.inputChars, ebiten.IsKeyPressed, a.joystickKBEnabled)

	tickKeys := a.keys.Tick()
	injecting := len(tickKeys) > 0 || a.keys.Pending() > 0

	in := resolveKeys(a.kbModel, ebiten.IsKeyPressed, a.liveKeys, injecting, tickKeys, a.joystickKBEnabled)
	// Inc J3a : résolution joystick clavier. Mapping fixe (J1=flèches+RightShift,
	// J2=WASD+LeftShift) défini dans joystick.go. Si le mode est désactivé
	// (défaut), retourne machine.NeutralJoystick — état neutre côté machine.
	keyboardJoy := joystickFromKeys(ebiten.IsKeyPressed, a.joystickKBEnabled)
	// Inc J4b : composition avec les gamepads matériels (max 2 simultanés, slot
	// par ordre de connexion). Hot-plug détecté à chaque tick via réconciliation
	// (updateGamepadSlots appelée en tête d'Update). Un gamepad déconnecté libère
	// son slot et retombe sur NeutralJoystick par construction.
	gamepadJoy := a.joystickFromGamepads()
	in.Joystick = uimodel.MergeJoysticks(keyboardJoy, gamepadJoy)
	// D11 (audit Codex 28/06) : quand la fenêtre Ebitengine perd le focus, forcer
	// NeutralJoystick. Sans cette garde, une direction tenue au moment de l'alt-tab
	// reste « collée » car l'événement de relâchement n'est jamais vu. Le clavier
	// émulation n'a PAS ce problème (learnLiveKeys/resolveKeys ne voient plus les
	// touches tenues → relâchement implicite à la frame suivante).
	if !ebiten.IsFocused() {
		in.Joystick = machine.NeutralJoystick
	}
	// Le curseur Ebitengine est en repère Layout (= LOGIQUE). Pour le crayon optique,
	// on le ramène au repère FRAMEBUFFER attendu par la machine : identité pour le MO5
	// (logique == framebuffer), mais Y/2 pour le gate-array dont le Layout est étiré ×2
	// en hauteur. Chaque machine convertit ensuite vers son propre repère écran dans
	// SetPointer (le MO5 y retranche sa bordure).
	cx, cy := ebiten.CursorPosition()
	in.PenX, in.PenY = uimodel.CursorToFramebuffer(a.family, a.fw, a.fh, cx, cy)
	in.PenDown = ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft)
	a.host.SetInput(in)
	return nil
}

// syncPause répercute l'état pause (F3) + ouverture de l'overlay sur le Host (suspend
// l'émulation) via la fonction pure overlay.ShouldPause (testée en CI).
func (a *App) syncPause() {
	a.host.SetPaused(overlay.ShouldPause(a.paused, a.overlay.IsOpen()))
}

// updateOverlay traite les entrées quand l'overlay est ouvert (clavier/souris UNIQUEMENT
// vers l'overlay : le court-circuit appelant garantit qu'aucune entrée n'atteint le cœur).
// Échap remonte d'un niveau (Browse→Main ; Main→ferme et ANNULE les éditions de next) ;
// sinon on délègue à ebitenui (focus clavier natif + clics) et on exécute les actions
// signalées par l'UI. L'overlayUI partage l'état overlay.Model avec l'App (même pointeur),
// donc Back()/GoMain()/GoBrowse() y sont reflétés ; un simple rebuild suffit.
func (a *App) updateOverlay() error {
	// Échap clavier OU Start gamepad : équivalents pour fermer/remonter d'un cran
	// l'overlay. Sans le Start gamepad ici, un utilisateur gamepad-only se
	// retrouverait piégé dans un overlay ouvert (Inc J4b codex review #178 P2).
	if inputJustPressed(ebiten.KeyEscape) || a.gamepadStartJustPressed() {
		a.overlay.Back()
		if !a.overlay.IsOpen() {
			a.updateTitle()
			return nil // fermé (annulation) : ne pas animer l'UI ce tick (elle disparaît)
		}
		a.overlayUI.rebuild() // changement de vue (Browse→Main)
	}
	a.overlayUI.ui.Update() // focus clavier natif (Tab/flèches/Entrée) + clics

	// Actions signalées par l'UI (pattern takeStart du launcher) : exécutées ICI (impur),
	// décidées LÀ-BAS (clic). Reset/Initprog = commandes Host idempotentes traitées même en
	// pause. Appliquer = montage/éjection des médias édités dans next (cf. applyLiveOps).
	if a.overlayUI.quit {
		return ErrUserQuit
	}
	// Reset / Init prog : commande Host PUIS fermeture de l'overlay (reprise). Sans la
	// fermeture, l'overlay reste ouvert = émulation gelée : la commande est bien appliquée
	// (le Host traite les commandes même en pause) mais le framebuffer gelé ne reflète pas
	// le redémarrage tant qu'on ne reprend pas — l'effet semblait alors différé à la sortie.
	// On ferme donc (parité menu v1, qui fait host.Reset()+menu.Close()) : l'effet est
	// immédiatement visible. Les éditions média en cours (next) sont abandonnées, comme pour
	// une annulation — Reset/Init prog sont une intention distincte du montage de médias.
	if a.overlayUI.takeReset() {
		a.host.Reset()
		a.overlay.Close()
		a.updateTitle()
		return nil
	}
	if a.overlayUI.takeInitprog() {
		a.host.Initprog()
		a.overlay.Close()
		a.updateTitle()
		return nil
	}
	if a.overlayUI.takeApply() {
		a.applyLiveOps(a.overlayUI.next)
	}
	if target, ok := a.overlayUI.takeSwitch(); ok {
		a.switchMachine(target)
		// switchMachine met a.overlayUI = nil (pattern Inc 5b : éviter de projeter
		// le profil/next de l'ancienne machine sur la nouvelle). Return immédiat —
		// sinon le takeToggleJoystick ci-dessous déréfère un overlayUI nil.
		return nil
	}
	if a.overlayUI.takeToggleJoystick() {
		// Inc J3a : toggle joystick clavier. Pas d'effet immédiat côté machine
		// (le state suit dans Update via joystickFromKeys), mais on rafraîchit
		// le bouton de l'overlay pour que son libellé reflète l'état courant.
		a.joystickKBEnabled = !a.joystickKBEnabled
		a.overlayUI.setJoystickKBEnabled(a.joystickKBEnabled)
		if a.onJoystickKBChange != nil {
			a.onJoystickKBChange(a.joystickKBEnabled)
		}
	}
	return nil
}

// switchMachine bascule à chaud vers la machine cible (depuis la vue ConfirmSwitch).
// Validation PURE d'abord (PrepareSwitch) : si elle échoue (ROM cible introuvable), on
// AFFICHE l'erreur et la session courante reste 100% intacte. Sinon on exécute le switch.
func (a *App) switchMachine(target machine.MachineProfile) {
	persisted := overlay.SwitchPersisted(target, a.romResolver)
	prep, err := overlay.PrepareSwitch(target, persisted, fileExists)
	if err != nil {
		a.overlayUI.errText = "Passage à " + target.Name + " impossible : " + err.Error()
		a.overlayUI.rebuild()
		return
	}
	if err := a.applyProfileSwitch(target, prep); err != nil {
		a.overlayUI.errText = "Démarrage de " + target.Name + " échoué : " + err.Error()
		a.overlayUI.rebuild()
		return
	}
	// Succès : fermer l'overlay, reprendre, et JETER l'overlayUI — il portait le profil et la
	// config de travail de l'ANCIENNE machine ; il sera recréé à la prochaine ouverture sur
	// la nouvelle (openOverlayUI), évitant tout média/param fantôme (revue Codex).
	a.overlay.Close()
	a.overlayUI = nil
	a.updateTitle()
	a.syncPause()
}

// applyProfileSwitch exécute l'ordre IMPÉRATIF du changement de machine (revue de plan
// Codex, B2). Doctrine state-safety : on instancie la NOUVELLE machine AVANT d'arrêter
// l'ancienne — si New échoue (ROM illisible/corrompue), la session courante est intacte.
// Une fois New réussi, on s'engage : Stop ancien Host (bloquant) → teardown médias+audio →
// attachMachine → monter médias → fenêtre → recréer le Player audio → Start.
func (a *App) applyProfileSwitch(target machine.MachineProfile, prep overlay.Prep) error {
	m, err := target.New(prep.Config)
	if err != nil {
		return err // rien arrêté : session courante intacte
	}
	a.host.Stop() // bloquant (close(stop)+<-done) : la goroutine de l'ancienne machine est terminée
	a.closeTape() // fermer les descripteurs de l'ancienne machine (sinon fuite)
	a.closeDisk() //
	a.tapeName, a.diskName, a.cartName = "", "", ""
	a.teardownAudio() // détruit le Player (lié à l'ancienne ring) APRÈS Stop, AVANT initAudio
	a.attachMachine(m, target)
	a.romName = ""
	if rom, _ := prep.Config[machine.KeyROM].(string); rom != "" {
		a.romName = filepath.Base(rom)
	}
	a.mountMedia(prep.Mounts) // médias à monter (vide par défaut : familles incompatibles, cf. B3)
	a.applyWindowSize()       // redimensionne la fenêtre selon la nouvelle famille
	a.initAudio()             // recrée le Player sur le NOUVEAU Host (audioPlayer == nil après teardown)
	a.host.Start()
	a.hostStarted = true
	return nil
}

// openOverlayUI construit (paresseusement) l'arbre ebitenui de l'overlay puis le
// synchronise sur l'état média RÉELLEMENT monté (CurrentConfig, dérivé). Appelé à chaque
// ouverture pour refléter d'éventuels changements de média survenus entre deux ouvertures.
func (a *App) openOverlayUI() {
	if a.overlayUI == nil {
		a.overlayUI = newOverlayUI(a.currentProfile, machine.Profiles(), &a.overlay, osListerUI, newUIKit())
	}
	a.overlayUI.open(a.currentProfile, a.mediaDir, a.CurrentConfig(), a.joystickKBEnabled)
}

// applyLiveOps applique les changements média de `next` vs l'état RÉELLEMENT monté
// (a.CurrentConfig(), JAMAIS next). La DÉCISION est pure (uimodel.LiveMediaOps, testée CI) ;
// seule l'exécution est ici. Contrainte #5 (revue Codex) : une erreur de montage ou une
// clé non applicable est SIGNALÉE (errText) et l'overlay reste ouvert ; on ne met à jour ni
// nom ni titre comme si ça avait réussi. Succès complet → fermeture + reprise (syncPause).
func (a *App) applyLiveOps(next machine.Config) {
	var errs []string
	for _, op := range uimodel.LiveMediaOps(a.currentProfile, a.CurrentConfig(), next) {
		switch op.Kind {
		case uimodel.OpMount:
			if err := a.mountLive(op.Key, op.Path); err != nil {
				errs = append(errs, op.Key+" : "+err.Error()) // NE touche ni nom ni titre
			}
		case uimodel.OpEject:
			a.ejectLive(op.Key)
		case uimodel.OpUnsupported:
			errs = append(errs, op.Key+" : réglage non applicable à chaud")
		}
	}
	a.updateTitle()
	if len(errs) == 0 {
		a.overlay.Close()
		a.updateTitle()
		return
	}
	// Échec partiel : re-synchroniser next sur ce qui est RÉELLEMENT monté (les montages
	// réussis sont reflétés, les échecs gardent l'ancien média), afficher l'erreur, rester ouvert.
	a.overlayUI.next = cloneConfig(a.CurrentConfig())
	a.overlayUI.errText = strings.Join(errs, "\n")
	a.overlayUI.rebuild()
}

// mountLive ouvre et monte un média à chaud, par clé conventionnelle. Sur erreur
// d'ouverture, NE touche RIEN (ni closer ni nom ni mediaDir) → l'état reste cohérent
// (contrainte #5). Extrait/parallèle de mountChosen (menu v1, retiré en 3b.4d).
func (a *App) mountLive(key, path string) error {
	switch key {
	case machine.KeyTape:
		t, err := impl.OpenTape(path, false)
		if err != nil {
			return err
		}
		a.closeTape()
		a.host.MountTape(t)
		a.tapeCloser = t
		a.tapeName = filepath.Base(path)
		a.mediaDir = filepath.Dir(path)
	case machine.KeyDisk:
		d, err := impl.OpenDisk(path, false)
		if err != nil {
			return err
		}
		a.closeDisk()
		a.host.MountDisk(d)
		a.diskCloser = d
		a.diskName = filepath.Base(path)
		a.mediaDir = filepath.Dir(path)
	case machine.KeyCart:
		c, err := impl.OpenCartridge(path)
		if err != nil {
			return err
		}
		a.host.MountCartridge(c)
		a.cartName = filepath.Base(path)
		a.mediaDir = filepath.Dir(path)
	default:
		return fmt.Errorf("clé média inconnue : %s", key)
	}
	return nil
}

// ejectLive éjecte un média à chaud, par clé conventionnelle (parallèle des branches
// d'éjection de handleMenuAction du menu v1, retiré en 3b.4d).
func (a *App) ejectLive(key string) {
	switch key {
	case machine.KeyTape:
		a.host.EjectTape()
		a.closeTape()
		a.tapeName = ""
	case machine.KeyDisk:
		a.host.EjectDisk()
		a.closeDisk()
		a.diskName = ""
	case machine.KeyCart:
		a.host.EjectCartridge()
		a.cartName = ""
	}
}

// typeAheadHighWater : on ne remplit la file de l'injecteur que jusqu'à ce
// niveau, sous sa borne (keyboard.DefaultQueueMax), pour ne jamais en perdre le
// début. Le reste attend dans typeAhead et est injecté au fil du jeu.
const typeAheadHighWater = 200

// queueTypeAhead ajoute une séquence à taper (--exec ou coller), en normalisant
// les fins de ligne (\r\n et \r → \n = ENT).
func (a *App) queueTypeAhead(s string) {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	a.typeAhead = append(a.typeAhead, []rune(s)...)
}

// feedTypeAhead déverse le tampon de saisie programmée dans l'injecteur sans
// dépasser sa file bornée (évite que le début d'un long script soit abandonné).
func (a *App) feedTypeAhead() {
	for len(a.typeAhead) > 0 && a.keys.Pending() < typeAheadHighWater {
		a.keys.Enqueue(a.typeAhead[0])
		a.typeAhead = a.typeAhead[1:]
	}
}

// closeTape / closeDisk ferment le fichier média courant s'il y en a un.
func (a *App) closeTape() {
	if a.tapeCloser != nil {
		a.tapeCloser.Close()
		a.tapeCloser = nil
	}
}

func (a *App) closeDisk() {
	if a.diskCloser != nil {
		a.diskCloser.Close()
		a.diskCloser = nil
	}
}

// CurrentProfile retourne le profil de la machine actuellement attachée — source du
// schéma (Params) pour DescribeLive/DiffLive de l'overlay (lot #117 Inc 3b). Renvoyé par
// valeur ; le slice Params est partagé en lecture seule (l'overlay ne le mute pas).
func (a *App) CurrentProfile() machine.MachineProfile { return a.currentProfile }

// CurrentConfig DÉRIVE — à la demande, jamais stockée — la configuration des médias
// modifiables à chaud RÉELLEMENT montés, base `old` que l'overlay passe à DescribeLive/
// DiffLive. La source de vérité est l'état vivant des médias (closers pour tape/disk, nom
// pour la cartouche qui n'a pas de closer), maintenu par TOUS les chemins (boot CLI,
// launcher, overlay) : aucune config parallèle à resynchroniser, aucun média fantôme
// possible après une éjection/montage. Ne porte que les Params LiveMutable File (cf.
// uimodel.LiveMediaConfig) ; les clés boot-only (rom) sont hors overlay, donc absentes.
func (a *App) CurrentConfig() machine.Config {
	mounted := map[string]string{}
	if a.tapeCloser != nil {
		mounted[machine.KeyTape] = a.tapeName
	}
	if a.diskCloser != nil {
		mounted[machine.KeyDisk] = a.diskName
	}
	// La cartouche n'a pas de closer : son nom est le seul témoin de montage. Garde le
	// même idiome qu'updateTitle (`!= "" && != "."`) car SetMediaNames stocke
	// filepath.Base("") == "." quand aucun --cart au boot CLI : sans ce garde, une
	// cartouche fantôme {cart:"."} serait projetée.
	if a.cartName != "" && a.cartName != "." {
		mounted[machine.KeyCart] = a.cartName
	}
	return uimodel.LiveMediaConfig(a.currentProfile, mounted)
}

// updateLauncher anime l'UI du launcher et, à l'action « Démarrer », instancie la
// machine puis bascule en mode émulateur. L'ordre est IMPÉRATIF (revue de plan
// Codex, P1) : attacher la machine → MONTER les médias AVANT host.Start() (un
// montage après Start passe par le canal de commandes et pourrait laisser le boot
// démarrer sans média) → fixer la taille fenêtre sur le framebuffer → initialiser
// l'audio → démarrer le Host.
func (a *App) updateLauncher() error {
	// ÉCHAP (non géré par ebitenui) : en navigateur de fichiers → annule (retour vue
	// principale) ; en vue principale → quitte l'application.
	if inputJustPressed(ebiten.KeyEscape) && !a.launcher.escapePressed() {
		return ErrUserQuit
	}
	a.launcher.ui.Update()
	req, ok := a.launcher.takeStart()
	if !ok {
		return nil
	}
	cfg, err := uimodel.BuildConfig(req.profile, req.values)
	if err != nil {
		a.launcher.setError(err)
		return nil
	}
	// Auto-détection de la ROM contrôleur cd90-640 à côté de la ROM choisie (miroir du
	// boot CLI) : sinon un disque .fd lancé depuis le launcher démarrerait sans contrôleur
	// (DOS inopérant), contrairement à « dcmo5 --rom … --disk … ». N'écrase pas une
	// disk-rom fournie explicitement.
	if dr := uimodel.ResolveDiskROM(cfg, fileExists); dr != "" {
		cfg[machine.KeyDiskROM] = dr
	}
	m, err := req.profile.New(cfg)
	if err != nil {
		a.launcher.setError(err)
		return nil
	}
	a.attachMachine(m, req.profile)
	if rom, _ := cfg[machine.KeyROM].(string); rom != "" {
		a.romName = filepath.Base(rom)
	}
	if a.onStart != nil {
		a.onStart(req.profile.ID, cfg) // persistance config PAR machine (ex. ROM mémorisée) côté cmd
	}
	a.mountMedia(uimodel.MediaMounts(req.profile, cfg))
	a.applyWindowSize()
	a.initAudio()
	a.host.Start()
	a.hostStarted = true
	a.launcher = nil // → mode émulateur
	a.updateTitle()
	return nil
}

// mountMedia ouvre et monte (à chaud, AVANT host.Start) les médias choisis dans le
// launcher, en traduisant chaque clé de paramètre en appel de montage typé. Un
// fichier illisible est ignoré (l'émulation démarre sans ce média).
func (a *App) mountMedia(mounts []uimodel.MediaMount) {
	for _, mt := range mounts {
		switch mt.Key {
		case machine.KeyTape:
			if t, err := impl.OpenTape(mt.Path, false); err == nil {
				a.closeTape()
				a.host.MountTape(t)
				a.tapeCloser = t
				a.tapeName = filepath.Base(mt.Path)
				a.mediaDir = filepath.Dir(mt.Path)
			}
		case machine.KeyDisk:
			if d, err := impl.OpenDisk(mt.Path, false); err == nil {
				a.closeDisk()
				a.host.MountDisk(d)
				a.diskCloser = d
				a.diskName = filepath.Base(mt.Path)
				a.mediaDir = filepath.Dir(mt.Path)
			}
		case machine.KeyCart:
			if c, err := impl.OpenCartridge(mt.Path); err == nil {
				a.host.MountCartridge(c)
				a.cartName = filepath.Base(mt.Path)
				a.mediaDir = filepath.Dir(mt.Path)
			}
		}
	}
}

// compile-time : *impl types satisfont media + io.Closer (sécurité de typage).
var (
	_ media.Tape = (*impl.FileTape)(nil)
	_ io.Closer  = (*impl.FileTape)(nil)
)

// Draw rend l'instantané du framebuffer du Host dans la surface Ebitengine.
func (a *App) Draw(screen *ebiten.Image) {
	if a.launcher != nil {
		a.launcher.ui.Draw(screen)
		return
	}
	if a.romMissing {
		screen.Fill(color.RGBA{R: 20, G: 0, B: 0, A: 0xFF})
		return
	}
	a.blitFramebuffer() // dernier instantané → a.fb (gelé si l'émulation est en pause)

	// Overlay ouvert : on dessine le framebuffer GELÉ en aspect-fit + voile (l'UI
	// ebitenui viendra en 3b.2). Le repère écran est ici la fenêtre réelle (Layout a
	// basculé via EmulatorLayoutSize).
	if a.overlay.IsOpen() {
		a.drawOverlay(screen)
		return
	}

	// Rendu plein écran habituel : le framebuffer remplit le repère logique (Ebitengine
	// met ensuite à l'échelle de la fenêtre). MO5 : échelle 1 ; gate-array : ×2 en hauteur.
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(
		float64(screen.Bounds().Dx())/float64(a.fw),
		float64(screen.Bounds().Dy())/float64(a.fh),
	)
	screen.DrawImage(a.fb, op)
}

// blitFramebuffer recopie le dernier instantané du Host dans a.fb (pas d'accès au
// cœur). Quand l'émulation est en pause (overlay/F3), le Host ne publie plus :
// l'instantané est donc figé « gratuitement ». Partagé par les deux chemins de Draw.
func (a *App) blitFramebuffer() {
	a.host.Framebuffer(a.fbPixels)
	for i, px := range a.fbPixels {
		a.fbBytes[i*4+0] = byte(px)
		a.fbBytes[i*4+1] = byte(px >> 8)
		a.fbBytes[i*4+2] = byte(px >> 16)
		a.fbBytes[i*4+3] = byte(px >> 24)
	}
	a.fb.WritePixels(a.fbBytes)
}

// drawOverlay dessine, par-dessus l'écran, le framebuffer GELÉ centré en aspect-fit
// (uimodel.FramebufferAspectFit, pur), un voile sombre, puis l'UI ebitenui de l'overlay
// (vue Main en 3b.2 ; Browse en 3b.3). Le letterbox est rempli en noir.
func (a *App) drawOverlay(screen *ebiten.Image) {
	sw, sh := screen.Bounds().Dx(), screen.Bounds().Dy()
	screen.Fill(color.RGBA{R: 0, G: 0, B: 0, A: 0xFF}) // letterbox (barres) noir
	x, y, w, h := uimodel.FramebufferAspectFit(a.family, a.fw, a.fh, sw, sh)
	op := &ebiten.DrawImageOptions{}
	// Échelle PAR AXE : le gate-array étire ainsi ×2 en hauteur (672×216 → 672×432),
	// comme en plein écran — pas d'aplatissement.
	op.GeoM.Scale(float64(w)/float64(a.fw), float64(h)/float64(a.fh))
	op.GeoM.Translate(float64(x), float64(y))
	screen.DrawImage(a.fb, op)
	// Voile sombre semi-transparent : signale que l'émulation est gelée et fait ressortir
	// la carte de l'UI.
	vector.DrawFilledRect(screen, 0, 0, float32(sw), float32(sh), color.RGBA{R: 0, G: 0, B: 0, A: 120}, false)
	// UI ebitenui par-dessus (racine transparente : le framebuffer + voile restent visibles
	// autour de la carte). nil si l'overlay n'a jamais été ouvert (ne devrait pas arriver ici).
	if a.overlayUI != nil {
		a.overlayUI.ui.Draw(screen)
	}
}

// applyWindowSize dimensionne la fenêtre selon la géométrie d'affichage de la
// famille courante (cf. uimodel.DisplayGeometry). Partagé par Run (boot direct) et
// par la transition launcher→émulateur.
func (a *App) applyWindowSize() {
	_, _, winW, winH := uimodel.DisplayGeometry(a.family, a.fw, a.fh)
	ebiten.SetWindowSize(winW, winH)
}

// Layout retourne les dimensions LOGIQUES de l'écran. En mode launcher, on rend
// l'UI ebitenui à la résolution réelle de la fenêtre (outW,outH). En mode émulateur,
// on retourne le repère logique de la machine dérivé du framebuffer par famille
// (uimodel.DisplayGeometry) : identique au framebuffer pour le MO5, étiré ×2 en
// hauteur pour le gate-array (correction d'aspect). La transition launcher→émulateur
// force aussi SetWindowSize pour éviter un premier rendu mal échelonné dans la
// fenêtre du launcher.
func (a *App) Layout(outW, outH int) (int, int) {
	if a.launcher != nil {
		return outW, outH
	}
	// Overlay ouvert → repère FENÊTRE réel (outW,outH) pour un rendu ebitenui au pixel
	// près ; fermé → repère logique d'affichage de la famille (inchangé). Pur, testé CI.
	return uimodel.EmulatorLayoutSize(a.overlay.IsOpen(), a.family, a.fw, a.fh, outW, outH)
}

// updateTitle met à jour le titre de fenêtre selon l'état courant.
func (a *App) updateTitle() {
	title := windowTitle
	if a.romMissing {
		title += " — ROM manquante"
	} else if a.romName != "" && a.romName != "." {
		title += " — " + a.romName
		if a.tapeName != "" && a.tapeName != "." {
			title += " [K7:" + a.tapeName + "]"
		}
		if a.diskName != "" && a.diskName != "." {
			title += " [FD:" + a.diskName + "]"
		}
		if a.cartName != "" && a.cartName != "." {
			title += " [CART:" + a.cartName + "]"
		}
	}
	if a.overlay.IsOpen() {
		title += " [OVERLAY]"
	}
	if a.paused {
		title += " [PAUSE]"
	}
	ebiten.SetWindowTitle(title)
}

// Run configure et lance la boucle Ebitengine. En mode émulateur (CLI direct), il
// dimensionne la fenêtre sur le framebuffer, initialise l'audio et démarre le Host.
// En mode launcher, il dimensionne une fenêtre de launcher et NE démarre rien :
// l'audio et le Host sont mis en route à la transition « Démarrer » (updateLauncher).
func Run(a *App) error {
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	if a.launcher != nil {
		ebiten.SetWindowSize(launcherWidth, launcherHeight)
	} else {
		a.applyWindowSize()
		a.initAudio()  // après que main a pu désactiver l'audio (--no-audio)
		a.host.Start() // lance la goroutine d'émulation
		a.hostStarted = true
	}
	defer func() {
		if a.host != nil && a.hostStarted {
			a.host.Stop()
		}
	}()
	a.updateTitle()
	err := ebiten.RunGame(a)
	if errors.Is(err, ErrUserQuit) {
		return ErrUserQuit
	}
	return err
}

// KeyMapping expose la table positionnelle MO5 pour les tests existants.
// Retourne une COPIE convertie depuis keyboard.MO5Model().SpecialKeys (peuplée
// par keyboard_init.go). Le pluriel par machine se lit via a.kbModel.SpecialKeys
// directement dans resolveKeys/learnLiveKeys ; cette fonction est MO5-only
// pour préserver la sémantique des anciens tests.
func KeyMapping() map[ebiten.Key]int {
	sp := keyboard.MO5Model().SpecialKeys
	out := make(map[ebiten.Key]int, len(sp))
	for k, v := range sp {
		out[ebiten.Key(k)] = v
	}
	return out
}

// ── Helpers input ─────────────────────────────────────────────────────────────

// fileExists indique si un chemin existe (os.Stat sans erreur). Sert à l'auto-détection
// de la ROM contrôleur disquette au launcher (cf. updateLauncher, uimodel.ResolveDiskROM).
func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// inputJustPressed détecte une pression nouvelle (non maintenue) via inpututil, dont
// l'état est rafraîchi par ebiten à CHAQUE frame, quel que soit l'appelant. Robustesse
// clé pour l'overlay (revue Codex) : quand un chemin cesse de consulter une touche
// pendant plusieurs frames (overlay capturant les entrées), une touche tenue à travers
// l'overlay n'est PAS re-déclenchée à la reprise — contrairement à un tracker maison mis
// à jour seulement à l'appel, qui devenait périmé.
func inputJustPressed(k ebiten.Key) bool {
	return inpututil.IsKeyJustPressed(k)
}

// pasteRequested détecte le raccourci « coller » : V vient d'être pressé avec
// Cmd (macOS) ou Ctrl maintenu.
func pasteRequested() bool {
	if !inputJustPressed(ebiten.KeyV) {
		return false
	}
	return ebiten.IsKeyPressed(ebiten.KeyMetaLeft) || ebiten.IsKeyPressed(ebiten.KeyMetaRight) ||
		ebiten.IsKeyPressed(ebiten.KeyControlLeft) || ebiten.IsKeyPressed(ebiten.KeyControlRight)
}

// learnLiveKeys apprend l'association touche physique → touche MO5 à partir des
// caractères décodés par l'OS cette frame (layout-safe). On n'apprend que les
// touches NON spéciales (les spéciales restent positionnelles via keyMapping).
//
// Les caractères produits par les touches apprises DÉJÀ TENUES (répétition OS)
// sont exclus : ainsi « tenir A puis presser B » apprend bien B→'b' et non B→'a'.
// Une touche just-pressed qui ne produit aucun caractère MO5 voit son
// association obsolète purgée (évite de tenir un ancien caractère).
func learnLiveKeys(model *keyboard.Model, learned map[ebiten.Key]liveKey, justPressed []ebiten.Key, chars []rune, pressed func(ebiten.Key) bool, joystickKBEnabled bool) {
	if len(justPressed) == 0 {
		return
	}
	jp := map[ebiten.Key]bool{}
	for _, k := range justPressed {
		jp[k] = true
	}
	// Répétitions OS des touches apprises encore tenues (hors just-pressed).
	heldRunes := map[rune]int{}
	for k, lk := range learned {
		if !jp[k] && pressed(k) {
			heldRunes[lk.r]++
		}
	}
	// Caractères « nouveaux » de la frame (hors répétitions des touches tenues).
	candidates := make([]rune, 0, len(chars))
	for _, r := range chars {
		if heldRunes[r] > 0 {
			heldRunes[r]--
			continue
		}
		candidates = append(candidates, r)
	}
	ci := 0
	for _, k := range justPressed {
		if _, special := model.SpecialKeys[int(k)]; special {
			continue
		}
		// Inc J3a : les touches réservées au joystick (WASD, AltGr) ne doivent
		// JAMAIS apprendre un caractère MO5 quand le mode joystick clavier est
		// activé — sinon chaque mouvement joystick J2 taperait les lettres
		// Z/Q/S/D en BASIC. Si le mode est désactivé (défaut), la fonction
		// retourne false et l'apprentissage normal a lieu.
		if isJoystickExclusiveKey(k, joystickKBEnabled) {
			continue
		}
		var r rune
		if ci < len(candidates) {
			r = candidates[ci]
			ci++
		}
		if mo5, shift, ok := model.CharToKey(r); ok {
			learned[k] = liveKey{mo5: mo5, shift: shift, r: r}
		} else {
			delete(learned, k) // pas de caractère MO5 → purge l'association obsolète
		}
	}
}

// resolveKeys construit l'instantané des touches MO5 à partir de l'état physique
// (pressed), des touches-caractères apprises (tenues), et de l'injecteur
// (tickKeys, pour --exec/collage). Fonction pure : testable sans Ebitengine.
//
// Politique MODIFICATEURS : quand une touche-caractère apprise est tenue, le
// SHIFT MO5 est piloté par le besoin du caractère (learned.shift), et les
// modificateurs PHYSIQUES (Shift/CNT/ACC) sont ignorés — cela évite le
// double-shift AZERTY (rangée chiffres) et la fuite d'AltGr vers ACC/CNT
// (ex. AltGr+0 = '@'), le caractère décodé par l'OS encodant déjà tout. Sans
// touche-caractère tenue, les modificateurs physiques restent positionnels.
// Pendant une injection (--exec/collage), Shift/CNT physiques sont filtrés.
func resolveKeys(model *keyboard.Model, pressed func(ebiten.Key) bool, learned map[ebiten.Key]liveKey, injecting bool, tickKeys []int, joystickKBEnabled bool) emu.InputState {
	// Joystick au repos par défaut (cf. machine.NeutralJoystick, Inc J0/J2a) :
	// la zéro-value Go {0x00, 0x00} serait interprétée par la machine comme
	// « toutes directions appuyées » en logique inversée — régression silencieuse
	// dès que le pipeline joystick est en service. Inc J3a remplacera ce repos
	// par l'état lu depuis uimodel.JoystickFromKeys.
	in := emu.InputState{
		Keys:     make([]bool, model.KeyCount),
		Joystick: machine.NeutralJoystick,
	}

	liveCharHeld := false
	shiftFromChars := false
	if !injecting {
		for k, lk := range learned {
			// Inc J3a fix codex P2 : si le mode joystick clavier est activé, on
			// IGNORE les associations apprises avant le toggle pour les touches
			// joystick (W/A/S/D). Sinon, une touche WASD tapée en BASIC AVANT
			// l'activation joystick reste dans `learned` et continue à émettre
			// son caractère MO5 à chaque mouvement joystick — exclusion
			// learnLiveKeys/SpecialKeys insuffisante pour ce cas.
			if isJoystickExclusiveKey(k, joystickKBEnabled) {
				continue
			}
			if pressed(k) && lk.mo5 >= 0 && lk.mo5 < model.KeyCount {
				in.Keys[lk.mo5] = true
				liveCharHeld = true
				if lk.shift {
					shiftFromChars = true
				}
			}
		}
	}

	// Touches positionnelles : table par MACHINE (model.SpecialKeys, peuplée par
	// keyboard_init.go). Sur MO5 c'est l'ancienne keyMapping ; sur TO8D les
	// scancodes diffèrent (Enter=0x46, SHIFT=0x51, etc.) — fini le bug « ENTER
	// tape un espace » qui venait du partage d'une table figée MO5.
	for eKeyInt, machineKey := range model.SpecialKeys {
		if injecting && (machineKey == model.ShiftKey || machineKey == model.CNTKey) {
			continue
		}
		if liveCharHeld && model.IsModifier(machineKey) {
			// SHIFT/CNT/ACC physiques ignorés quand une touche-caractère est tenue :
			// le caractère décodé par l'OS encode déjà le modificateur (anti
			// double-shift AZERTY ; anti-fuite AltGr → ACC/CNT, ex. AltGr+0 = '@').
			continue
		}
		// Inc J3a : touches réservées au joystick exclues du clavier émulation
		// QUAND le mode joystick clavier est activé (toggle F12). Couvre AltGr
		// (sinon ACC parasite à chaque fire J1). KeyShiftLeft et les flèches
		// NE SONT PAS exclues (cf. D5 : double-input acceptés).
		if isJoystickExclusiveKey(ebiten.Key(eKeyInt), joystickKBEnabled) {
			continue
		}
		if pressed(ebiten.Key(eKeyInt)) && machineKey < model.KeyCount {
			in.Keys[machineKey] = true
		}
	}
	if liveCharHeld && shiftFromChars {
		in.Keys[model.ShiftKey] = true
	}

	for _, k := range tickKeys {
		if k >= 0 && k < model.KeyCount {
			in.Keys[k] = true
		}
	}
	return in
}

// La table positionnelle MO5 vit désormais dans keyboard_init.go (mo5SpecialKeys)
// et est injectée dans keyboard.MO5Model().SpecialKeys au load du paquet. La
// table TO8D (to8dSpecialKeys) y vit aussi. Voir Inc Ka du fix clavier TO8D.

// TitleForState retourne le titre de fenêtre pour un état donné.
// Fonction pure testable sans Ebitengine.
func TitleForState(romMissing, paused bool, romName, tapeName, diskName string) string {
	title := windowTitle
	if romMissing {
		title += " — ROM manquante"
	} else if romName != "" && romName != "." {
		title += " — " + romName
		if tapeName != "" && tapeName != "." {
			title += " [" + tapeName + "]"
		} else if diskName != "" && diskName != "." {
			title += " [" + diskName + "]"
		}
	}
	if paused {
		title += " [PAUSE]"
	}
	return fmt.Sprintf("%s", title)
}
