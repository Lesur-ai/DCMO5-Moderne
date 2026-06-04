// Package spec centralise les constantes matérielles du Thomson MO5.
// Il ne contient aucune logique mutable.
package spec

// Horloge CPU
const (
	CPUClockHz = 1_000_000 // Motorola 6809 à 1 MHz nominal
)

// Framebuffer logique (ref: dcmo5video.c xbitmap=336, ybitmap=216)
const (
	FrameWidth  = 336 // 320 pixels actifs + 2 bordures de 8 px
	FrameHeight = 216 // 200 lignes actives + 2 bordures de 8 px
)

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

// Adresses mémoire significatives
const (
	AddrVideoColors uint16 = 0x0000 // base RAM vidéo couleurs
	AddrVideoForms  uint16 = 0x2000 // base RAM vidéo formes
	AddrUserRAM     uint16 = 0x4000 // base RAM utilisateur fixe
	AddrROMBank     uint16 = 0xA000 // base ROM banque / cartouche
	AddrROMSys      uint16 = 0xC000 // base ROM système

	// Vecteurs 6809 (big-endian, au sommet de la ROM)
	VectorReset uint16 = 0xFFFE
	VectorNMI   uint16 = 0xFFFC
	VectorSWI   uint16 = 0xFFFA
	VectorIRQ   uint16 = 0xFFF8
	VectorFIRQ  uint16 = 0xFFF6
	VectorSWI2  uint16 = 0xFFF4
	VectorSWI3  uint16 = 0xFFF2
)

// Touches clavier MO5 (ref: dcmo5global.h MO5KEY_MAX=58)
const (
	KeyMax    = 58 // nombre de touches du clavier MO5
	JoyKeyMax = 10 // nombre total de contacts des deux manettes
	JoyMax    = 2  // nombre de manettes
)

// Paramètres cassette .k7
const (
	K7BaudRate = 1200 // débit nominal cassette (bauds)
)

// Paramètres disquette .fd (CD90-640 compatible)
const (
	FDSectorSize = 256                                           // octets par secteur
	FDSectors    = 16                                            // secteurs par piste
	FDTracks     = 40                                            // pistes par face
	FDFaces      = 2                                             // nombre de faces
	FDDiskSize   = FDFaces * FDTracks * FDSectors * FDSectorSize // 327 680 octets
)

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

// PaletteColor retourne une copie des composantes RGB brutes (index [0,15])
// de l'entrée i de la palette (avant correction gamma).
func PaletteColor(i int) [3]uint8 {
	return palette[i]
}

// PaletteLen retourne le nombre d'entrées de la palette (19).
func PaletteLen() int {
	return len(palette)
}

// GammaLookup retourne la valeur corrigée pour le niveau d'intensité n ∈ [0,15].
func GammaLookup(n int) uint8 {
	return gammaTable[n]
}

// GammaLen retourne la taille de la table gamma (16).
func GammaLen() int {
	return len(gammaTable)
}
