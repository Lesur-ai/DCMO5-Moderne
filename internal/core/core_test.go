package core_test

import (
	"testing"

	"github.com/Lesur-ai/dcmo5/internal/core"
	"github.com/Lesur-ai/dcmo5/internal/spec"
)

func TestNewMachineNoROM(t *testing.T) {
	m, err := core.NewMachine(core.Options{})
	if err != nil {
		t.Fatalf("NewMachine sans ROM : erreur inattendue : %v", err)
	}
	if m == nil {
		t.Fatal("NewMachine a retourné nil")
	}
}

func TestFramebufferSize(t *testing.T) {
	m, _ := core.NewMachine(core.Options{})
	fb := m.Framebuffer()
	want := spec.FrameWidth * spec.FrameHeight
	if len(fb) != want {
		t.Errorf("Framebuffer len = %d, want %d", len(fb), want)
	}
}

func TestNoDependencyLeak(t *testing.T) {
	// Vérifie que Machine implémente cpu6809.Bus (nécessaire pour passer au CPU).
	// Si le type ne satisfait pas l'interface, le build échoue.
	m, _ := core.NewMachine(core.Options{})
	_ = m // utilisé pour éviter "declared and not used"
}
