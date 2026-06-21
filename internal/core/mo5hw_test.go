package core_test

// mo5hw_test.go — constantes matérielles MO5 (déplacées depuis internal/spec au
// lot #109 : géométrie écran, carte mémoire, palette, gamma, repère crayon).

import (
	"testing"

	"github.com/Lesur-ai/dcmo5/internal/core"
)

func TestFrameConstants(t *testing.T) {
	if core.FrameWidth != 336 {
		t.Errorf("FrameWidth = %d, want 336", core.FrameWidth)
	}
	if core.FrameHeight != 216 {
		t.Errorf("FrameHeight = %d, want 216", core.FrameHeight)
	}
}

func TestRAMPartition(t *testing.T) {
	if core.RAMTotalSize != 0xC000 {
		t.Errorf("RAMTotalSize = 0x%X, want 0xC000", core.RAMTotalSize)
	}
	// Les deux pages vidéo + RAM user ne doivent pas dépasser la RAM totale.
	videoTotal := core.RAMVideoPages * core.RAMVideoSize
	if videoTotal+core.RAMUserSize > core.RAMTotalSize {
		t.Errorf("partition RAM incohérente: 2*video(%d) + user(%d) = %d > total(%d)",
			core.RAMVideoSize, core.RAMUserSize, videoTotal+core.RAMUserSize, core.RAMTotalSize)
	}
	// La RAM utilisateur commence après les deux pages vidéo.
	if core.RAMUserOffset != uint16(core.RAMVideoPages*core.RAMVideoSize) {
		t.Errorf("RAMUserOffset = 0x%X, want 0x%X", core.RAMUserOffset,
			uint16(core.RAMVideoPages*core.RAMVideoSize))
	}
}

func TestPaletteImmutable(t *testing.T) {
	if core.PaletteLen() != 19 {
		t.Errorf("PaletteLen = %d, want 19", core.PaletteLen())
	}
	// Vérifier que deux appels successifs retournent des copies indépendantes.
	a := core.PaletteColor(0)
	a[0] = 99
	b := core.PaletteColor(0)
	if b[0] == 99 {
		t.Error("PaletteColor doit retourner une copie, pas une référence mutable")
	}
}

func TestGammaTableMonotonic(t *testing.T) {
	if core.GammaLen() != 16 {
		t.Errorf("GammaLen = %d, want 16", core.GammaLen())
	}
	for i := 1; i < core.GammaLen(); i++ {
		prev := core.GammaLookup(i - 1)
		curr := core.GammaLookup(i)
		if curr <= prev {
			t.Errorf("GammaLookup(%d)=%d <= GammaLookup(%d)=%d (non monotone)", i, curr, i-1, prev)
		}
	}
}

// TestPenFromFramebuffer vérifie la conversion repère framebuffer → écran actif
// MO5, en particulier les bords de la zone active (cas off-by-one critiques).
func TestPenFromFramebuffer(t *testing.T) {
	cases := []struct {
		name         string
		cx, cy       int
		wantX, wantY int
		inActiveZone bool // dans 0..319 / 0..199 ?
	}{
		{"coin haut-gauche actif", core.BorderWidth, core.BorderWidth, 0, 0, true},
		{"coin bas-droit actif",
			core.BorderWidth + core.ActiveWidth - 1, core.BorderWidth + core.ActiveHeight - 1,
			core.ActiveWidth - 1, core.ActiveHeight - 1, true},
		{"milieu écran", core.BorderWidth + 160, core.BorderWidth + 100, 160, 100, true},
		{"bordure gauche (x=7)", 7, 100, -1, 92, false}, // juste hors zone à gauche
		{"bordure haute (y=7)", 100, 7, 92, -1, false},  // juste hors zone en haut
		{"origine fenêtre (0,0)", 0, 0, -core.BorderWidth, -core.BorderWidth, false},
		{"juste après bord droit",
			core.BorderWidth + core.ActiveWidth, 100, core.ActiveWidth, 92, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			gotX, gotY := core.PenFromFramebuffer(c.cx, c.cy)
			if gotX != c.wantX || gotY != c.wantY {
				t.Errorf("PenFromFramebuffer(%d,%d) = (%d,%d), want (%d,%d)",
					c.cx, c.cy, gotX, gotY, c.wantX, c.wantY)
			}
			active := gotX >= 0 && gotX < core.ActiveWidth && gotY >= 0 && gotY < core.ActiveHeight
			if active != c.inActiveZone {
				t.Errorf("zone active = %v, want %v (coord (%d,%d))", active, c.inActiveZone, gotX, gotY)
			}
		})
	}
}
