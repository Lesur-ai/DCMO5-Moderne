package core_test

import (
	"testing"

	"github.com/Lesur-ai/dcmo5/internal/core"
)

// ── Taille et structure du framebuffer ────────────────────────────────────────

func TestFramebufferDimensions(t *testing.T) {
	m := newMachine(t)
	fb := m.Framebuffer()
	want := core.FrameWidth * core.FrameHeight
	if len(fb) != want {
		t.Errorf("Framebuffer: len = %d, want %d", len(fb), want)
	}
}

func TestFramebufferAlpha(t *testing.T) {
	// Chaque pixel doit avoir alpha = 0xFF (opaque).
	m := newMachine(t)
	fb := m.Framebuffer()
	for i, px := range fb {
		if px>>24 != 0xFF {
			t.Errorf("pixel[%d] alpha = 0x%02X, want 0xFF", i, px>>24)
			break
		}
	}
}

// ── Bordure couleur ───────────────────────────────────────────────────────────

func TestFramebuffer_BorderColor(t *testing.T) {
	// Avec RAM vidéo tout à zéro, la bordure utilise la couleur 0 (noir MO5 = 0,0,0).
	m := newMachine(t)
	fb := m.Framebuffer()
	// Pixel (0, 0) doit être la couleur de bordure (noir avec gamma = 0x00000000 + alpha)
	// core.GammaLookup(0) = 0, donc RGBA = 0xFF000000
	borderPixel := fb[0]
	if borderPixel != 0xFF000000 {
		t.Errorf("bordure pixel (0,0) = 0x%08X, want 0xFF000000 (noir)", borderPixel)
	}
}

func TestFramebuffer_BorderRows(t *testing.T) {
	// Les 8 premières et 8 dernières lignes sont entièrement de la couleur bordure.
	m := newMachine(t)
	fb := m.Framebuffer()
	borderColor := fb[0] // couleur de référence ligne 0

	for y := 0; y < 8; y++ {
		for x := 0; x < core.FrameWidth; x++ {
			if fb[y*core.FrameWidth+x] != borderColor {
				t.Errorf("bordure haute (%d,%d): couleur inattendue", x, y)
				return
			}
		}
	}
	for y := core.FrameHeight - 8; y < core.FrameHeight; y++ {
		for x := 0; x < core.FrameWidth; x++ {
			if fb[y*core.FrameWidth+x] != borderColor {
				t.Errorf("bordure basse (%d,%d): couleur inattendue", x, y)
				return
			}
		}
	}
}

// ── Déterminisme ─────────────────────────────────────────────────────────────

func TestFramebuffer_Deterministic(t *testing.T) {
	m := newMachine(t)
	fb1 := m.Framebuffer()
	fb2 := m.Framebuffer()
	for i := range fb1 {
		if fb1[i] != fb2[i] {
			t.Errorf("framebuffer non déterministe au pixel %d", i)
			return
		}
	}
}

func TestFramebuffer_RamChangeAffectsPixels(t *testing.T) {
	// Écrire une valeur non nulle en RAM vidéo couleurs doit changer les pixels actifs.
	m := newMachine(t)
	fb0 := m.Framebuffer()
	// Écrire en RAM vidéo couleurs ligne 0, premier octet (CPU 0x0000 = ram[0x0000])
	// Cela correspond à la ligne active 0, colonne 0.
	m.Write8(0x0000, 0x71) // fg=7 (blanc), bg=1 (rouge)
	fb1 := m.Framebuffer()
	// Les pixels actifs de la ligne 8 (première ligne active) doivent avoir changé.
	changed := false
	for x := 8; x < 8+320; x++ {
		if fb0[8*core.FrameWidth+x] != fb1[8*core.FrameWidth+x] {
			changed = true
			break
		}
	}
	if !changed {
		t.Error("écriture en RAM vidéo n'a pas modifié les pixels actifs correspondants")
	}
}

// ── Contenu palette ───────────────────────────────────────────────────────────

func TestFramebuffer_ForegroundColor(t *testing.T) {
	// Forcer une ligne avec couleur fg=7 (blanc) et forme=0xFF (pixels tous à 1).
	// Tous les pixels de la ligne doivent être de couleur 7 (blanc).
	m := newMachine(t)
	// Écrire les couleurs (CPU 0x0000 → ram[0x0000], page 0).
	for col := 0; col < 40; col++ {
		m.Write8(uint16(col), 0x70) // fg=7 (blanc), bg=0
	}
	// Écrire les formes via page 1 (CPU 0x0000 → ram[0x2000]).
	// Le rendu lit toujours ram[0x2000] indépendamment de la page active.
	m.Write8(0xA7C0, 0x01)
	for col := 0; col < 40; col++ {
		m.Write8(uint16(col), 0xFF) // tous pixels allumés
	}
	m.Write8(0xA7C0, 0x00) // repasser page 0
	fb := m.Framebuffer()
	// Couleur 7 (blanc) via spec : rgb = [15,15,15], gamma[15]=255
	wantR := uint32(core.GammaLookup(15))
	wantG := uint32(core.GammaLookup(15))
	wantB := uint32(core.GammaLookup(15))
	want := 0xFF000000 | (wantB << 16) | (wantG << 8) | wantR

	firstActiveLine := 8 // borderPx
	for x := 8; x < 8+320; x++ {
		px := fb[firstActiveLine*core.FrameWidth+x]
		if px != want {
			t.Errorf("pixel (%d,%d) = 0x%08X, want 0x%08X (blanc)", x, firstActiveLine, px, want)
			return
		}
	}
}
