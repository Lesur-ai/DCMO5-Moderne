// Package app adapte le cœur MO5 au desktop via Ebitengine.
// Il est le seul package autorisé à importer Ebitengine.
package app

import (
	"errors"
	"fmt"
	"image/color"
	"path/filepath"

	"github.com/Lesur-ai/dcmo5/internal/core"
	"github.com/Lesur-ai/dcmo5/internal/spec"
	"github.com/hajimehoshi/ebiten/v2"
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

	// État desktop
	paused     bool
	romMissing bool
	romName    string
	tapeName   string
	diskName   string
}

// New crée une application avec la machine donnée.
func New(machine *core.Machine) *App {
	fb := ebiten.NewImage(spec.FrameWidth, spec.FrameHeight)
	fb.Fill(color.RGBA{R: 0, G: 0, B: 0, A: 0xFF})
	return &App{
		machine: machine,
		fb:      fb,
		keys:    newKeyInjector(defaultKeyHoldFrames, defaultKeyGapFrames),
	}
}

// SetROMStatus indique si la ROM est absente (affichage d'avertissement).
func (a *App) SetROMStatus(missing bool) { a.romMissing = missing }

// SetMediaNames configure les noms de médias montés pour le titre fenêtre.
func (a *App) SetMediaNames(rom, tape, disk string) {
	a.romName = filepath.Base(rom)
	a.tapeName = filepath.Base(tape)
	a.diskName = filepath.Base(disk)
}

// Update est appelé à chaque tick (60 Hz) : entrées + émulation CPU.
func (a *App) Update() error {
	// Quitter proprement via Escape ou fermeture fenêtre
	if ebiten.IsKeyPressed(ebiten.KeyEscape) {
		return ErrUserQuit
	}

	// F5 = reset machine
	if inputJustPressed(ebiten.KeyF5) {
		a.machine.Reset()
	}

	// F3 = pause / resume (KeyP est la touche MO5 P=0x1C, on évite le conflit)
	if inputJustPressed(ebiten.KeyF3) {
		a.paused = !a.paused
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

	var active [spec.KeyMax]bool
	// Touches spéciales en mode positionnel (état physique continu).
	for eKey, mo5Key := range keyMapping {
		if ebiten.IsKeyPressed(eKey) {
			active[mo5Key] = true
		}
	}
	// Touches caractère injectées (une frappe à la fois, maintenue puis relâchée).
	for _, k := range a.keys.Tick() {
		if k >= 0 && k < spec.KeyMax {
			active[k] = true
		}
	}
	// Appliquer l'état résultant à la matrice clavier de la machine.
	for k := 0; k < spec.KeyMax; k++ {
		a.machine.SetKey(core.Key(k), active[k])
	}

	// Souris → crayon optique (coords logiques directes après Layout)
	mx, my := ebiten.CursorPosition()
	a.machine.SetPen(mx, my, ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft))

	// Avancer l'émulation d'une frame
	toRun := cyclesPerFrame - a.extraCycles
	if toRun < 0 {
		toRun = 0
	}
	consumed := a.machine.Step(toRun)
	a.extraCycles = consumed - toRun
	if a.extraCycles < 0 {
		a.extraCycles = 0
	}
	return nil
}

// Draw rend le framebuffer machine dans la surface Ebitengine.
func (a *App) Draw(screen *ebiten.Image) {
	if a.romMissing {
		screen.Fill(color.RGBA{R: 20, G: 0, B: 0, A: 0xFF})
		return
	}
	pixels := a.machine.Framebuffer()
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
			title += " [" + a.tapeName + "]"
		} else if a.diskName != "" && a.diskName != "." {
			title += " [" + a.diskName + "]"
		}
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
	ebiten.KeyControlLeft:  0x35, // CNT
	ebiten.KeyControlRight: 0x35,
	ebiten.KeyAltLeft:      0x36, // ACC (accent)
	ebiten.KeyAltRight:     0x36,
	ebiten.KeyTab:          0x39, // BASIC
	ebiten.KeyEnd:          0x37, // STP (stop)
}

// NOTE : la touche SHIFT MO5 (0x38) n'est volontairement PAS mappée en
// positionnel. En saisie par caractère, l'OS applique déjà Shift pour produire
// le bon caractère (« 1 » vs « ! », majuscules…) ; l'injecteur décide alors
// seul si SHIFT MO5 doit être pressé. Propager en plus le Shift physique
// produirait un double-Shift (ex. AZERTY : « 1 » deviendrait « ! »).

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
