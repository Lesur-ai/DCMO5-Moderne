// Package app adapte le cœur MO5 au desktop via Ebitengine.
// Il est le seul package autorisé à importer Ebitengine.
package app

import (
	"github.com/Lesur-ai/dcmo5/internal/spec"
	"github.com/hajimehoshi/ebiten/v2"
)

const (
	windowTitle  = "DCMO5 Moderne"
	windowScaleX = 2
	windowScaleY = 2
)

// App implémente ebiten.Game et orchestre la boucle principale.
type App struct{}

// New crée une application prête à être lancée via ebiten.RunGame.
func New() *App {
	return &App{}
}

// Update est appelé à chaque tick (logique). Stub pour P1.
func (a *App) Update() error {
	return nil
}

// Draw est appelé à chaque frame (rendu). Stub pour P1.
func (a *App) Draw(screen *ebiten.Image) {}

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
