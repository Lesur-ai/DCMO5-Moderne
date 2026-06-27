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

// --- EmulatorLayoutSize : bascule du repère Layout selon l'ouverture de l'overlay ---

func TestEmulatorLayoutSize_OverlayClosed_DisplayGeometry(t *testing.T) {
	// Overlay fermé : Layout renvoie le repère logique d'affichage de la famille,
	// INDÉPENDAMMENT de la taille fenêtre (Ebitengine met à l'échelle).
	if w, h := uimodel.EmulatorLayoutSize(false, machine.FamilyMO, mo5FW, mo5FH, 1280, 800); w != 336 || h != 216 {
		t.Errorf("MO5 overlay fermé = %dx%d, want logique 336x216 (DisplayGeometry)", w, h)
	}
	if w, h := uimodel.EmulatorLayoutSize(false, machine.FamilyTOGateArray, gaFW, gaFH, 1280, 800); w != 672 || h != 432 {
		t.Errorf("gate-array overlay fermé = %dx%d, want logique 672x432 (DisplayGeometry)", w, h)
	}
}

func TestEmulatorLayoutSize_OverlayOpen_WindowFrame(t *testing.T) {
	// Overlay ouvert : Layout passe au repère FENÊTRE réel (1:1), pour un rendu
	// ebitenui au pixel près — quelle que soit la famille.
	for _, fam := range []machine.Family{machine.FamilyMO, machine.FamilyTOGateArray} {
		if w, h := uimodel.EmulatorLayoutSize(true, fam, mo5FW, mo5FH, 800, 600); w != 800 || h != 600 {
			t.Errorf("famille %d overlay ouvert = %dx%d, want repère fenêtre 800x600", fam, w, h)
		}
	}
}

// --- FramebufferAspectFit : rectangle d'aspect-fit centré ---

func TestFramebufferAspectFit(t *testing.T) {
	cases := []struct {
		name                       string
		family                     machine.Family
		fw, fh, outW, outH         int
		wantX, wantY, wantW, wantH int
	}{
		// Surface = aspect d'affichage exact → remplit tout, sans letterbox.
		{"MO5 fit exact", machine.FamilyMO, mo5FW, mo5FH, 672, 432, 0, 0, 672, 432},
		// Gate-array : framebuffer 672×216 mais aspect d'affichage 672×432 → même rect
		// que le MO5 (preuve qu'on utilise DisplayGeometry, pas le ratio brut 672:216).
		{"gate-array fit exact", machine.FamilyTOGateArray, gaFW, gaFH, 672, 432, 0, 0, 672, 432},
		// Fenêtre trop large → barres verticales (letterbox horizontal), centré.
		{"MO5 large → letterbox H", machine.FamilyMO, mo5FW, mo5FH, 1000, 432, 164, 0, 672, 432},
		// Fenêtre trop haute → barres horizontales (letterbox vertical), centré.
		{"MO5 haute → letterbox V", machine.FamilyMO, mo5FW, mo5FH, 672, 600, 0, 84, 672, 432},
		// Surface plus petite : on réduit en préservant l'aspect (672:432 → 336:216).
		{"MO5 réduit", machine.FamilyMO, mo5FW, mo5FH, 336, 216, 0, 0, 336, 216},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			x, y, w, h := uimodel.FramebufferAspectFit(c.family, c.fw, c.fh, c.outW, c.outH)
			if x != c.wantX || y != c.wantY || w != c.wantW || h != c.wantH {
				t.Errorf("got (%d,%d,%d,%d), want (%d,%d,%d,%d)", x, y, w, h, c.wantX, c.wantY, c.wantW, c.wantH)
			}
			// Invariants : le rectangle tient dans la surface et y est centré.
			if w > c.outW || h > c.outH {
				t.Errorf("rect %dx%d déborde la surface %dx%d", w, h, c.outW, c.outH)
			}
			if x < 0 || y < 0 || x*2+w > c.outW+1 || y*2+h > c.outH+1 {
				t.Errorf("rect non centré : x=%d y=%d w=%d h=%d dans %dx%d", x, y, w, h, c.outW, c.outH)
			}
		})
	}
}

// TestFramebufferAspectFit_PreservesDisplayAspect : sur une surface quelconque, le rect
// rendu conserve l'aspect d'affichage de la famille À L'ARRONDI ENTIER PRÈS — borne exacte
// |w·logH − h·logW| < max(logW,logH), car une seule des deux dimensions subit la troncature
// d'une division entière. C'est ce qui garantit qu'aucun écrasement n'est réintroduit quand
// l'overlay est ouvert (contrairement au bug #147 où le TO8D était aplati).
func TestFramebufferAspectFit_PreservesDisplayAspect(t *testing.T) {
	for _, fam := range []machine.Family{machine.FamilyMO, machine.FamilyTOGateArray} {
		logW, logH, _, _ := uimodel.DisplayGeometry(fam, gaFW, gaFH) // gaFW/FH = framebuffer max
		_, _, w, h := uimodel.FramebufferAspectFit(fam, gaFW, gaFH, 1337, 911)
		skew := w*logH - h*logW // == 0 en aspect exact ; |skew| borné par l'arrondi
		if skew < 0 {
			skew = -skew
		}
		bound := logW
		if logH > bound {
			bound = logH
		}
		if skew >= bound {
			t.Errorf("famille %d : rect %dx%d s'écarte de l'aspect d'affichage %dx%d de %d (≥ %d)",
				fam, w, h, logW, logH, skew, bound)
		}
	}
}

// TestFramebufferAspectFit_Degenerate : surface ou géométrie nulle → rect nul (rien à
// dessiner), pas de division par zéro.
func TestFramebufferAspectFit_Degenerate(t *testing.T) {
	for _, c := range []struct{ outW, outH int }{{0, 432}, {672, 0}, {0, 0}} {
		if x, y, w, h := uimodel.FramebufferAspectFit(machine.FamilyMO, mo5FW, mo5FH, c.outW, c.outH); x|y|w|h != 0 {
			t.Errorf("surface %dx%d : rect = (%d,%d,%d,%d), want (0,0,0,0)", c.outW, c.outH, x, y, w, h)
		}
	}
}
