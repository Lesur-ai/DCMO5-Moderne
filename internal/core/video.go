// Fichier : video.go — génération du framebuffer MO5 depuis la RAM vidéo.
package core

import "github.com/Lesur-ai/dcmo5/internal/spec"

const (
	borderPx    = 8   // largeur bordure en pixels logiques
	activeLines = 200 // lignes actives MO5
	activeCols  = 40  // octets de couleurs par ligne (40 × 8 pixels = 320 px)
)

// Framebuffer génère le framebuffer RGBA 336×216 depuis la RAM vidéo courante.
// Les pixels sont encodés RGBA little-endian (0xAABBGGRR) pour Ebitengine.
//
// Layout :
//
//	Lignes 0..7          : bordure haute (bordercolor)
//	Lignes 8..207        : 200 lignes actives MO5
//	Lignes 208..215      : bordure basse (bordercolor)
//	Colonnes 0..7        : bordure gauche
//	Colonnes 8..327      : 320 pixels actifs (40 octets × 8 bits)
//	Colonnes 328..335    : bordure droite
func (m *Machine) Framebuffer() []uint32 {
	fb := make([]uint32, spec.FrameWidth*spec.FrameHeight)
	m.FramebufferInto(fb)
	return fb
}

// FramebufferInto rend le framebuffer dans dst (taille exacte
// spec.FrameWidth*spec.FrameHeight) sans allocation, pour éviter la pression GC
// à 60 Hz (les allocations par frame provoquent des pauses GC qui glitchent
// l'audio). dst trop petit est ignoré.
func (m *Machine) FramebufferInto(fb []uint32) {
	if len(fb) < spec.FrameWidth*spec.FrameHeight {
		return
	}
	borderRGBA := m.paletteRGBA(int(m.port[0]>>1) & 0x0F)

	// Bordure haute (lignes 0–7)
	for y := 0; y < borderPx; y++ {
		fillRow(fb, y, borderRGBA)
	}

	// 200 lignes actives (lignes 8–207)
	for line := 0; line < activeLines; line++ {
		y := borderPx + line
		rowBase := y * spec.FrameWidth
		// Bordure gauche
		for x := 0; x < borderPx; x++ {
			fb[rowBase+x] = borderRGBA
		}
		// 40 octets → 320 pixels actifs
		m.composeLine(fb, rowBase+borderPx, line*activeCols)
		// Bordure droite
		for x := borderPx + 320; x < spec.FrameWidth; x++ {
			fb[rowBase+x] = borderRGBA
		}
	}

	// Bordure basse (lignes 208–215)
	for y := borderPx + activeLines; y < spec.FrameHeight; y++ {
		fillRow(fb, y, borderRGBA)
	}
}

// composeLine remplit 320 pixels (40 octets) dans fb à partir du décalage dst.
// ramOffset est l'index dans ram[] des couleurs de cette ligne.
//
// Le rendu est TOUJOURS indépendant du bit de sélection de page CPU (port[0]&1).
// Ce bit n'affecte que l'accès CPU via le bus (Read8/Write8), pas le rendu hardware.
// Ref: dcmo5video.c ComposeMO5line() — ram[a]=couleurs, ram[0x2000|a]=formes.
// Couleurs : toujours à ram[0x0000-0x1FFF], formes : toujours à ram[0x2000-0x3FFF].
func (m *Machine) composeLine(fb []uint32, dst, ramOffset int) {
	const (
		colorBase = 0x0000 // couleurs : ram[0x0000-0x1FFF] (invariant)
		formsBase = 0x2000 // formes   : ram[0x2000-0x3FFF] (invariant)
	)

	for i := 0; i < activeCols; i++ {
		colorByte := m.ram[colorBase+ramOffset+i]
		bg := int(colorByte & 0x0F)        // nibble bas = couleur fond (pixel=0)
		fg := int((colorByte >> 4) & 0x0F) // nibble haut = couleur tracé (pixel=1)
		formByte := m.ram[formsBase+ramOffset+i]

		bgRGBA := m.paletteRGBA(bg)
		fgRGBA := m.paletteRGBA(fg)

		for bit := 7; bit >= 0; bit-- {
			if (formByte>>uint(bit))&1 == 1 {
				fb[dst] = fgRGBA
			} else {
				fb[dst] = bgRGBA
			}
			dst++
		}
	}
}

// ── Signaux de synchronisation vidéo ─────────────────────────────────────────

// initn retourne 0x80 si le balayage est dans la zone active de trame,
// 0 sinon. Utilisé par les ports 0xA7C3, 0xA7D8, 0xA7E7.
// Ref: dcmo5emulation.c Initn() — zone active = lignes 56..255
func (m *Machine) initn() int {
	n := m.videolinenumber
	c := m.videolinecycle
	if n < 56 || n > 255 {
		return 0
	}
	if n == 56 && c < 24 {
		return 0
	}
	if n == 255 && c > 62 {
		return 0
	}
	return 0x80
}

// iniln retourne 0x20 si le balayage est dans la zone active de ligne,
// 0 sinon. Utilisé par le port 0xA7E6.
// Ref: dcmo5emulation.c Iniln() — zone active = cycles 23..63
func (m *Machine) iniln() int {
	if m.videolinecycle < 23 {
		return 0
	}
	return 0x20
}

// paletteRGBA retourne la couleur RGBA d'un index palette Thomson (0–18)
// avec correction gamma appliquée. Format : 0xAABBGGRR (Ebitengine RGBA).
func (m *Machine) paletteRGBA(idx int) uint32 {
	if idx < 0 || idx >= spec.PaletteLen() {
		idx = 0
	}
	rgb := spec.PaletteColor(idx)
	r := uint32(spec.GammaLookup(int(rgb[0])))
	g := uint32(spec.GammaLookup(int(rgb[1])))
	b := uint32(spec.GammaLookup(int(rgb[2])))
	return 0xFF000000 | (b << 16) | (g << 8) | r
}

// fillRow remplit une ligne entière avec une couleur uniforme.
func fillRow(fb []uint32, y int, color uint32) {
	base := y * spec.FrameWidth
	for x := 0; x < spec.FrameWidth; x++ {
		fb[base+x] = color
	}
}
