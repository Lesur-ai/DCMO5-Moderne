package app_test

import (
	"testing"

	"github.com/Lesur-ai/dcmo5/internal/app"
	"github.com/Lesur-ai/dcmo5/internal/spec"
)

// Les tests de internal/app doivent éviter d'instancier App ou tout type
// Ebitengine : leur construction initialise GLFW/Metal et plante en CI headless.
// On teste uniquement les fonctions pures qui n'initialisent pas Ebitengine.

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
		t.Errorf("LogicalSize() instable entre deux appels : (%d,%d) != (%d,%d)", w1, h1, w2, h2)
	}
}
