package spec_test

import (
	"testing"

	"github.com/Lesur-ai/dcmo5/internal/spec"
)

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

func TestKeyMax(t *testing.T) {
	if spec.KeyMax != 58 {
		t.Errorf("KeyMax = %d, want 58", spec.KeyMax)
	}
}
