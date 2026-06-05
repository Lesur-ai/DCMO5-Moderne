package app_test

import (
	"strings"
	"testing"

	"github.com/Lesur-ai/dcmo5/internal/app"
	"github.com/Lesur-ai/dcmo5/internal/spec"
)

func TestLogicalSizeMatchesSpec(t *testing.T) {
	w, h := app.LogicalSize()
	if w != spec.FrameWidth || h != spec.FrameHeight {
		t.Errorf("LogicalSize() = (%d,%d), want (%d,%d)", w, h, spec.FrameWidth, spec.FrameHeight)
	}
}

func TestLogicalSizeStable(t *testing.T) {
	w1, h1 := app.LogicalSize()
	w2, h2 := app.LogicalSize()
	if w1 != w2 || h1 != h2 {
		t.Errorf("LogicalSize() instable")
	}
}

func TestKeyMappingNoDuplicates(t *testing.T) {
	seen := map[int]string{}
	// Doublons légitimes : SHIFT (0x38), CNT (0x35), ACC (0x36) ont chacun une
	// variante gauche et droite sur le même index MO5.
	legit := map[int]bool{0x38: true, 0x35: true, 0x36: true}
	for eKey, mo5Key := range app.KeyMapping() {
		if legit[mo5Key] {
			continue
		}
		if prev, dup := seen[mo5Key]; dup {
			t.Errorf("doublon MO5 0x%02X: %v et %v", mo5Key, prev, eKey)
		}
		seen[mo5Key] = eKey.String()
	}
}

func TestKeyMappingValidRange(t *testing.T) {
	for eKey, mo5Key := range app.KeyMapping() {
		if mo5Key < 0 || mo5Key >= spec.KeyMax {
			t.Errorf("touche %v → index MO5 %d hors-bornes [0,%d)", eKey, mo5Key, spec.KeyMax)
		}
	}
}

// ── TitleForState ─────────────────────────────────────────────────────────────

func TestTitle_Normal(t *testing.T) {
	got := app.TitleForState(false, false, "mo5.rom", "", "")
	if !strings.Contains(got, "mo5.rom") {
		t.Errorf("titre normal: %q ne contient pas 'mo5.rom'", got)
	}
	if strings.Contains(got, "PAUSE") || strings.Contains(got, "manquante") {
		t.Errorf("titre normal ne doit pas contenir PAUSE/manquante: %q", got)
	}
}

func TestTitle_ROMmissing(t *testing.T) {
	got := app.TitleForState(true, false, "", "", "")
	if !strings.Contains(got, "manquante") {
		t.Errorf("ROM manquante: %q ne contient pas 'manquante'", got)
	}
}

func TestTitle_Paused(t *testing.T) {
	got := app.TitleForState(false, true, "mo5.rom", "", "")
	if !strings.Contains(got, "[PAUSE]") {
		t.Errorf("pause: %q ne contient pas '[PAUSE]'", got)
	}
}

func TestTitle_WithTape(t *testing.T) {
	got := app.TitleForState(false, false, "mo5.rom", "jeu.k7", "")
	if !strings.Contains(got, "jeu.k7") {
		t.Errorf("avec tape: %q ne contient pas 'jeu.k7'", got)
	}
}

func TestTitle_PausedROMmissing(t *testing.T) {
	got := app.TitleForState(true, true, "", "", "")
	if !strings.Contains(got, "manquante") || !strings.Contains(got, "[PAUSE]") {
		t.Errorf("pause+ROM manquante: %q", got)
	}
}
