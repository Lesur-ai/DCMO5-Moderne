// Package app adapte le cœur MO5 au desktop via Ebitengine.
// Il est le seul package autorisé à importer Ebitengine.
package app

import (
	"image/color"

	"github.com/Lesur-ai/dcmo5/internal/core"
	"github.com/Lesur-ai/dcmo5/internal/spec"
	"github.com/hajimehoshi/ebiten/v2"
)

const (
	windowTitle  = "DCMO5 Moderne"
	windowScaleX = 2
	windowScaleY = 2
)

// cyclesPerFrame est le nombre de cycles CPU par frame à 60 Hz.
// spec.CPUClockHz (1 MHz) / 60 fps ≈ 16 667 cycles.
const cyclesPerFrame = spec.CPUClockHz / 60

// App implémente ebiten.Game et orchestre la boucle principale.
type App struct {
	machine     *core.Machine
	fb          *ebiten.Image // framebuffer logique 336×216
	extraCycles int           // cycles en excès de la frame précédente
}

// New crée une application avec la machine donnée.
func New(machine *core.Machine) *App {
	fb := ebiten.NewImage(spec.FrameWidth, spec.FrameHeight)
	fb.Fill(color.RGBA{R: 0, G: 0, B: 0, A: 0xFF})
	return &App{machine: machine, fb: fb}
}

// Update est appelé à chaque tick (60 Hz) : entrées + émulation CPU.
func (a *App) Update() error {
	// 1. Mapping input clavier → touches MO5
	for eKey, mo5Key := range keyMapping {
		a.machine.SetKey(core.Key(mo5Key), ebiten.IsKeyPressed(eKey))
	}

	// 2. Souris → crayon optique
	// ebiten.CursorPosition() retourne déjà des coordonnées logiques après Layout().
	mx, my := ebiten.CursorPosition()
	pressed := ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft)
	a.machine.SetPen(mx, my, pressed)

	// 3. Avancer l'émulation d'une frame (gestion de l'excès de cycles)
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
	// Lire le framebuffer depuis le cœur MO5
	pixels := a.machine.Framebuffer()
	// Convertir []uint32 (AABBGGRR) vers []byte (RGBA pour Ebitengine WritePixels)
	buf := make([]byte, len(pixels)*4)
	for i, px := range pixels {
		buf[i*4+0] = byte(px)       // R
		buf[i*4+1] = byte(px >> 8)  // G
		buf[i*4+2] = byte(px >> 16) // B
		buf[i*4+3] = byte(px >> 24) // A
	}
	a.fb.WritePixels(buf)

	// Scale le framebuffer logique vers la fenêtre physique
	op := &ebiten.DrawImageOptions{}
	scaleX := float64(screen.Bounds().Dx()) / float64(spec.FrameWidth)
	scaleY := float64(screen.Bounds().Dy()) / float64(spec.FrameHeight)
	op.GeoM.Scale(scaleX, scaleY)
	screen.DrawImage(a.fb, op)
}

// Layout retourne les dimensions logiques fixes du framebuffer MO5.
func (a *App) Layout(_, _ int) (int, int) {
	return LogicalSize()
}

// LogicalSize retourne les dimensions logiques du framebuffer MO5.
// Fonction pure, testable sans initialiser Ebitengine.
func LogicalSize() (int, int) {
	return spec.FrameWidth, spec.FrameHeight
}

// Run configure et lance la boucle Ebitengine.
func Run(a *App) error {
	ebiten.SetWindowTitle(windowTitle)
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	ebiten.SetWindowSize(windowScaleX*spec.FrameWidth, windowScaleY*spec.FrameHeight)
	return ebiten.RunGame(a)
}

// KeyMapping retourne la table de mapping touches Ebitengine → indices MO5.
// Exposée pour les tests de validité (doublons, bornes).
func KeyMapping() map[ebiten.Key]int { return keyMapping }

// keyMapping mappe les touches Ebitengine vers les indices de touches MO5.
// Ref: dcmo5keyb.h mo5key[] (noms et indices 0x00–0x39).
var keyMapping = map[ebiten.Key]int{
	// Rangée chiffres (haut)
	ebiten.Key1: 0x2F, // 1 !
	ebiten.Key2: 0x27, // 2 "
	ebiten.Key3: 0x1F, // 3 #
	ebiten.Key4: 0x17, // 4 $
	ebiten.Key5: 0x0F, // 5 %
	ebiten.Key6: 0x07, // 6 &
	ebiten.Key7: 0x06, // 7 '
	ebiten.Key8: 0x0E, // 8 (
	ebiten.Key9: 0x16, // 9 )
	ebiten.Key0: 0x1E, // 0 `

	// Rangée AZERTY ligne 1
	ebiten.KeyA: 0x2D, // A
	ebiten.KeyZ: 0x25, // Z
	ebiten.KeyE: 0x1D, // E
	ebiten.KeyR: 0x15, // R
	ebiten.KeyT: 0x0D, // T
	ebiten.KeyY: 0x05, // Y
	ebiten.KeyU: 0x04, // U
	ebiten.KeyI: 0x0C, // I
	ebiten.KeyO: 0x14, // O
	ebiten.KeyP: 0x1C, // P

	// Rangée ligne 2
	ebiten.KeyQ: 0x2B, // Q
	ebiten.KeyS: 0x23, // S
	ebiten.KeyD: 0x1B, // D
	ebiten.KeyF: 0x13, // F
	ebiten.KeyG: 0x0B, // G
	ebiten.KeyH: 0x03, // H
	ebiten.KeyJ: 0x02, // J
	ebiten.KeyK: 0x0A, // K
	ebiten.KeyL: 0x12, // L

	// Rangée ligne 3
	ebiten.KeyW: 0x30, // W
	ebiten.KeyX: 0x28, // X
	ebiten.KeyC: 0x32, // C
	ebiten.KeyV: 0x2A, // V
	ebiten.KeyB: 0x22, // B
	ebiten.KeyN: 0x00, // N
	ebiten.KeyM: 0x1A, // M

	// Touches spéciales
	ebiten.KeySpace:        0x20, // ESPACE
	ebiten.KeyEnter:        0x34, // ENT
	ebiten.KeyBackspace:    0x01, // EFF
	ebiten.KeyInsert:       0x09, // INS
	ebiten.KeyArrowRight:   0x19, // →
	ebiten.KeyArrowLeft:    0x29, // ←
	ebiten.KeyArrowDown:    0x21, // ↓
	ebiten.KeyArrowUp:      0x31, // ↑
	ebiten.KeyShiftLeft:    0x38, // SHIFT
	ebiten.KeyShiftRight:   0x38, // SHIFT
	ebiten.KeyControlLeft:  0x35, // CNT
	ebiten.KeyControlRight: 0x35, // CNT

	// Ponctuation
	ebiten.KeyComma:      0x08, // , <
	ebiten.KeyPeriod:     0x10, // . >
	ebiten.KeySlash:      0x24, // / ?
	ebiten.KeyMinus:      0x26, // - =
	ebiten.KeyEqual:      0x2E, // + ;
	ebiten.KeySemicolon:  0x2E, // + ; (alt)
	ebiten.KeyApostrophe: 0x18, // @ ^
}
