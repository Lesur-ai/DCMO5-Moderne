// Package app adapte le cœur MO5 au desktop via Ebitengine.
// Il est le seul package autorisé à importer Ebitengine.
package app

import (
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

// Layout retourne les dimensions logiques de la surface de rendu.
// Ebitengine gère le scaling vers la fenêtre physique.
func (a *App) Layout(outsideWidth, outsideHeight int) (int, int) {
	return outsideWidth, outsideHeight
}

// Run configure et lance la boucle Ebitengine.
func Run(a *App) error {
	ebiten.SetWindowTitle(windowTitle)
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	ebiten.SetWindowSize(windowScaleX*320, windowScaleY*200)
	return ebiten.RunGame(a)
}
