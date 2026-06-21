// Fichier : video.go — décodage vidéo gate-array (5 modes) vers un framebuffer
// logique FIXE de 672×216, et palette programmable EF9369.
//
// Référence : dcto8dvideo.c (Daniel Coulom, GPLv3) — Decode320x16/320x4/
// 320x4special/160x16/640x2, Palette(), Displaysegment/Displayborder.
//
// La réf décode au fil du balayage dans une surface SDL redimensionnable via une
// table xpixel[] de mise à l'échelle. Ici la frame est FIXE à XBITMAP×YBITMAP :
// en fixant la largeur cible à XBITMAP, xpixel[i] devient l'identité, donc chaque
// octet vidéo produit exactement 16 pixels logiques (un « segment »). On génère
// la trame entière depuis la mémoire (indépendamment du balayage courant), comme
// le MO5 : 42 segments par ligne (0 et 41 = bordure, 1–40 = 40 octets actifs),
// 216 lignes (8 bordure haute + 200 actives + 8 bordure basse).
package gatearray

const (
	xbitmap = 672 // largeur du framebuffer logique (42 segments × 16 px)
	ybitmap = 216 // hauteur (8 + 200 + 8)

	activeBytes = 40  // octets vidéo par ligne (segments 1–40)
	segPixels   = 16  // pixels logiques par segment/octet
	activeLines = 200 // lignes actives
)

// intens est la table de correction gamma de la datasheet EF9369 (réf C : intens[]).
var intens = [16]int{80, 118, 128, 136, 142, 147, 152, 156, 160, 163, 166, 169, 172, 175, 178, 180}

// FrameSize retourne la taille (fixe) du framebuffer logique gate-array.
func (g *GateArray) FrameSize() (w, h int) { return xbitmap, ybitmap }

// rgbaFromRVB convertit une couleur EF9369 (r,v,b ∈ [0,15]) en RGBA Ebitengine
// (0xAABBGGRR) avec la correction gamma datasheet. Réf C : Palette() —
// composante = 2*(intens[k]-64)+16.
func rgbaFromRVB(r, v, b int) uint32 {
	rc := uint32(2*(intens[r&0x0f]-64) + 16)
	gc := uint32(2*(intens[v&0x0f]-64) + 16)
	bc := uint32(2*(intens[b&0x0f]-64) + 16)
	return 0xFF000000 | bc<<16 | gc<<8 | rc
}

// refreshPalette recalcule toute la palette rendue (pcolor) depuis x7da. Appelée
// au reset ; en fonctionnement, paletteWrite met à jour pcolor entrée par entrée
// (uniquement à l'écriture du 2e octet, pour respecter le latch EF9369).
func (g *GateArray) refreshPalette() {
	for n := 0; n < 16; n++ {
		lo := int(g.x7da[2*n])
		hi := int(g.x7da[2*n+1])
		g.pcolor[n] = rgbaFromRVB(lo&0x0f, (lo>>4)&0x0f, hi&0x0f)
	}
}

// DecodeFrame rend le framebuffer courant dans dst (≥ xbitmap*ybitmap). Chaque
// ligne : bordure pleine hors zone active, sinon bordure gauche + 40 octets
// décodés selon le mode + bordure droite. Utilise la palette rendue latchée
// (g.pcolor), maintenue par paletteWrite/refreshPalette.
func (g *GateArray) DecodeFrame(dst []uint32) {
	if len(dst) < xbitmap*ybitmap {
		return
	}
	border := g.pcolor[g.bordercolor&0x0f]

	for y := 0; y < ybitmap; y++ {
		row := y * xbitmap
		// videolinenumber affiché : 48..263. Zone active = 56..255 (réf C).
		if vln := y + 48; vln < 56 || vln > 255 {
			for x := 0; x < xbitmap; x++ {
				dst[row+x] = border
			}
			continue
		}
		line := (y + 48) - 56 // ligne active 0..199
		px := row
		// Bordure gauche (segment 0).
		for x := 0; x < segPixels; x++ {
			dst[px] = border
			px++
		}
		// 40 octets actifs (segments 1..40).
		for o := 0; o < activeBytes; o++ {
			idx := line*activeBytes + o
			colorByte := g.ram[g.pagevideoBase+idx]
			formByte := g.ram[g.pagevideoBase+(idx|0x2000)]
			g.decodeByte(dst[px:px+segPixels], colorByte, formByte, &g.pcolor)
			px += segPixels
		}
		// Bordure droite (segment 41).
		for x := 0; x < segPixels; x++ {
			dst[px] = border
			px++
		}
	}
}

// decodeByte écrit les 16 pixels logiques d'un octet vidéo dans out, selon le mode
// courant. colorByte = octet « couleurs » (pagevideo[idx]), formByte = octet
// « formes » (pagevideo[idx|0x2000]). Traduction fidèle des Decode* de la réf C.
func (g *GateArray) decodeByte(out []uint32, colorByte, formByte byte, pal *[16]uint32) {
	switch g.vmode {
	case mode320x16:
		// Standard : couleur fond/forme codées dans colorByte (3 bits + 1 bit
		// « pastel » inversé), masque de pixels dans formByte. 8 pixels × 2.
		color := int(colorByte)
		shape := int(formByte)
		c0 := (color & 0x07) | ((^color & 0x80) >> 4)        // fond
		c1 := ((color >> 3) & 0x07) | ((^color & 0x40) >> 3) // forme
		for i := 7; i >= 0; i-- {
			ci := c0
			if (shape>>uint(i))&1 != 0 {
				ci = c1
			}
			col := pal[ci&0x0f]
			out[2*(7-i)] = col
			out[2*(7-i)+1] = col
		}
	case mode320x4:
		// bitmap4 : 2 bits par pixel (formByte = poids fort, colorByte = faible).
		c0 := int(formByte)
		c1 := int(colorByte)
		for i := 7; i >= 0; i-- {
			ci := ((c0 << 1) >> uint(i) & 2) | (c1 >> uint(i) & 1)
			col := pal[ci&0x0f]
			out[2*(7-i)] = col
			out[2*(7-i)+1] = col
		}
	case mode320x4special:
		// bitmap4 spécial : 16 bits → 8 pixels de 2 bits, × 2.
		c0 := (int(formByte) << 8) | (int(colorByte) & 0xff)
		j := 0
		for i := 14; i >= 0; i -= 2 {
			col := pal[(c0>>uint(i))&3]
			out[2*j] = col
			out[2*j+1] = col
			j++
		}
	case mode160x16:
		// bitmap16 : 16 bits → 4 pixels de 4 bits, × 4.
		c0 := (int(formByte) << 8) | (int(colorByte) & 0xff)
		j := 0
		for i := 12; i >= 0; i -= 4 {
			col := pal[(c0>>uint(i))&0x0f]
			for k := 0; k < 4; k++ {
				out[4*j+k] = col
			}
			j++
		}
	case mode640x2:
		// 80 colonnes : 16 bits → 16 pixels de 1 bit.
		c0 := (int(formByte) << 8) | (int(colorByte) & 0xff)
		for i := 15; i >= 0; i-- {
			ci := 0
			if (c0>>uint(i))&1 != 0 {
				ci = 1
			}
			out[15-i] = pal[ci]
		}
	}
}
