package app_test

import (
	"testing"

	"github.com/Lesur-ai/dcmo5/internal/app"
	"github.com/Lesur-ai/dcmo5/internal/spec"
)

func TestLogicalSizeMatchesSpec(t *testing.T) {
	w, h := app.LogicalSize()
	if w != spec.FrameWidth || h != spec.FrameHeight {
		t.Errorf("LogicalSize() = (%d,%d), want (%d,%d)",
			w, h, spec.FrameWidth, spec.FrameHeight)
	}
}

func TestLogicalSizeStable(t *testing.T) {
	w1, h1 := app.LogicalSize()
	w2, h2 := app.LogicalSize()
	if w1 != w2 || h1 != h2 {
		t.Errorf("LogicalSize() instable: (%d,%d) != (%d,%d)", w1, h1, w2, h2)
	}
}

func TestKeyMappingNoDuplicates(t *testing.T) {
	// Vérifie qu'aucun index MO5 n'est mappé deux fois sauf les cas légitimes
	// (ex: SHIFT gauche/droite → même touche MO5, CNT idem).
	seen := map[int]string{}
	// On accepte les doublons légitimes sur ces indices
	legit := map[int]bool{
		0x38: true, // SHIFT L et R → même touche MO5
		0x35: true, // CNT L et R → même touche MO5
		0x2E: true, // + ; et ; alt → même touche MO5
	}
	for eKey, mo5Key := range app.KeyMapping() {
		if legit[mo5Key] {
			continue
		}
		if prev, dup := seen[mo5Key]; dup {
			t.Errorf("doublon MO5 0x%02X : touche Ebitengine %v et %v", mo5Key, prev, eKey)
		}
		seen[mo5Key] = eKey.String()
	}
}

func TestKeyMappingValidRange(t *testing.T) {
	// Chaque index MO5 doit être dans [0, spec.KeyMax).
	for eKey, mo5Key := range app.KeyMapping() {
		if mo5Key < 0 || mo5Key >= spec.KeyMax {
			t.Errorf("touche Ebitengine %v → index MO5 %d hors-bornes [0,%d)", eKey, mo5Key, spec.KeyMax)
		}
	}
}
