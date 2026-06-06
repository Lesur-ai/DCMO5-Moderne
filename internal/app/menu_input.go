// Fichier : menu_input.go — hit-test souris du menu de pilotage.
// Utilise exactement la même géométrie que le rendu (menu_render.go) pour que
// l'item surligné corresponde toujours à l'item cliqué.
package app

import "github.com/Lesur-ai/dcmo5/internal/menu"

// menuItemAt retourne l'index de l'item sous le curseur (mx,my) en coordonnées
// logiques, ou -1 si le curseur n'est sur aucun item.
func menuItemAt(m *menu.Model, mx, my int) int {
	x, y, w, h := contentRect(m.State())
	// Tolérance horizontale = largeur du fond de sélection.
	if mx < x-6 || mx > x+w+6 {
		return -1
	}
	switch m.State() {
	case menu.StateMain:
		startY := mainStartY(y)
		for i := range m.MainLabels() {
			top := mainItemTop(i, startY)
			if my >= top && my < top+menuLineH {
				return i
			}
		}
		return -1
	case menu.StateBrowse:
		startY := browseStartY(y)
		first, count := m.VisibleWindow(browseMaxVisible(y, h))
		if my < startY {
			return -1
		}
		row := (my - startY) / menuLineH
		if row < 0 || row >= count {
			return -1
		}
		return first + row
	}
	return -1
}
