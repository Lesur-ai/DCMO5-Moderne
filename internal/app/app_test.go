package app_test

import (
	"testing"

	"github.com/Lesur-ai/dcmo5/internal/app"
	"github.com/Lesur-ai/dcmo5/internal/spec"
)

func TestLayoutReturnsMO5Dimensions(t *testing.T) {
	a := app.New()
	w, h := a.Layout(1920, 1080)
	if w != spec.FrameWidth || h != spec.FrameHeight {
		t.Errorf("Layout(1920,1080) = (%d,%d), want (%d,%d)",
			w, h, spec.FrameWidth, spec.FrameHeight)
	}
}

func TestLayoutIndependentOfWindowSize(t *testing.T) {
	a := app.New()
	w1, h1 := a.Layout(640, 480)
	w2, h2 := a.Layout(1280, 960)
	if w1 != w2 || h1 != h2 {
		t.Errorf("Layout doit retourner des dimensions fixes quelle que soit la fenêtre : "+
			"(%d,%d) != (%d,%d)", w1, h1, w2, h2)
	}
}
