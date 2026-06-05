// Package app adapte le cœur MO5 au desktop via Ebitengine.
// Il est le seul package autorisé à importer Ebitengine.
package app

import (
	"image/color"

	"github.com/Lesur-ai/dcmo5/internal/spec"
	"github.com/hajimehoshi/ebiten/v2"
)

const (
	windowTitle  = "DCMO5 Moderne"
	windowScaleX = 2
	windowScaleY = 2
)

// mo5Black est la couleur de fond MO5 (index 0 de la palette Thomson,
// avec correction gamma appliquée : R=G=B=0).
var mo5Black = color.RGBA{R: 0, G: 0, B: 0, A: 0xFF}

// App implémente ebiten.Game et orchestre la boucle principale.
type App struct {
	fb *ebiten.Image // framebuffer logique 336×216
}

// New crée une application prête à être lancée via ebiten.RunGame.
func New() *App {
	fb := ebiten.NewImage(spec.FrameWidth, spec.FrameHeight)
	fb.Fill(mo5Black)
	return &App{fb: fb}
}

// Update est appelé à chaque tick (logique). Stub pour P1.
func (a *App) Update() error {
	return nil
}

// Draw rend le framebuffer logique dans la surface Ebitengine.
// Ebitengine scale automatiquement fb (336×216 logiques) vers la fenêtre physique.
func (a *App) Draw(screen *ebiten.Image) {
	op := &ebiten.DrawImageOptions{}
	scaleX := float64(screen.Bounds().Dx()) / float64(spec.FrameWidth)
	scaleY := float64(screen.Bounds().Dy()) / float64(spec.FrameHeight)
	op.GeoM.Scale(scaleX, scaleY)
	screen.DrawImage(a.fb, op)
}

// Layout retourne les dimensions logiques fixes du framebuffer MO5.
// Ebitengine gère le scaling physique vers la fenêtre ; la surface de rendu
// a toujours exactement spec.FrameWidth × spec.FrameHeight pixels logiques.
func (a *App) Layout(_, _ int) (int, int) {
	return spec.FrameWidth, spec.FrameHeight
}

// Run configure et lance la boucle Ebitengine.
func Run(a *App) error {
	ebiten.SetWindowTitle(windowTitle)
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	ebiten.SetWindowSize(windowScaleX*spec.FrameWidth, windowScaleY*spec.FrameHeight)
	return ebiten.RunGame(a)
}
