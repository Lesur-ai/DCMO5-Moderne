// Fichier : menu_render.go — rendu Ebitengine du menu de pilotage.
// Le rendu ne décide de rien : il reflète l'état d'un *menu.Model.
//
// Design (336x216 logique, police basicfont 7x13) : panneau étroit centré
// laissant voir le jeu sur les côtés, titre épuré souligné, sélection discrète
// (marqueur + fond léger), groupes séparés par un petit espace, footer tronqué
// pour ne jamais déborder.
package app

import (
	"image/color"

	"github.com/Lesur-ai/dcmo5/internal/menu"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"golang.org/x/image/font/basicfont"
)

// menuFace : police bitmap ASCII (rétro, lisible, sans dépendance de shaping).
var menuFace = basicfont.Face7x13

const (
	menuCharW    = 7  // largeur d'un glyphe basicfont
	menuAscent   = 11 // haut-de-ligne → baseline (text v1)
	menuLineH    = 14 // pas vertical entre items
	menuGroupGap = 4  // espace supplémentaire entre groupes d'items
	menuPadding  = 10 // marge interne du panneau
)

// Palette du menu (douce, esthétique Thomson bleu nuit / cyan / ambre).
var (
	menuVeil      = color.RGBA{0, 0, 0, 168}     // assombrit l'écran émulé
	menuPanel     = color.RGBA{10, 18, 42, 242}  // fond du panneau
	menuBorder    = color.RGBA{42, 82, 146, 255} // liseré externe
	menuBorderHi  = color.RGBA{122, 214, 255, 190}
	menuTitle     = color.RGBA{136, 232, 255, 255}
	menuText      = color.RGBA{232, 238, 248, 255}
	menuDimText   = color.RGBA{145, 164, 194, 255}
	menuSection   = color.RGBA{255, 196, 88, 255}
	menuSelAccent = color.RGBA{255, 176, 48, 255}
	menuSelFill   = color.RGBA{255, 176, 48, 48}
	menuFooterCol = color.RGBA{160, 176, 204, 255}
)

// menuGroupEnds : indices (inclus) après lesquels insérer un espace de groupe.
// Reflète l'ordre de mainMenu : Charger(0-2) | Ejecter(3-5) | Systeme(6-8).
var menuGroupEnds = map[int]bool{2: true, 5: true}

// panelRect retourne le rectangle du panneau selon l'état (le navigateur est
// un peu plus large pour les chemins ; le menu principal plus étroit).
func panelRect(state menu.State) (x, y, w, h int) {
	if state == menu.StateBrowse {
		return 28, 10, 280, 196
	}
	// Menu principal : panneau un peu plus haut pour loger les 10 entrées.
	return 48, 6, 240, 204
}

// contentRect retourne la zone de contenu interne (padding appliqué).
func contentRect(state menu.State) (x, y, w, h int) {
	px, py, pw, ph := panelRect(state)
	return px + menuPadding, py + menuPadding, pw - 2*menuPadding, ph - 2*menuPadding
}

// mainItemTop retourne l'ordonnée du haut de l'item i du menu principal,
// gaps de groupe inclus. Partagé par le rendu et le hit-test souris.
func mainItemTop(i, startY int) int {
	y := startY + i*menuLineH
	for g := 0; g < i; g++ {
		if menuGroupEnds[g] {
			y += menuGroupGap
		}
	}
	return y
}

// mainStartY / browseStartY : première ligne d'items selon les titres au-dessus.
func mainStartY(contentY int) int   { return contentY + menuLineH + 8 }
func browseStartY(contentY int) int { return contentY + 2*menuLineH + 6 }

// browseMaxVisible : nombre de lignes affichables dans le navigateur.
func browseMaxVisible(contentY, contentH int) int {
	footerY := contentY + contentH - menuLineH
	n := (footerY - browseStartY(contentY)) / menuLineH
	if n < 1 {
		n = 1
	}
	return n
}

// drawMenu dessine l'overlay du menu si celui-ci est ouvert.
func drawMenu(screen *ebiten.Image, m *menu.Model) {
	if !m.IsOpen() {
		return
	}
	b := screen.Bounds()
	vector.DrawFilledRect(screen, 0, 0, float32(b.Dx()), float32(b.Dy()), menuVeil, false)

	px, py, pw, ph := panelRect(m.State())
	// Liseré double (externe sombre + filet clair) pour un cadre net.
	vector.DrawFilledRect(screen, float32(px-1), float32(py-1), float32(pw+2), float32(ph+2), menuBorder, false)
	vector.DrawFilledRect(screen, float32(px), float32(py), float32(pw), float32(ph), menuPanel, false)
	vector.StrokeRect(screen, float32(px), float32(py), float32(pw), float32(ph), 1, menuBorderHi, false)

	switch m.State() {
	case menu.StateMain:
		drawMainMenu(screen, m)
	case menu.StateBrowse:
		drawBrowser(screen, m)
	}
}

func drawMainMenu(screen *ebiten.Image, m *menu.Model) {
	x, y, w, h := contentRect(menu.StateMain)
	cx := x + w/2

	drawCenteredText(screen, "DCMO5 PILOTAGE", cx, y, menuTitle)
	drawDivider(screen, cx, y+menuLineH+1, 150, menuBorderHi)

	startY := mainStartY(y)
	for i, label := range m.MainLabels() {
		drawItem(screen, label, x, mainItemTop(i, startY), w, i == m.MainIndex())
	}

	footerY := y + h - menuLineH + 2
	drawFittedText(screen, "Fleches/souris   ENTREE/clic   ECHAP", x, footerY, w, menuFooterCol)
}

func drawBrowser(screen *ebiten.Image, m *menu.Model) {
	x, y, w, h := contentRect(menu.StateBrowse)

	drawText(screen, "CHARGER "+kindLabel(m.Kind()), x, y, menuSection)
	drawFittedText(screen, m.CurrentDir(), x, y+menuLineH, w, menuDimText)

	startY := browseStartY(y)
	footerY := y + h - menuLineH + 2
	maxVisible := browseMaxVisible(y, h)

	entries := m.Entries()
	sel := m.BrowseIndex()
	first, count := m.VisibleWindow(maxVisible)
	for row := 0; row < count; row++ {
		i := first + row
		label := entries[i].Name
		if entries[i].IsDir {
			if label == ".." {
				label = "[ .. ]"
			} else {
				label = "[" + label + "]"
			}
		}
		drawItem(screen, label, x, startY+row*menuLineH, w, i == sel)
	}
	if first > 0 {
		drawText(screen, "^", x+w-menuCharW, startY, menuDimText)
	}
	if first+count < len(entries) {
		drawText(screen, "v", x+w-menuCharW, footerY-menuLineH, menuDimText)
	}
	drawFittedText(screen, "Molette/fleches   ENTREE ouvrir   ECHAP retour", x, footerY, w, menuFooterCol)
}

// drawItem dessine une ligne de menu. La sélection est marquée discrètement :
// petit accent vertical à gauche + fond très léger, sans barre pleine largeur.
func drawItem(screen *ebiten.Image, label string, x, y, w int, selected bool) {
	if selected {
		vector.DrawFilledRect(screen, float32(x-3), float32(y-1), float32(w+6), float32(menuLineH), menuSelFill, false)
		vector.DrawFilledRect(screen, float32(x-6), float32(y+2), 3, 9, menuSelAccent, false)
	}
	drawText(screen, label, x, y, menuText)
}

// drawText dessine du texte à (x,y) (coin haut-gauche) dans la couleur donnée.
func drawText(screen *ebiten.Image, s string, x, y int, clr color.Color) {
	text.Draw(screen, s, menuFace, x, y+menuAscent, clr)
}

// drawCenteredText centre le texte horizontalement autour de cx.
func drawCenteredText(screen *ebiten.Image, s string, cx, y int, clr color.Color) {
	drawText(screen, s, cx-len(s)*menuCharW/2, y, clr)
}

// drawFittedText tronque le texte pour qu'il tienne dans maxW pixels.
func drawFittedText(screen *ebiten.Image, s string, x, y, maxW int, clr color.Color) {
	maxChars := maxW / menuCharW
	if maxChars > 0 && len(s) > maxChars {
		if maxChars <= 3 {
			s = s[:maxChars]
		} else {
			s = s[:maxChars-3] + "..."
		}
	}
	drawText(screen, s, x, y, clr)
}

// drawDivider dessine un trait horizontal fin centré sur cx.
func drawDivider(screen *ebiten.Image, cx, y, width int, clr color.Color) {
	vector.DrawFilledRect(screen, float32(cx-width/2), float32(y), float32(width), 1, clr, false)
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
