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

func TestRAMPartition(t *testing.T) {
	if spec.RAMTotalSize != 0xC000 {
		t.Errorf("RAMTotalSize = 0x%X, want 0xC000", spec.RAMTotalSize)
	}
	// Les deux pages vidéo + RAM user ne doivent pas dépasser la RAM totale.
	videoTotal := spec.RAMVideoPages * spec.RAMVideoSize
	if videoTotal+spec.RAMUserSize > spec.RAMTotalSize {
		t.Errorf("partition RAM incohérente: 2*video(%d) + user(%d) = %d > total(%d)",
			spec.RAMVideoSize, spec.RAMUserSize, videoTotal+spec.RAMUserSize, spec.RAMTotalSize)
	}
	// La RAM utilisateur commence après les deux pages vidéo.
	if spec.RAMUserOffset != uint16(spec.RAMVideoPages*spec.RAMVideoSize) {
		t.Errorf("RAMUserOffset = 0x%X, want 0x%X", spec.RAMUserOffset,
			uint16(spec.RAMVideoPages*spec.RAMVideoSize))
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

func TestPaletteImmutable(t *testing.T) {
	if spec.PaletteLen() != 19 {
		t.Errorf("PaletteLen = %d, want 19", spec.PaletteLen())
	}
	// Vérifier que deux appels successifs retournent des copies indépendantes.
	a := spec.PaletteColor(0)
	a[0] = 99
	b := spec.PaletteColor(0)
	if b[0] == 99 {
		t.Error("PaletteColor doit retourner une copie, pas une référence mutable")
	}
}

func TestGammaTableMonotonic(t *testing.T) {
	if spec.GammaLen() != 16 {
		t.Errorf("GammaLen = %d, want 16", spec.GammaLen())
	}
	for i := 1; i < spec.GammaLen(); i++ {
		prev := spec.GammaLookup(i - 1)
		curr := spec.GammaLookup(i)
		if curr <= prev {
			t.Errorf("GammaLookup(%d)=%d <= GammaLookup(%d)=%d (non monotone)", i, curr, i-1, prev)
		}
	}
}

func TestKeyMax(t *testing.T) {
	if spec.KeyMax != 58 {
		t.Errorf("KeyMax = %d, want 58", spec.KeyMax)
	}
}
