// Fichier : menu_input.go — hit-test souris du menu de pilotage.
// Utilise exactement la même géométrie que le rendu (menu_render.go) pour que
// l'item surligné corresponde toujours à l'item cliqué.
package app

import "github.com/Lesur-ai/dcmo5/internal/menu"

// menuItemAt retourne l'index de l'item sous le curseur (mx,my) en coordonnées
// logiques, ou -1 si le curseur n'est sur aucun item.
func menuItemAt(m *menu.Model, mx, my int) int {
	x, y, w, h := menuContentRect()
	// Tolérance horizontale = largeur de la barre de sélection.
	if mx < x-4 || mx > x+w+4 {
		return -1
	}
	switch m.State() {
	case menu.StateMain:
		startY := mainStartY(y)
		row := rowAt(my, startY)
		if row < 0 || row >= len(m.MainLabels()) {
			return -1
		}
		return row
	case menu.StateBrowse:
		startY := browseStartY(y)
		first, count := m.VisibleWindow(browseMaxVisible(y, h))
		row := rowAt(my, startY)
		if row < 0 || row >= count {
			return -1
		}
		return first + row
	}
	return -1
}

// rowAt retourne l'indice de ligne pour l'ordonnée my à partir de startY, ou -1
// si my est au-dessus de la première ligne.
func rowAt(my, startY int) int {
	if my < startY {
		return -1
	}
	return (my - startY) / menuLineH
}
