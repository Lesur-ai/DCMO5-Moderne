package spec_test

import (
	"testing"

	"github.com/Lesur-ai/dcmo5/internal/spec"
)

// TestPenFromFramebuffer vérifie la conversion repère framebuffer → écran actif
// MO5, en particulier les bords de la zone active (cas off-by-one critiques).
func TestPenFromFramebuffer(t *testing.T) {
	cases := []struct {
		name         string
		cx, cy       int
		wantX, wantY int
		inActiveZone bool // dans 0..319 / 0..199 ?
	}{
		{"coin haut-gauche actif", spec.BorderWidth, spec.BorderWidth, 0, 0, true},
		{"coin bas-droit actif",
			spec.BorderWidth + spec.ActiveWidth - 1, spec.BorderWidth + spec.ActiveHeight - 1,
			spec.ActiveWidth - 1, spec.ActiveHeight - 1, true},
		{"milieu écran", spec.BorderWidth + 160, spec.BorderWidth + 100, 160, 100, true},
		{"bordure gauche (x=7)", 7, 100, -1, 92, false}, // juste hors zone à gauche
		{"bordure haute (y=7)", 100, 7, 92, -1, false},  // juste hors zone en haut
		{"origine fenêtre (0,0)", 0, 0, -spec.BorderWidth, -spec.BorderWidth, false},
		{"juste après bord droit",
			spec.BorderWidth + spec.ActiveWidth, 100, spec.ActiveWidth, 92, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			gotX, gotY := spec.PenFromFramebuffer(c.cx, c.cy)
			if gotX != c.wantX || gotY != c.wantY {
				t.Errorf("PenFromFramebuffer(%d,%d) = (%d,%d), want (%d,%d)",
					c.cx, c.cy, gotX, gotY, c.wantX, c.wantY)
			}
			active := gotX >= 0 && gotX < spec.ActiveWidth && gotY >= 0 && gotY < spec.ActiveHeight
			if active != c.inActiveZone {
				t.Errorf("zone active = %v, want %v (coord (%d,%d))", active, c.inActiveZone, gotX, gotY)
			}
		})
	}
}
