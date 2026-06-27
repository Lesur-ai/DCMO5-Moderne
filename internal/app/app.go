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
	"github.com/Lesur-ai/dcmo5/internal/menu"
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
	launcher    *launcher
	onStart     func(profileID string, cfg machine.Config) // hook de persistance config à l'action « Démarrer » (mode launcher)
	hostStarted bool                                       // host.Start() a été appelé (garde le Stop différé)

	// Menu de pilotage (v1, en cours de remplacement par l'overlay — lot #117 Inc 3b).
	menu *menu.Model
	// overlay : machine d'état PURE de l'overlay Échap (zéro-value = fermé, aucun
	// constructeur). En 3b.1, ouvert via la touche debug F9 (gelé + voile, sans UI) ;
	// l'UI ebitenui et la bascule d'Échap arrivent en 3b.2+/3b.4 (retrait du menu v1).
	overlay  overlay.Model
	mediaDir string // répertoire de départ du navigateur de fichiers

	// Médias montés : Closer des fichiers ouverts (fermeture à l'éjection/remplacement)
	tapeCloser io.Closer
	diskCloser io.Closer

	// Audio (le lecteur consomme la ring du Host ; il ne touche jamais le cœur)
	audioPlayer   *ebaudio.Player
	audioDisabled bool

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
	a.menu = menu.NewModel(osLister)
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
	a.menu = menu.NewModel(osLister)
	a.launcher = newLauncher(profiles, mediaDir, osListerUI, initial, selected)
	return a
}

// SetOnStart enregistre un hook appelé avec l'ID du profil sélectionné et la config
// validée au moment où l'utilisateur lance une machine depuis le launcher (transition
// launcher→émulateur). L'ID permet à la couche cmd de persister le choix PAR machine
// (ex. chemin ROM) sans coupler l'App au package config. Sans effet en mode émulateur
// (chemin CLI à boot direct).
func (a *App) SetOnStart(fn func(profileID string, cfg machine.Config)) { a.onStart = fn }

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

// startMediaDir choisit le répertoire de départ du navigateur du menu : le
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

// osLister liste un répertoire réel pour le navigateur du menu.
func osLister(dir string) ([]menu.Entry, error) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	out := make([]menu.Entry, 0, len(ents))
	for _, e := range ents {
		out = append(out, menu.Entry{Name: e.Name(), IsDir: e.IsDir()})
	}
	return out, nil
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
// depuis le menu (évite une fuite du descripteur initial). nil est accepté.
func (a *App) SetStartupMediaClosers(tape, disk io.Closer) {
	a.tapeCloser = tape
	a.diskCloser = disk
}

// Update est appelé à chaque tick (60 Hz) : il publie les entrées vers le Host
// et pilote le menu. L'émulation, elle, avance dans la goroutine du Host.
func (a *App) Update() error {
	// Mode launcher : aucune émulation (host==nil). On rend/anime l'UI ebitenui et,
	// si l'utilisateur a validé « Démarrer », on instancie la machine. Branché TOUT
	// EN HAUT, avant tout accès à menu/host/keys (qui sont nil/inactifs ici).
	if a.launcher != nil {
		return a.updateLauncher()
	}

	// OVERLAY OUVERT : capture STRICTE. Branché TOUT EN HAUT, return immédiat → aucune
	// entrée (Échap menu, F3, F5, collage, touches live, crayon) n'atteint le cœur tant
	// que l'overlay est ouvert. La contrainte #3 (revue Codex) est satisfaite par la
	// STRUCTURE (ce return), pas par des gardes éparpillés.
	if a.overlay.IsOpen() {
		if err := a.updateOverlay(); err != nil {
			return err
		}
		a.syncPause() // une action (fermeture) a pu changer l'état
		return nil
	}

	// ÉCHAP pilote le menu : ouvre s'il est fermé, remonte d'un niveau sinon.
	if inputJustPressed(ebiten.KeyEscape) {
		if a.menu.IsOpen() {
			a.menu.Back()
		} else {
			a.menu.Toggle()
		}
		a.syncPause()
		a.updateTitle()
	}

	// Menu ouvert : il capte toutes les entrées et suspend l'émulation.
	if a.menu.IsOpen() {
		err := a.updateMenu()
		a.syncPause() // une action a pu fermer le menu
		if err != nil {
			return err
		}
		return nil
	}

	// DEBUG (3b.1) : F9 ouvre l'overlay pour valider à l'œil le rendu gelé + voile + la
	// capture stricte, SANS toucher au menu v1 (toujours sur Échap). Atteint uniquement
	// quand le menu est fermé (le gate ci-dessus a déjà rendu la main sinon) et l'overlay
	// fermé (le court-circuit en haut de Update a rendu la main sinon) → les deux ne
	// peuvent JAMAIS être ouverts en même temps. F9 + ce bloc disparaissent en 3b.4, où
	// Échap pilotera l'overlay et le menu v1 sera retiré.
	if inputJustPressed(ebiten.KeyF9) {
		a.overlay.Toggle()
		a.syncPause() // gèle l'émulation AU MÊME TICK que l'ouverture (sinon une frame avance)
		a.updateTitle()
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
	learnLiveKeys(a.kbModel, a.liveKeys, a.justPressed, a.inputChars, ebiten.IsKeyPressed)

	tickKeys := a.keys.Tick()
	injecting := len(tickKeys) > 0 || a.keys.Pending() > 0

	in := resolveKeys(a.kbModel, ebiten.IsKeyPressed, a.liveKeys, injecting, tickKeys)
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

// syncPause répercute l'état pause/menu/overlay sur le Host (suspend l'émulation).
// L'overlay gèle l'émulation via la fonction pure overlay.ShouldPause (testée en CI) ;
// le menu v1 garde sa contribution tant qu'il existe (retiré en 3b.4, où il ne restera
// que ShouldPause).
func (a *App) syncPause() {
	a.host.SetPaused(overlay.ShouldPause(a.paused, a.overlay.IsOpen()) || a.menu.IsOpen())
}

// updateOverlay traite les entrées quand l'overlay est ouvert. En 3b.1 il n'y a pas
// encore d'UI ebitenui : on gère seulement la FERMETURE (Échap ou la touche debug F9
// → Back, qui depuis la vue principale ferme l'overlay). Le clavier/souris vers l'UI
// et l'application des changements média arrivent en 3b.2/3b.3.
func (a *App) updateOverlay() error {
	if inputJustPressed(ebiten.KeyEscape) || inputJustPressed(ebiten.KeyF9) {
		a.overlay.Back()
		a.updateTitle()
	}
	return nil
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

// updateMenu traite les entrées (clavier ET souris) quand le menu est ouvert.
func (a *App) updateMenu() error {
	if inputJustPressed(ebiten.KeyArrowUp) {
		a.menu.MoveUp()
	}
	if inputJustPressed(ebiten.KeyArrowDown) {
		a.menu.MoveDown()
	}
	mx, my := ebiten.CursorPosition()
	hovered := menuItemAt(a.menu, mx, my)
	if hovered >= 0 {
		a.selectMenuIndex(hovered)
	}
	if _, wy := ebiten.Wheel(); wy != 0 && a.menu.State() == menu.StateBrowse {
		if wy < 0 {
			a.menu.MoveDown()
		} else {
			a.menu.MoveUp()
		}
	}
	activate := inputJustPressed(ebiten.KeyEnter)
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) && hovered >= 0 {
		a.selectMenuIndex(hovered)
		activate = true
	}
	if activate {
		return a.handleMenuAction(a.menu.Activate(a.mediaDir))
	}
	return nil
}

// selectMenuIndex positionne la sélection selon l'état courant du menu.
func (a *App) selectMenuIndex(i int) {
	switch a.menu.State() {
	case menu.StateMain:
		a.menu.SetMainIndex(i)
	case menu.StateBrowse:
		a.menu.SetBrowseIndex(i)
	}
}

// handleMenuAction exécute l'intention produite par le menu via le Host
// (commandes asynchrones traitées par la goroutine propriétaire de la machine).
func (a *App) handleMenuAction(act menu.Action) error {
	switch act {
	case menu.ActResume:
		// Le modèle a déjà fermé le menu.
	case menu.ActReset:
		a.host.Reset()
		a.menu.Close()
	case menu.ActInitprog:
		a.host.Initprog()
		a.menu.Close()
	case menu.ActQuit:
		return ErrUserQuit
	case menu.ActEjectTape:
		a.host.EjectTape()
		a.closeTape()
		a.tapeName = ""
	case menu.ActEjectDisk:
		a.host.EjectDisk()
		a.closeDisk()
		a.diskName = ""
	case menu.ActEjectCart:
		a.host.EjectCartridge()
		a.cartName = ""
	case menu.ActMountChosen:
		a.mountChosen()
	}
	a.updateTitle()
	return nil
}

// mountChosen ouvre le fichier choisi et le monte via le Host, en fermant
// proprement le média précédent du même type.
func (a *App) mountChosen() {
	path, kind := a.menu.Chosen()
	if path == "" {
		return
	}
	a.mediaDir = filepath.Dir(path)
	switch kind {
	case menu.KindTape:
		t, err := impl.OpenTape(path, false)
		if err != nil {
			return
		}
		a.closeTape()
		a.host.MountTape(t)
		a.tapeCloser = t
		a.tapeName = filepath.Base(path)
	case menu.KindDisk:
		d, err := impl.OpenDisk(path, false)
		if err != nil {
			return
		}
		a.closeDisk()
		a.host.MountDisk(d)
		a.diskCloser = d
		a.diskName = filepath.Base(path)
	case menu.KindCart:
		c, err := impl.OpenCartridge(path)
		if err != nil {
			return
		}
		a.host.MountCartridge(c)
		a.cartName = filepath.Base(path)
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
// launcher, menu) : aucune config parallèle à resynchroniser, aucun média fantôme
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

	drawMenu(screen, a.menu)
}

// blitFramebuffer recopie le dernier instantané du Host dans a.fb (pas d'accès au
// cœur). Quand l'émulation est en pause (overlay/menu/F3), le Host ne publie plus :
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
// (uimodel.FramebufferAspectFit, pur) puis un voile sombre. L'UI ebitenui de l'overlay
// (vues Main/Browse) se superposera en 3b.2+. Le letterbox est rempli en noir.
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
	// Voile sombre semi-transparent : signale que l'émulation est gelée et prépare le
	// contraste de l'UI à venir.
	vector.DrawFilledRect(screen, 0, 0, float32(sw), float32(sh), color.RGBA{R: 0, G: 0, B: 0, A: 120}, false)
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
	} else if a.menu != nil && a.menu.IsOpen() {
		title += " [MENU]"
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

// KeyMapping expose la table pour les tests.
func KeyMapping() map[ebiten.Key]int { return keyMapping }

// ── Helpers input ─────────────────────────────────────────────────────────────

// pressedLastFrame mémorise les touches pressées au tick précédent.
var pressedLastFrame = map[ebiten.Key]bool{}

// fileExists indique si un chemin existe (os.Stat sans erreur). Sert à l'auto-détection
// de la ROM contrôleur disquette au launcher (cf. updateLauncher, uimodel.ResolveDiskROM).
func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// inputJustPressed détecte une pression nouvelle (non maintenue).
func inputJustPressed(k ebiten.Key) bool {
	now := ebiten.IsKeyPressed(k)
	was := pressedLastFrame[k]
	pressedLastFrame[k] = now
	return now && !was
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
func learnLiveKeys(model *keyboard.Model, learned map[ebiten.Key]liveKey, justPressed []ebiten.Key, chars []rune, pressed func(ebiten.Key) bool) {
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
		if _, special := keyMapping[k]; special {
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
func resolveKeys(model *keyboard.Model, pressed func(ebiten.Key) bool, learned map[ebiten.Key]liveKey, injecting bool, tickKeys []int) emu.InputState {
	in := emu.InputState{Keys: make([]bool, model.KeyCount)}

	liveCharHeld := false
	shiftFromChars := false
	if !injecting {
		for k, lk := range learned {
			if pressed(k) && lk.mo5 >= 0 && lk.mo5 < model.KeyCount {
				in.Keys[lk.mo5] = true
				liveCharHeld = true
				if lk.shift {
					shiftFromChars = true
				}
			}
		}
	}

	for eKey, mo5Key := range keyMapping {
		if injecting && (mo5Key == model.ShiftKey || mo5Key == model.CNTKey) {
			continue
		}
		if liveCharHeld && model.IsModifier(mo5Key) {
			// SHIFT/CNT/ACC physiques ignorés quand une touche-caractère est tenue :
			// le caractère décodé par l'OS encode déjà le modificateur (anti
			// double-shift AZERTY ; anti-fuite AltGr → ACC/CNT, ex. AltGr+0 = '@').
			continue
		}
		if pressed(eKey) && mo5Key < model.KeyCount {
			in.Keys[mo5Key] = true
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

// keyMapping mappe les touches spéciales Ebitengine vers les indices MO5.
// Les touches « caractère » (lettres, chiffres, ponctuation) ne sont PAS ici :
// elles passent par l'injecteur de caractères (voir keyboard.go), ce qui gère
// le layout OS et les combinaisons Shift. Ce mapping ne couvre que les touches
// dont l'état physique continu fait sens (déplacement, contrôle, édition).
// Ref: dcmo5keyb.h mo5key[] (indices 0x00–0x39).
var keyMapping = map[ebiten.Key]int{
	ebiten.KeySpace:        0x20, // ESPACE
	ebiten.KeyEnter:        0x34, // ENT
	ebiten.KeyBackspace:    0x01, // EFF (effacement)
	ebiten.KeyInsert:       0x09, // INS
	ebiten.KeyDelete:       0x33, // RAZ
	ebiten.KeyHome:         0x11, // [retour]
	ebiten.KeyArrowRight:   0x19,
	ebiten.KeyArrowLeft:    0x29,
	ebiten.KeyArrowDown:    0x21,
	ebiten.KeyArrowUp:      0x31,
	ebiten.KeyShiftLeft:    0x38, // SHIFT (voir gestion conditionnelle dans Update)
	ebiten.KeyShiftRight:   0x38,
	ebiten.KeyControlLeft:  0x35, // CNT
	ebiten.KeyControlRight: 0x35,
	ebiten.KeyAltLeft:      0x36, // ACC (accent)
	ebiten.KeyAltRight:     0x36,
	ebiten.KeyTab:          0x39, // BASIC
	ebiten.KeyEnd:          0x37, // STP (stop)
}

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
