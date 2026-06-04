package spec_test

import "testing"
import "github.com/Lesur-ai/dcmo5/internal/spec"

func TestFramebufferDimensions(t *testing.T) {
	if spec.FrameWidth != 336 {
		t.Errorf("FrameWidth = %d, want 336", spec.FrameWidth)
	}
	if spec.FrameHeight != 216 {
		t.Errorf("FrameHeight = %d, want 216", spec.FrameHeight)
	}
}

func TestRAMSizes(t *testing.T) {
	if spec.RAMTotalSize != 0xC000 {
		t.Errorf("RAMTotalSize = 0x%X, want 0xC000", spec.RAMTotalSize)
	}
	if spec.RAMVideoSize != 0x2000 {
		t.Errorf("RAMVideoSize = 0x%X, want 0x2000", spec.RAMVideoSize)
	}
	if spec.RAMVideoSize*2 > spec.RAMTotalSize {
		t.Error("deux pages vidéo dépassent la RAM totale")
	}
}

func TestDiskSize(t *testing.T) {
	want := 327680
	if spec.FDDiskSize != want {
		t.Errorf("FDDiskSize = %d, want %d", spec.FDDiskSize, want)
	}
}

func TestVectors(t *testing.T) {
	if spec.VectorReset != 0xFFFE {
		t.Errorf("VectorReset = 0x%X, want 0xFFFE", spec.VectorReset)
	}
	if spec.VectorNMI != 0xFFFC {
		t.Errorf("VectorNMI = 0x%X, want 0xFFFC", spec.VectorNMI)
	}
	if spec.VectorIRQ != 0xFFF8 {
		t.Errorf("VectorIRQ = 0x%X, want 0xFFF8", spec.VectorIRQ)
	}
}

func TestPaletteSize(t *testing.T) {
	if len(spec.Palette) != 19 {
		t.Errorf("Palette len = %d, want 19", len(spec.Palette))
	}
}

func TestGammaTableMonotonic(t *testing.T) {
	g := spec.GammaTable
	for i := 1; i < len(g); i++ {
		if g[i] <= g[i-1] {
			t.Errorf("GammaTable[%d]=%d <= GammaTable[%d]=%d (non monotone)", i, g[i], i-1, g[i-1])
		}
	}
}

func TestKeyMax(t *testing.T) {
	if spec.KeyMax != 58 {
		t.Errorf("KeyMax = %d, want 58", spec.KeyMax)
	}
}
