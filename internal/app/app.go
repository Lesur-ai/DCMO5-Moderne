// Package app adapte le cœur MO5 au desktop via Ebitengine.
// Il est le seul package autorisé à importer Ebitengine.
package app

import (
	"errors"
	"fmt"
	"image/color"
	"io"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"

	"github.com/Lesur-ai/dcmo5/internal/core"
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

// cyclesPerFrame est le nombre de cycles CPU par frame à 60 Hz.
const cyclesPerFrame = spec.CPUClockHz / 60

// App implémente ebiten.Game et orchestre la boucle principale.
type App struct {
	machine     *core.Machine
	fb          *ebiten.Image
	extraCycles int

	// Saisie clavier
	keys       *keyInjector
	inputChars []rune

	// Menu de pilotage
	menu     *menu.Model
	mediaDir string // répertoire de départ du navigateur de fichiers

	// Médias montés : on garde les Closer pour fermer les fichiers à l'éjection
	// ou avant un remplacement (éviter les fuites de descripteurs).
	tapeCloser io.Closer
	diskCloser io.Closer

	// Audio (architecture audio-driven : le thread audio pilote l'émulation).
	// mu protège tout accès à machine (thread audio Read vs thread jeu Update/Draw).
	// emuPaused est lu par le thread audio, écrit par le thread jeu (atomique).
	mu            sync.Mutex
	emuPaused     atomic.Bool
	audioPlayer   *ebaudio.Player
	audioActive   bool // true si l'émulation est cadencée par le thread audio
	audioDisabled bool

	// État desktop
	paused     bool
	romMissing bool
	romName    string
	tapeName   string
	diskName   string
	cartName   string
}

// New crée une application avec la machine donnée.
func New(machine *core.Machine) *App {
	fb := ebiten.NewImage(spec.FrameWidth, spec.FrameHeight)
	fb.Fill(color.RGBA{R: 0, G: 0, B: 0, A: 0xFF})
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		home = "."
	}
	a := &App{
		machine:  machine,
		fb:       fb,
		keys:     newKeyInjector(defaultKeyHoldFrames, defaultKeyGapFrames),
		mediaDir: home,
	}
	a.menu = menu.NewModel(osLister)
	return a
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

// SetStartupMediaClosers confie à l'App les descripteurs des médias ouverts au
// démarrage (CLI), pour qu'ils soient fermés proprement si on les remplace
// depuis le menu (évite une fuite du descripteur initial). nil est accepté.
func (a *App) SetStartupMediaClosers(tape, disk io.Closer) {
	a.tapeCloser = tape
	a.diskCloser = disk
}

// Update est appelé à chaque tick (60 Hz) : entrées + émulation CPU.
func (a *App) Update() error {
	// ÉCHAP pilote le menu : ouvre s'il est fermé, remonte d'un niveau sinon.
	if inputJustPressed(ebiten.KeyEscape) {
		if a.menu.IsOpen() {
			a.menu.Back()
		} else {
			a.menu.Toggle()
		}
		a.refreshPause()
		a.updateTitle()
	}

	// Menu ouvert : il capte toutes les entrées et suspend l'émulation.
	if a.menu.IsOpen() {
		err := a.updateMenu()
		a.refreshPause() // une action a pu fermer le menu
		if err != nil {
			return err
		}
		return nil
	}

	// F5 = reset machine (sous verrou : le thread audio touche la machine)
	if inputJustPressed(ebiten.KeyF5) {
		a.mu.Lock()
		a.machine.Reset()
		a.mu.Unlock()
	}

	// F3 = pause / resume (KeyP est la touche MO5 P=0x1C, on évite le conflit)
	if inputJustPressed(ebiten.KeyF3) {
		a.paused = !a.paused
		a.refreshPause()
		a.updateTitle()
	}

	if a.paused {
		return nil
	}

	// Saisie clavier MO5 : caractères (layout OS + Shift) + touches spéciales.
	//
	// LIMITE connue : les touches imprimables (lettres/chiffres) sont jouées en
	// impulsions par l'injecteur, pas maintenues en continu — adapté à la frappe
	// de texte (BASIC) mais pas au maintien d'une touche dans un jeu. Un mode
	// « gaming » positionnel (toggle) est prévu dans une étape ultérieure.
	a.inputChars = ebiten.AppendInputChars(a.inputChars[:0])
	for _, r := range a.inputChars {
		a.keys.Enqueue(r)
	}

	// Touches caractère injectées (une frappe à la fois, maintenue puis relâchée).
	tickKeys := a.keys.Tick()
	// « injecting » : une frappe caractère est en cours de rejeu (hold ou gap).
	// Pendant ce temps, le Shift physique ne doit PAS être propagé : l'OS a déjà
	// produit le bon caractère, et l'injecteur pilote seul SHIFT MO5 (sinon
	// double-Shift, ex. AZERTY « 1 » → « ! »). Hors saisie, Shift redevient une
	// touche positionnelle maintenable (jeux, combinaisons MO5).
	injecting := len(tickKeys) > 0 || a.keys.Pending() > 0

	var active [spec.KeyMax]bool
	// Touches spéciales en mode positionnel (état physique continu).
	for eKey, mo5Key := range keyMapping {
		if mo5Key == mo5KeyShift && injecting {
			continue
		}
		if ebiten.IsKeyPressed(eKey) {
			active[mo5Key] = true
		}
	}
	for _, k := range tickKeys {
		if k >= 0 && k < spec.KeyMax {
			active[k] = true
		}
	}
	mx, my := ebiten.CursorPosition()
	penPressed := ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft)

	// Tout accès à la machine est protégé : en audio-driven, le thread audio la
	// fait tourner en parallèle (audioReader.Read).
	a.mu.Lock()
	for k := 0; k < spec.KeyMax; k++ {
		a.machine.SetKey(core.Key(k), active[k])
	}
	a.machine.SetPen(mx, my, penPressed)
	// L'émulation n'est avancée ici QUE si l'audio ne la pilote pas (mode
	// --no-audio ou init audio échouée). Sinon, audioReader.Read s'en charge,
	// cadencé par l'horloge audio.
	if !a.audioActive {
		toRun := cyclesPerFrame - a.extraCycles
		if toRun < 0 {
			toRun = 0
		}
		consumed := a.machine.Step(toRun)
		a.extraCycles = consumed - toRun
		if a.extraCycles < 0 {
			a.extraCycles = 0
		}
	}
	a.mu.Unlock()
	return nil
}

// refreshPause met à jour le drapeau de pause lu par le thread audio. À appeler
// après tout changement de pause ou d'état du menu.
func (a *App) refreshPause() {
	a.emuPaused.Store(a.paused || a.menu.IsOpen())
}

// updateMenu traite les entrées (clavier ET souris) quand le menu est ouvert et
// exécute l'action sélectionnée. L'émulation est suspendue tant que le menu est
// affiché.
func (a *App) updateMenu() error {
	// Clavier : flèches.
	if inputJustPressed(ebiten.KeyArrowUp) {
		a.menu.MoveUp()
	}
	if inputJustPressed(ebiten.KeyArrowDown) {
		a.menu.MoveDown()
	}

	// Souris : le survol surligne l'item pointé.
	mx, my := ebiten.CursorPosition()
	hovered := menuItemAt(a.menu, mx, my)
	if hovered >= 0 {
		a.selectMenuIndex(hovered)
	}

	// Molette : défile le navigateur de fichiers.
	if _, wy := ebiten.Wheel(); wy != 0 && a.menu.State() == menu.StateBrowse {
		if wy < 0 {
			a.menu.MoveDown()
		} else {
			a.menu.MoveUp()
		}
	}

	// Validation : ENTRÉE, ou clic gauche sur un item.
	activate := inputJustPressed(ebiten.KeyEnter)
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) && hovered >= 0 {
		a.selectMenuIndex(hovered)
		activate = true
	}
	if activate {
		act := a.menu.Activate(a.mediaDir)
		return a.handleMenuAction(act)
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

// handleMenuAction exécute l'intention produite par le menu. Les actions qui
// touchent la machine sont protégées par a.mu (le thread audio peut tourner).
func (a *App) handleMenuAction(act menu.Action) error {
	if act == menu.ActQuit {
		return ErrUserQuit
	}
	a.mu.Lock()
	switch act {
	case menu.ActResume:
		// Le modèle a déjà fermé le menu.
	case menu.ActReset:
		a.machine.Reset()
		a.menu.Close()
	case menu.ActEjectTape:
		a.machine.EjectTape()
		a.closeTape()
		a.tapeName = ""
	case menu.ActEjectDisk:
		a.machine.EjectDisk()
		a.closeDisk()
		a.diskName = ""
	case menu.ActEjectCart:
		a.machine.EjectCartridge()
		a.cartName = ""
	case menu.ActMountChosen:
		a.mountChosen()
	}
	a.mu.Unlock()
	a.updateTitle()
	return nil
}

// mountChosen ouvre le fichier choisi dans le navigateur et le monte sur la
// machine, en remplaçant proprement le média précédent du même type.
func (a *App) mountChosen() {
	path, kind := a.menu.Chosen()
	if path == "" {
		return
	}
	// Mémoriser le répertoire pour rouvrir le navigateur au même endroit.
	a.mediaDir = filepath.Dir(path)

	switch kind {
	case menu.KindTape:
		t, err := impl.OpenTape(path, false)
		if err != nil {
			return
		}
		a.closeTape()
		a.machine.MountTape(t)
		a.tapeCloser = t
		a.tapeName = filepath.Base(path)
	case menu.KindDisk:
		d, err := impl.OpenDisk(path, false)
		if err != nil {
			return
		}
		a.closeDisk()
		a.machine.MountDisk(d)
		a.diskCloser = d
		a.diskName = filepath.Base(path)
	case menu.KindCart:
		c, err := impl.OpenCartridge(path)
		if err != nil {
			return
		}
		a.machine.MountCartridge(c)
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

// Draw rend le framebuffer machine dans la surface Ebitengine.
func (a *App) Draw(screen *ebiten.Image) {
	if a.romMissing {
		screen.Fill(color.RGBA{R: 20, G: 0, B: 0, A: 0xFF})
		return
	}
	// Framebuffer() lit la RAM vidéo ; la protéger du thread audio qui fait
	// avancer l'émulation (écritures RAM).
	a.mu.Lock()
	pixels := a.machine.Framebuffer()
	a.mu.Unlock()
	buf := make([]byte, len(pixels)*4)
	for i, px := range pixels {
		buf[i*4+0] = byte(px)
		buf[i*4+1] = byte(px >> 8)
		buf[i*4+2] = byte(px >> 16)
		buf[i*4+3] = byte(px >> 24)
	}
	a.fb.WritePixels(buf)
	op := &ebiten.DrawImageOptions{}
	scaleX := float64(screen.Bounds().Dx()) / float64(spec.FrameWidth)
	scaleY := float64(screen.Bounds().Dy()) / float64(spec.FrameHeight)
	op.GeoM.Scale(scaleX, scaleY)
	screen.DrawImage(a.fb, op)

	// Overlay du menu de pilotage par-dessus l'écran émulé.
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
	a.initAudio() // après que main a pu désactiver l'audio (--no-audio)
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

// titleForState retourne le titre de fenêtre pour un état donné.
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
