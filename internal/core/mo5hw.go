// Fichier : mo5hw.go — constantes matérielles spécifiques au Thomson MO5
// (géométrie écran, niveau audio, carte mémoire, palette, repère crayon).
//
// Elles vivaient dans internal/spec à l'époque mono-machine (v1). Depuis la v2
// multi-machines, spec ne conserve que le transverse (horloge CPU, vecteurs
// 6809, taux d'échantillonnage audio par défaut) : chaque machine porte SA
// géométrie, SA palette et SA carte mémoire. Le TO8D (famille gate-array)
// déclarera les siennes dans son propre Device, sans toucher au MO5.
package core

// Framebuffer logique (ref: dcmo5video.c xbitmap=336, ybitmap=216).
const (
	FrameWidth  = 336 // 320 pixels actifs + 2 bordures de 8 px
	FrameHeight = 216 // 200 lignes actives + 2 bordures de 8 px

	BorderWidth  = 8   // largeur de la bordure en pixels (haut/bas/gauche/droite)
	ActiveWidth  = 320 // pixels actifs horizontaux de l'écran MO5
	ActiveHeight = 200 // lignes actives de l'écran MO5
)

// AudioLevelMax est le niveau sonore maximal : le haut-parleur MO5 est piloté
// par un registre 6 bits (ports 0xA7C1/0xA7CD).
const AudioLevelMax = 0x3F

// Carte mémoire MO5 — 48 Ko de RAM physique organisée ainsi :
//
//	0x0000–0x1FFF  RAM vidéo couleurs  (8 Ko, page 0 ou 1 selon port[0]&1)
//	0x2000–0x3FFF  RAM vidéo formes    (8 Ko, même sélection de banque)
//	0x4000–0x9FFF  RAM utilisateur     (24 Ko fixe)
//	0xA000–0xBFFF  ROM banque / cart   (8 Ko, commutable)
//	0xC000–0xFFFF  ROM système         (16 Ko)
const (
	RAMTotalSize  = 0xC000 // 48 Ko RAM physique totale
	RAMVideoSize  = 0x2000 // 8 Ko par page vidéo (couleurs OU formes)
	RAMVideoPages = 2      // nombre de pages vidéo (banque 0 et banque 1)
	RAMUserOffset = 0x4000 // début RAM utilisateur fixe
	RAMUserSize   = 0x6000 // 24 Ko RAM utilisateur (0x4000–0x9FFF)

	CartSize     = 0x10000 // 4 banques × 16 Ko espace cartouche
	CartBankSize = 0x4000  // 16 Ko par banque cartouche

	PortSize = 0x40 // 64 octets ports d'E/S
)

// Adresses mémoire significatives MO5.
const (
	AddrVideoColors uint16 = 0x0000 // base RAM vidéo couleurs
	AddrVideoForms  uint16 = 0x2000 // base RAM vidéo formes
	AddrUserRAM     uint16 = 0x4000 // base RAM utilisateur fixe
	AddrROMBank     uint16 = 0xA000 // base ROM banque / cartouche
	AddrROMSys      uint16 = 0xC000 // base ROM système
)

// PenFromFramebuffer convertit une position curseur exprimée dans le repère du
// framebuffer logique (0..FrameWidth-1 / 0..FrameHeight-1, bordure incluse) vers
// le repère de l'écran actif MO5 (0..ActiveWidth-1 / 0..ActiveHeight-1) attendu
// par le crayon optique (readPenXY).
//
// Le framebuffer a une bordure symétrique de BorderWidth pixels : le pixel actif
// (0,0) est dessiné en (BorderWidth, BorderWidth). On retranche la bordure. Une
// position hors zone active produit volontairement une coordonnée négative ou
// ≥ Active{Width,Height} : readPenXY l'interprète comme « pas de détection »
// (carry positionné). On ne borne donc pas ici.
func PenFromFramebuffer(cursorX, cursorY int) (penX, penY int) {
	return cursorX - BorderWidth, cursorY - BorderWidth
}

// palette Thomson MO5 (16 couleurs utilisateur + 3 couleurs système).
// Référence: dcmo5video.c Initpalette() — composantes R,G,B sur [0,15],
// correction gamma appliquée par le rendu via GammaLookup.
// Index 0–15 : couleurs utilisateur. Index 16–18 : couleurs internes.
// 0 noir  1 rouge  2 vert   3 jaune  4 bleu   5 magenta  6 cyan   7 blanc
// 8 gris  9 rose  10 v.clair 11 j.clair 12 b.clair 13 m.clair 14 c.clair 15 orange
var palette = [19][3]uint8{
	/* 0 */ {0, 0, 0},
	/* 1 */ {15, 0, 0},
	/* 2 */ {0, 15, 0},
	/* 3 */ {15, 15, 0},
	/* 4 */ {0, 0, 15},
	/* 5 */ {15, 0, 15},
	/* 6 */ {0, 15, 15},
	/* 7 */ {15, 15, 15},
	/* 8 */ {7, 7, 7},
	/* 9 */ {10, 3, 3},
	/* 10 */ {3, 10, 3},
	/* 11 */ {10, 10, 3},
	/* 12 */ {3, 3, 10},
	/* 13 */ {10, 3, 10},
	/* 14 */ {7, 14, 14},
	/* 15 */ {11, 3, 0},
	/* 16 */ {11, 11, 11},
	/* 17 */ {14, 14, 14},
	/* 18 */ {2, 2, 2},
}

// gammaTable est la table de correction gamma utilisée par Initpalette().
// Mappe les 16 niveaux d'intensité [0,15] vers les valeurs uint8 [0,255].
var gammaTable = [16]uint8{
	0, 60, 90, 110, 130, 148, 165, 180, 193, 205, 215, 225, 230, 235, 240, 255,
}

// PaletteColor retourne une copie des composantes RGB brutes (index [0,18])
// de l'entrée i de la palette (avant correction gamma).
func PaletteColor(i int) [3]uint8 { return palette[i] }

// PaletteLen retourne le nombre d'entrées de la palette (19).
func PaletteLen() int { return len(palette) }

// GammaLookup retourne la valeur corrigée pour le niveau d'intensité n ∈ [0,15].
func GammaLookup(n int) uint8 { return gammaTable[n] }

// GammaLen retourne la taille de la table gamma (16).
func GammaLen() int { return len(gammaTable) }
