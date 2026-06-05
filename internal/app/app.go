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
	return &App{machine: machine, fb: fb}
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

	// Mapping input clavier → touches MO5
	for eKey, mo5Key := range keyMapping {
		a.machine.SetKey(core.Key(mo5Key), ebiten.IsKeyPressed(eKey))
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

// keyMapping mappe les touches Ebitengine vers les indices de touches MO5.
// Ref: dcmo5keyb.h mo5key[] (indices 0x00–0x39).
var keyMapping = map[ebiten.Key]int{
	ebiten.Key1: 0x2F, ebiten.Key2: 0x27, ebiten.Key3: 0x1F,
	ebiten.Key4: 0x17, ebiten.Key5: 0x0F, ebiten.Key6: 0x07,
	ebiten.Key7: 0x06, ebiten.Key8: 0x0E, ebiten.Key9: 0x16,
	ebiten.Key0: 0x1E,
	ebiten.KeyA: 0x2D, ebiten.KeyZ: 0x25, ebiten.KeyE: 0x1D,
	ebiten.KeyR: 0x15, ebiten.KeyT: 0x0D, ebiten.KeyY: 0x05,
	ebiten.KeyU: 0x04, ebiten.KeyI: 0x0C, ebiten.KeyO: 0x14,
	ebiten.KeyP: 0x1C, ebiten.KeyQ: 0x2B, ebiten.KeyS: 0x23,
	ebiten.KeyD: 0x1B, ebiten.KeyF: 0x13, ebiten.KeyG: 0x0B,
	ebiten.KeyH: 0x03, ebiten.KeyJ: 0x02, ebiten.KeyK: 0x0A,
	ebiten.KeyL: 0x12, ebiten.KeyW: 0x30, ebiten.KeyX: 0x28,
	ebiten.KeyC: 0x32, ebiten.KeyV: 0x2A, ebiten.KeyB: 0x22,
	ebiten.KeyN: 0x00, ebiten.KeyM: 0x1A,
	ebiten.KeySpace: 0x20, ebiten.KeyEnter: 0x34,
	ebiten.KeyBackspace: 0x01, ebiten.KeyInsert: 0x09,
	ebiten.KeyArrowRight: 0x19, ebiten.KeyArrowLeft: 0x29,
	ebiten.KeyArrowDown: 0x21, ebiten.KeyArrowUp: 0x31,
	ebiten.KeyShiftLeft: 0x38, ebiten.KeyShiftRight: 0x38,
	ebiten.KeyControlLeft: 0x35, ebiten.KeyControlRight: 0x35,
	ebiten.KeyComma: 0x08, ebiten.KeyPeriod: 0x10,
	ebiten.KeySlash: 0x24, ebiten.KeyMinus: 0x26,
	ebiten.KeyEqual: 0x2E, ebiten.KeySemicolon: 0x2E,
	ebiten.KeyApostrophe: 0x18,
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
