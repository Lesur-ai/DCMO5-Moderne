package gatearray_test

// Tests du décodage vidéo gate-array (#113) : dimensions, bordure, palette
// programmable EF9369, et golden par mode. Boîte noire via les ports e7c3/e7da/
// e7db/e7dc/e7dd + Write8 dans la RAM vidéo, puis DecodeFrame.

import (
	"testing"

	"github.com/Lesur-ai/dcmo5/internal/machine/gatearray"
)

const (
	xb = 672
	yb = 216
)

// intensRef reproduit la table gamma EF9369 pour calculer la couleur RGBA attendue.
var intensRef = [16]int{80, 118, 128, 136, 142, 147, 152, 156, 160, 163, 166, 169, 172, 175, 178, 180}

func wantColor(r, v, b int) uint32 {
	rc := uint32(2*(intensRef[r]-64) + 16)
	gc := uint32(2*(intensRef[v]-64) + 16)
	bc := uint32(2*(intensRef[b]-64) + 16)
	return 0xFF000000 | bc<<16 | gc<<8 | rc
}

// setColor programme la couleur n (r,v,b ∈ [0,15]) de la palette via e7db+e7da.
func setColor(g *gatearray.GateArray, n, r, v, b int) {
	g.Write8(0xE7DB, byte(2*n))      // index dans x7da
	g.Write8(0xE7DA, byte(r|(v<<4))) // octet pair : r|v
	g.Write8(0xE7DA, byte(b))        // octet impair : b
}

// writeColorByte / writeFormByte écrivent l'octet « couleurs » / « formes » du
// premier octet vidéo (ligne 0, octet 0) via la RAM vidéo CPU (e7c3 page bit0).
func writeColorByte(g *gatearray.GateArray, v byte) {
	g.Write8(0xE7C3, 0x00) // page couleurs → 0x4000 = ram[0]
	g.Write8(0x4000, v)
}

func writeFormByte(g *gatearray.GateArray, v byte) {
	g.Write8(0xE7C3, 0x01) // page formes → 0x4000 = ram[0x2000]
	g.Write8(0x4000, v)
}

// firstActivePixel : index dans le framebuffer du pixel k (0..15) du premier
// octet actif (ligne active 0 = y 8 ; octet 0 commence après la bordure gauche 16).
func firstActivePixel(k int) int { return 8*xb + 16 + k }

func newFrame() []uint32 { return make([]uint32, xb*yb) }

func TestFrameSize(t *testing.T) {
	g := newGA()
	if w, h := g.FrameSize(); w != xb || h != yb {
		t.Errorf("FrameSize = %dx%d, want %dx%d", w, h, xb, yb)
	}
}

func TestDecodeFrameFillsAll(t *testing.T) {
	g := newGA()
	fb := newFrame()
	g.DecodeFrame(fb)
	// Tous les pixels ont alpha 0xFF (aucune case laissée à zéro).
	for i, px := range fb {
		if px>>24 != 0xFF {
			t.Fatalf("pixel %d alpha = 0x%02X, want 0xFF (case non rendue)", i, px>>24)
		}
	}
}

func TestBorderColor(t *testing.T) {
	g := newGA()
	// Bordure = couleur 5 ; programme la couleur 5 et e7dd (bits0-3 = bordure).
	setColor(g, 5, 9, 4, 2)
	g.Write8(0xE7DD, 0x05)
	fb := newFrame()
	g.DecodeFrame(fb)
	want := wantColor(9, 4, 2)
	// Coin (0,0) = bordure haute ; pixel (0, ligne active 0) = bordure gauche.
	if fb[0] != want {
		t.Errorf("bordure haute (0,0) = 0x%08X, want 0x%08X", fb[0], want)
	}
	if v := fb[8*xb+0]; v != want {
		t.Errorf("bordure gauche (0,8) = 0x%08X, want 0x%08X", v, want)
	}
}

func TestPaletteProgrammable(t *testing.T) {
	g := newGA()
	// Mode standard ; couleur fond 1, forme 2 distinctes ; octet color=0xD1
	// (bg=1, fg=2 sans bit pastel), forme 0x80 (1er pixel = forme).
	setColor(g, 1, 1, 0, 0)
	setColor(g, 2, 0, 2, 0)
	g.Write8(0xE7DC, 0x00) // mode 320x16 standard
	writeColorByte(g, 0xD1)
	writeFormByte(g, 0x80)
	fb := newFrame()
	g.DecodeFrame(fb)
	// Pixels 0-1 = forme (couleur 2) ; pixels 2-15 = fond (couleur 1).
	if v := fb[firstActivePixel(0)]; v != wantColor(0, 2, 0) {
		t.Errorf("pixel forme = 0x%08X, want 0x%08X (couleur 2)", v, wantColor(0, 2, 0))
	}
	if v := fb[firstActivePixel(2)]; v != wantColor(1, 0, 0) {
		t.Errorf("pixel fond = 0x%08X, want 0x%08X (couleur 1)", v, wantColor(1, 0, 0))
	}
	// Reprogrammer la couleur 2 doit changer le pixel forme (palette dynamique).
	setColor(g, 2, 0, 0, 7)
	g.DecodeFrame(fb)
	if v := fb[firstActivePixel(0)]; v != wantColor(0, 0, 7) {
		t.Errorf("après reprog : pixel forme = 0x%08X, want 0x%08X", v, wantColor(0, 0, 7))
	}
}

func TestDecode320x16Doubling(t *testing.T) {
	g := newGA()
	setColor(g, 1, 1, 0, 0) // fond
	setColor(g, 2, 0, 2, 0) // forme
	g.Write8(0xE7DC, 0x00)
	writeColorByte(g, 0xD1) // bg=1 fg=2
	writeFormByte(g, 0xC0)  // 2 bits de poids fort = forme → 4 px forme, 12 px fond
	fb := newFrame()
	g.DecodeFrame(fb)
	for k := 0; k < 4; k++ {
		if v := fb[firstActivePixel(k)]; v != wantColor(0, 2, 0) {
			t.Errorf("320x16 pixel %d = 0x%08X, want forme (chaque bit = 2 px)", k, v)
		}
	}
	for k := 4; k < 16; k++ {
		if v := fb[firstActivePixel(k)]; v != wantColor(1, 0, 0) {
			t.Errorf("320x16 pixel %d = 0x%08X, want fond", k, v)
		}
	}
}

func TestDecode640x2(t *testing.T) {
	g := newGA()
	setColor(g, 0, 0, 0, 0) // couleur 0
	setColor(g, 1, 7, 7, 7) // couleur 1
	g.Write8(0xE7DC, 0x2a)  // mode 640x2 (80 colonnes)
	// 16 bits = form<<8 | color. form=0x80 (bit15), color=0x01 (bit0).
	writeColorByte(g, 0x01)
	writeFormByte(g, 0x80)
	fb := newFrame()
	g.DecodeFrame(fb)
	// 1 pixel par bit : pixel 0 (bit15) et pixel 15 (bit0) = couleur 1, reste = 0.
	if v := fb[firstActivePixel(0)]; v != wantColor(7, 7, 7) {
		t.Errorf("640x2 pixel 0 = 0x%08X, want couleur 1", v)
	}
	if v := fb[firstActivePixel(15)]; v != wantColor(7, 7, 7) {
		t.Errorf("640x2 pixel 15 = 0x%08X, want couleur 1", v)
	}
	if v := fb[firstActivePixel(1)]; v != wantColor(0, 0, 0) {
		t.Errorf("640x2 pixel 1 = 0x%08X, want couleur 0", v)
	}
}

func TestDecode160x16(t *testing.T) {
	g := newGA()
	for n := 0; n < 16; n++ {
		setColor(g, n, n, n, n) // couleur n = (n,n,n)
	}
	g.Write8(0xE7DC, 0x7b) // mode 160x16 (bitmap16)
	// 16 bits → 4 pixels de 4 bits (chaque ×4). c0 = form<<8 | color.
	// Choisir form=0x12, color=0x34 → c0=0x1234 → nibbles 1,2,3,4 (i=12,8,4,0).
	writeColorByte(g, 0x34)
	writeFormByte(g, 0x12)
	fb := newFrame()
	g.DecodeFrame(fb)
	wantNibbles := []int{1, 2, 3, 4}
	for p, n := range wantNibbles {
		for k := 0; k < 4; k++ { // chaque pixel logique répété 4×
			if v := fb[firstActivePixel(p*4+k)]; v != wantColor(n, n, n) {
				t.Errorf("160x16 pixel %d (nibble %d) = 0x%08X, want couleur %d", p*4+k, n, v, n)
			}
		}
	}
}

func TestVideoModeSwitch(t *testing.T) {
	g := newGA()
	setColor(g, 0, 0, 0, 0)
	setColor(g, 1, 7, 7, 7)
	writeColorByte(g, 0xFF)
	writeFormByte(g, 0xFF)
	fb := newFrame()

	// En 640x2, color/form=0xFF → c0=0xFFFF → tous les pixels = couleur 1.
	g.Write8(0xE7DC, 0x2a)
	g.DecodeFrame(fb)
	if v := fb[firstActivePixel(3)]; v != wantColor(7, 7, 7) {
		t.Fatalf("640x2 plein = 0x%08X, want couleur 1", v)
	}
	// Bascule en 320x4 : décodage différent (2 bits/pixel) → couleur d'index 3.
	setColor(g, 3, 1, 2, 3)
	g.Write8(0xE7DC, 0x21) // mode 320x4
	g.DecodeFrame(fb)
	if v := fb[firstActivePixel(0)]; v != wantColor(1, 2, 3) {
		t.Errorf("320x4 (form=color=0xFF → index 3) = 0x%08X, want couleur 3", v)
	}
}

// TestPaletteLatch vérifie la sémantique EF9369 : la couleur rendue n'est mise à
// jour qu'à l'écriture du 2e octet (index impair). Écrire seulement l'octet pair
// ne doit PAS changer la couleur affichée (anti couleur transitoire).
func TestPaletteLatch(t *testing.T) {
	g := newGA()
	setColor(g, 4, 1, 2, 3) // couleur 4 complète = (1,2,3)
	g.Write8(0xE7DD, 0x04)  // bordure = couleur 4 (pour l'observer en fb[0])

	// Écrire SEULEMENT l'octet pair de la couleur 4 (nouveaux r/v), pas l'impair.
	g.Write8(0xE7DB, byte(2*4))
	g.Write8(0xE7DA, byte(7|(7<<4)))
	fb := newFrame()
	g.DecodeFrame(fb)
	if v := fb[0]; v != wantColor(1, 2, 3) {
		t.Errorf("octet pair seul : bordure = 0x%08X, want 0x%08X (ancienne couleur latchée)", v, wantColor(1, 2, 3))
	}
	// Écrire l'octet impair → la couleur se met à jour.
	g.Write8(0xE7DA, byte(5))
	g.DecodeFrame(fb)
	if v := fb[0]; v != wantColor(7, 7, 5) {
		t.Errorf("après octet impair : bordure = 0x%08X, want 0x%08X (latch validé)", v, wantColor(7, 7, 5))
	}
}

func TestRenderVideoLineKeepsPaletteHistory(t *testing.T) {
	g := newGA()
	setColor(g, 1, 1, 0, 0)
	setColor(g, 2, 0, 2, 0)

	g.Write8(0xE7DD, 0x01)
	g.RenderVideoLine(48)
	g.Write8(0xE7DD, 0x02)
	g.RenderVideoLine(49)

	fb := newFrame()
	g.DecodeFrame(fb)
	if v := fb[0]; v != wantColor(1, 0, 0) {
		t.Fatalf("ligne déjà balayée recolorée = 0x%08X, want ancienne palette rouge", v)
	}
	if v := fb[xb]; v != wantColor(0, 2, 0) {
		t.Fatalf("ligne suivante = 0x%08X, want nouvelle palette verte", v)
	}
}

func TestRenderVideoSegmentsKeepsPaletteHistoryWithinLine(t *testing.T) {
	g := newGA()
	setColor(g, 1, 1, 0, 0)
	setColor(g, 2, 0, 2, 0)

	g.Write8(0xE7DD, 0x01)
	g.RenderVideoSegments(48, 11) // segmentmax=1 : seul le segment 0 est fige.
	g.Write8(0xE7DD, 0x02)
	g.RenderVideoSegments(48, 27) // segments suivants figes avec la nouvelle bordure.

	fb := newFrame()
	g.DecodeFrame(fb)
	if v := fb[0]; v != wantColor(1, 0, 0) {
		t.Fatalf("segment déjà balayé recoloré = 0x%08X, want ancienne palette rouge", v)
	}
	if v := fb[16]; v != wantColor(0, 2, 0) {
		t.Fatalf("segment suivant = 0x%08X, want nouvelle palette verte", v)
	}
}
