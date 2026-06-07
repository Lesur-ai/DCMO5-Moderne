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

	"github.com/Lesur-ai/dcmo5/internal/core"
	"github.com/Lesur-ai/dcmo5/internal/emu"
	"github.com/Lesur-ai/dcmo5/internal/keyboard"
	"github.com/Lesur-ai/dcmo5/internal/media"
	"github.com/Lesur-ai/dcmo5/internal/media/impl"
	"github.com/Lesur-ai/dcmo5/internal/menu"
	"github.com/Lesur-ai/dcmo5/internal/spec"
	"github.com/hajimehoshi/ebiten/v2"
	ebaudio "github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// ErrUserQuit est retourné par Run quand l'utilisateur ferme la fenêtre.
var ErrUserQuit = errors.New("quit")

const (
	windowTitle  = "DCMO5 Moderne"
	windowScaleX = 2
	windowScaleY = 2
)

// App implémente ebiten.Game et orchestre l'UI autour d'un emu.Host.
type App struct {
	host     *emu.Host
	fb       *ebiten.Image
	fbPixels []uint32 // tampon framebuffer réutilisé (anti-alloc/GC)
	fbBytes  []byte   // tampon RGBA réutilisé pour WritePixels

	// Saisie clavier
	keys       *keyboard.Injector
	inputChars []rune

	// Saisie programmée (--exec, coller). execSeq attend la fin du délai de boot,
	// puis alimente typeAhead, lui-même vidé progressivement vers l'injecteur
	// (sans dépasser sa file bornée, donc sans perdre le début d'un long script).
	execSeq         string
	execDelayFrames int
	typeAhead       []rune

	// Menu de pilotage
	menu     *menu.Model
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

// New crée une application pilotant la machine donnée via un emu.Host.
func New(machine *core.Machine) *App {
	fb := ebiten.NewImage(spec.FrameWidth, spec.FrameHeight)
	fb.Fill(color.RGBA{R: 0, G: 0, B: 0, A: 0xFF})
	a := &App{
		host:     emu.New(machine, defaultAudioGain),
		fb:       fb,
		fbPixels: make([]uint32, spec.FrameWidth*spec.FrameHeight),
		fbBytes:  make([]byte, spec.FrameWidth*spec.FrameHeight*4),
		keys:     keyboard.NewInjector(keyboard.DefaultHoldFrames, keyboard.DefaultGapFrames),
		mediaDir: startMediaDir(os.Getwd, os.UserHomeDir),
	}
	a.menu = menu.NewModel(osLister)
	return a
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

	// Saisie clavier MO5 : caractères (layout OS + Shift) + touches spéciales.
	// LIMITE connue : les touches imprimables sont jouées en impulsions par
	// l'injecteur (adapté à la frappe de texte), pas maintenues en continu.
	a.inputChars = ebiten.AppendInputChars(a.inputChars[:0])
	for _, r := range a.inputChars {
		if r == ' ' {
			continue // l'espace live est géré en positionnel (KeySpace), pas via l'injecteur
		}
		a.keys.Enqueue(r)
	}
	tickKeys := a.keys.Tick()
	// Pendant une saisie caractère, le Shift physique n'est pas propagé (l'OS a
	// déjà produit le bon caractère ; sinon double-Shift AZERTY « 1 » → « ! »).
	injecting := len(tickKeys) > 0 || a.keys.Pending() > 0

	var in emu.InputState
	for eKey, mo5Key := range keyMapping {
		// Pendant une injection (saisie caractère, --exec, coller), ne pas
		// propager les modificateurs physiques : l'OS a déjà produit le bon
		// caractère, et le SHIFT/CNT physiques (ex. Ctrl maintenu pour Ctrl+V)
		// parasiteraient les frappes injectées.
		if injecting && (mo5Key == keyboard.Mo5KeyShift || mo5Key == keyboard.Mo5KeyCNT) {
			continue
		}
		if ebiten.IsKeyPressed(eKey) {
			in.Keys[mo5Key] = true
		}
	}
	for _, k := range tickKeys {
		if k >= 0 && k < spec.KeyMax {
			in.Keys[k] = true
		}
	}
	// Le curseur Ebitengine est en repère framebuffer (Layout = 336×216, bordure
	// incluse). Le crayon MO5 attend le repère écran actif → on retranche la
	// bordure. Hors zone active, le cœur (readPenXY) signalera « pas de détection ».
	in.PenX, in.PenY = spec.PenFromFramebuffer(ebiten.CursorPosition())
	in.PenDown = ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft)
	a.host.SetInput(in)
	return nil
}

// syncPause répercute l'état pause/menu sur le Host (suspend l'émulation).
func (a *App) syncPause() { a.host.SetPaused(a.paused || a.menu.IsOpen()) }

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

// compile-time : *impl types satisfont media + io.Closer (sécurité de typage).
var (
	_ media.Tape = (*impl.FileTape)(nil)
	_ io.Closer  = (*impl.FileTape)(nil)
)

// Draw rend l'instantané du framebuffer du Host dans la surface Ebitengine.
func (a *App) Draw(screen *ebiten.Image) {
	if a.romMissing {
		screen.Fill(color.RGBA{R: 20, G: 0, B: 0, A: 0xFF})
		return
	}
	a.host.Framebuffer(a.fbPixels) // copie de l'instantané (pas d'accès au cœur)
	for i, px := range a.fbPixels {
		a.fbBytes[i*4+0] = byte(px)
		a.fbBytes[i*4+1] = byte(px >> 8)
		a.fbBytes[i*4+2] = byte(px >> 16)
		a.fbBytes[i*4+3] = byte(px >> 24)
	}
	a.fb.WritePixels(a.fbBytes)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(
		float64(screen.Bounds().Dx())/float64(spec.FrameWidth),
		float64(screen.Bounds().Dy())/float64(spec.FrameHeight),
	)
	screen.DrawImage(a.fb, op)

	drawMenu(screen, a.menu)
}

// Layout retourne les dimensions logiques fixes du framebuffer MO5.
func (a *App) Layout(_, _ int) (int, int) { return LogicalSize() }

// LogicalSize retourne les dimensions logiques. Testable sans Ebitengine.
func LogicalSize() (int, int) { return spec.FrameWidth, spec.FrameHeight }

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
	if a.menu != nil && a.menu.IsOpen() {
		title += " [MENU]"
	}
	if a.paused {
		title += " [PAUSE]"
	}
	ebiten.SetWindowTitle(title)
}

// Run configure et lance la boucle Ebitengine.
func Run(a *App) error {
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	ebiten.SetWindowSize(windowScaleX*spec.FrameWidth, windowScaleY*spec.FrameHeight)
	a.initAudio()  // après que main a pu désactiver l'audio (--no-audio)
	a.host.Start() // lance la goroutine d'émulation
	defer a.host.Stop()
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
