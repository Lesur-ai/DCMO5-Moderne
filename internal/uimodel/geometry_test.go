package uimodel_test

import (
	"testing"

	"github.com/Lesur-ai/dcmo5/internal/machine"
	"github.com/Lesur-ai/dcmo5/internal/uimodel"
)

// Dimensions réelles des framebuffers (cf. core.FrameWidth/Height = 336×216 et
// gatearray xbitmap/ybitmap = 672×216). On les fige ici en littéraux pour que le
// test échoue si la géométrie d'une famille dérive.
const (
	mo5FW, mo5FH = 336, 216
	gaFW, gaFH   = 672, 216
)

func TestDisplayGeometry_MO5_Unchanged(t *testing.T) {
	logW, logH, winW, winH := uimodel.DisplayGeometry(machine.FamilyMO, mo5FW, mo5FH)
	// MO5 strictement inchangé : logique = framebuffer, fenêtre = ×2.
	if logW != 336 || logH != 216 {
		t.Errorf("logique MO5 = %dx%d, want 336x216", logW, logH)
	}
	if winW != 672 || winH != 432 {
		t.Errorf("fenêtre MO5 = %dx%d, want 672x432", winW, winH)
	}
}

func TestDisplayGeometry_GateArray_StretchedHeight(t *testing.T) {
	logW, logH, winW, winH := uimodel.DisplayGeometry(machine.FamilyTOGateArray, gaFW, gaFH)
	// Gate-array : hauteur étirée ×2 au niveau logique, fenêtre 1:1 avec le logique.
	if logW != 672 || logH != 432 {
		t.Errorf("logique gate-array = %dx%d, want 672x432", logW, logH)
	}
	if winW != 672 || winH != 432 {
		t.Errorf("fenêtre gate-array = %dx%d, want 672x432", winW, winH)
	}
}

// L'objectif fonctionnel de #147 : les deux familles doivent présenter le MÊME
// ratio d'aspect fenêtre (≈ 1.555), preuve que l'écrasement TO8D est corrigé.
func TestDisplayGeometry_AspectRatioAlignedAcrossFamilies(t *testing.T) {
	_, _, mw, mh := uimodel.DisplayGeometry(machine.FamilyMO, mo5FW, mo5FH)
	_, _, gw, gh := uimodel.DisplayGeometry(machine.FamilyTOGateArray, gaFW, gaFH)
	// Comparaison en entiers (produit en croix) pour éviter les flottants :
	// mw/mh == gw/gh  ⇔  mw*gh == gw*mh.
	if mw*gh != gw*mh {
		t.Errorf("aspects fenêtre différents : MO %d/%d vs gate-array %d/%d", mw, mh, gw, gh)
	}
}

func TestDisplayGeometry_TO7_ProvisionalMOLike(t *testing.T) {
	logW, logH, winW, winH := uimodel.DisplayGeometry(machine.FamilyTO7, mo5FW, mo5FH)
	if logW != mo5FW || logH != mo5FH || winW != 2*mo5FW || winH != 2*mo5FH {
		t.Errorf("TO7 provisoire = log %dx%d win %dx%d, want MO-like log %dx%d win %dx%d",
			logW, logH, winW, winH, mo5FW, mo5FH, 2*mo5FW, 2*mo5FH)
	}
}

func TestDisplayGeometry_UnknownFamilyPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("DisplayGeometry aurait dû paniquer sur une famille hors énumération")
		}
	}()
	// Valeur volontairement hors de l'énumération Family.
	uimodel.DisplayGeometry(machine.Family(999), mo5FW, mo5FH)
}

func TestCursorToFramebuffer_MO5_Identity(t *testing.T) {
	// MO5 : repère logique == framebuffer → identité, y compris aux bords.
	cases := [][2]int{{0, 0}, {10, 20}, {335, 215}}
	for _, c := range cases {
		fbX, fbY := uimodel.CursorToFramebuffer(machine.FamilyMO, mo5FW, mo5FH, c[0], c[1])
		if fbX != c[0] || fbY != c[1] {
			t.Errorf("MO5 (%d,%d) → (%d,%d), want identité", c[0], c[1], fbX, fbY)
		}
	}
}

func TestCursorToFramebuffer_GateArray_HalvesY(t *testing.T) {
	// Gate-array : X inchangé, Y ramené à l'échelle framebuffer (y/2).
	cases := []struct{ x, y, wantX, wantY int }{
		{0, 0, 0, 0},
		{100, 200, 100, 100}, // y/2
		{671, 431, 671, 215}, // bord bas-droit : Y plafonne dans le framebuffer (215)
		{300, 1, 300, 0},     // arrondi vers le bas
	}
	for _, c := range cases {
		fbX, fbY := uimodel.CursorToFramebuffer(machine.FamilyTOGateArray, gaFW, gaFH, c.x, c.y)
		if fbX != c.wantX || fbY != c.wantY {
			t.Errorf("gate-array (%d,%d) → (%d,%d), want (%d,%d)", c.x, c.y, fbX, fbY, c.wantX, c.wantY)
		}
	}
}
