// Fichier : menu_render.go — rendu Ebitengine du menu de pilotage.
// Le rendu ne décide de rien : il reflète l'état d'un *menu.Model.
package app

import (
	"image/color"

	"github.com/Lesur-ai/dcmo5/internal/menu"
	"github.com/Lesur-ai/dcmo5/internal/spec"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"golang.org/x/image/font/basicfont"
)

// menuFace est la police bitmap du menu (rétro, lisible, ASCII, sans asset
// externe ni dépendance de shaping lourde). basicfont ne couvre que l'ASCII :
// tous les libellés du menu sont donc volontairement en ASCII.
var menuFace = basicfont.Face7x13

// menuAscent est la distance haut-de-ligne → baseline (text v1 positionne le
// texte par sa baseline).
const menuAscent = 11

// Palette du menu (cohérente avec l'esthétique Thomson : bleu/cyan/orange).
var (
	menuVeil      = color.RGBA{0, 0, 0, 190}      // voile assombrissant l'écran
	menuPanel     = color.RGBA{16, 24, 64, 245}   // fond du panneau
	menuBorder    = color.RGBA{96, 160, 255, 255} // liseré
	menuTitle     = color.RGBA{120, 220, 255, 255}
	menuText      = color.RGBA{220, 230, 245, 255}
	menuDimText   = color.RGBA{150, 165, 195, 255}
	menuSelBar    = color.RGBA{230, 140, 40, 255} // barre de sélection orange
	menuSelText   = color.RGBA{20, 20, 20, 255}
	menuFooterCol = color.RGBA{150, 165, 195, 255}
)

const (
	menuLineH   = 15 // hauteur d'une ligne de texte
	menuMargin  = 14 // marge du panneau
	menuPadding = 10 // marge interne
)

// drawMenu dessine l'overlay du menu si celui-ci est ouvert.
func drawMenu(screen *ebiten.Image, m *menu.Model) {
	if !m.IsOpen() {
		return
	}
	w, h := float32(spec.FrameWidth), float32(spec.FrameHeight)
	vector.DrawFilledRect(screen, 0, 0, w, h, menuVeil, false)

	px, py := float32(menuMargin), float32(menuMargin)
	pw, ph := w-2*menuMargin, h-2*menuMargin
	// Panneau + liseré.
	vector.DrawFilledRect(screen, px-1, py-1, pw+2, ph+2, menuBorder, false)
	vector.DrawFilledRect(screen, px, py, pw, ph, menuPanel, false)

	switch m.State() {
	case menu.StateMain:
		drawMainMenu(screen, m, int(px)+menuPadding, int(py)+menuPadding, int(pw)-2*menuPadding, int(ph)-2*menuPadding)
	case menu.StateBrowse:
		drawBrowser(screen, m, int(px)+menuPadding, int(py)+menuPadding, int(pw)-2*menuPadding, int(ph)-2*menuPadding)
	}
}

func drawMainMenu(screen *ebiten.Image, m *menu.Model, x, y, w, _ int) {
	drawText(screen, "D C M O 5   -   M E N U", x, y, menuTitle)
	startY := y + menuLineH + 6
	for i, label := range m.MainLabels() {
		ly := startY + i*menuLineH
		drawItem(screen, label, x, ly, w, i == m.MainIndex())
	}
	footer := "fleches: naviguer   ENTREE: valider   ECHAP: fermer"
	footerY := startY + (len(m.MainLabels())+1)*menuLineH
	drawText(screen, footer, x, footerY, menuFooterCol)
	_ = w
}

func drawBrowser(screen *ebiten.Image, m *menu.Model, x, y, w, h int) {
	title := "CHARGER " + kindLabel(m.Kind())
	drawText(screen, title, x, y, menuTitle)
	drawText(screen, truncPath(m.CurrentDir(), w), x, y+menuLineH, menuDimText)

	startY := y + 2*menuLineH + 6
	footerY := y + h - menuLineH
	maxVisible := (footerY - startY) / menuLineH
	if maxVisible < 1 {
		maxVisible = 1
	}

	entries := m.Entries()
	sel := m.BrowseIndex()
	// Fenêtre de scroll centrée sur la sélection.
	first := 0
	if len(entries) > maxVisible {
		first = sel - maxVisible/2
		if first < 0 {
			first = 0
		}
		if first > len(entries)-maxVisible {
			first = len(entries) - maxVisible
		}
	}
	last := first + maxVisible
	if last > len(entries) {
		last = len(entries)
	}
	for i := first; i < last; i++ {
		ly := startY + (i-first)*menuLineH
		label := entries[i].Name
		if entries[i].IsDir {
			if label == ".." {
				label = "[ .. ]"
			} else {
				label = "[" + label + "]"
			}
		}
		drawItem(screen, label, x, ly, w, i == sel)
	}
	if first > 0 {
		drawText(screen, "^", x+w-10, startY, menuDimText)
	}
	if last < len(entries) {
		drawText(screen, "v", x+w-10, footerY-menuLineH, menuDimText)
	}
	drawText(screen, "ENTREE: ouvrir/charger   ECHAP: retour", x, footerY, menuFooterCol)
}

// drawItem dessine une ligne de menu, surlignée si sélectionnée.
func drawItem(screen *ebiten.Image, label string, x, y, w int, selected bool) {
	if selected {
		vector.DrawFilledRect(screen, float32(x-4), float32(y-1), float32(w+8), float32(menuLineH), menuSelBar, false)
		drawText(screen, label, x, y, menuSelText)
	} else {
		drawText(screen, label, x, y, menuText)
	}
}

// drawText dessine du texte à (x,y) (coin haut-gauche) dans la couleur donnée.
func drawText(screen *ebiten.Image, s string, x, y int, clr color.Color) {
	text.Draw(screen, s, menuFace, x, y+menuAscent, clr)
}

func kindLabel(k menu.Kind) string {
	switch k {
	case menu.KindTape:
		return "CASSETTE"
	case menu.KindDisk:
		return "DISQUETTE"
	case menu.KindCart:
		return "CARTOUCHE"
	default:
		return ""
	}
}

// truncPath tronque un chemin trop long pour la largeur disponible (en gardant
// la fin, plus informative). Largeur approximée à 7 px/caractère (basicfont).
func truncPath(path string, w int) string {
	maxChars := w / 7
	if maxChars <= 0 || len(path) <= maxChars {
		return path
	}
	if maxChars <= 3 {
		return path[len(path)-maxChars:]
	}
	return "..." + path[len(path)-(maxChars-3):]
}
