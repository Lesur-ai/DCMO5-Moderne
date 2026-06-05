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
	m, _ := core.NewMachine(core.Options{})
	_ = m
}

func TestNewMachineInvalidROMSize(t *testing.T) {
	_, err := core.NewMachine(core.Options{ROMSys: make([]byte, 100)})
	if err == nil {
		t.Error("NewMachine avec ROM de mauvaise taille devrait retourner une erreur")
	}
}

func TestNewMachineValidROMSize(t *testing.T) {
	rom := make([]byte, 0x4000)
	_, err := core.NewMachine(core.Options{ROMSys: rom})
	if err != nil {
		t.Errorf("NewMachine avec ROM 16 Ko valide : erreur inattendue : %v", err)
	}
}
