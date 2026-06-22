// Package ui hébergera l'IHM ebitenui du projet (Lot #117 : launcher + overlay
// data-driven). Ce fichier n'ancre que la dépendance ebitenui — il ne contient
// AUCUNE logique d'IHM (launcher/overlay aux PR suivantes du lot).
//
// But : (1) figer ebitenui dans go.mod (un import est nécessaire, sinon `go mod
// tidy` l'élague) ; (2) garantir, dès cette PR, que la dépendance compile en
// CGO_ENABLED=0 (cible Windows de la distribution) — la CI ajoute un garde-fou
// de cross-compilation qui exerce ce paquet.
package ui

import (
	// Imports anonymes : on ancre les paquets ebitenui que l'IHM utilisera, sans
	// encore les employer. La construction effective des widgets arrive aux PR
	// suivantes (uimodel pur, puis launcher, puis overlay).
	_ "github.com/ebitenui/ebitenui"
	_ "github.com/ebitenui/ebitenui/widget"
)
