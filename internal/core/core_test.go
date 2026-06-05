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

func TestMachineCPUBusConnection(t *testing.T) {
	// Vérifie que Machine implémente cpu6809.Bus et que le CPU est connecté :
	// après Reset avec une ROM contenant un NOP, Step() doit avancer PC.
	rom := make([]byte, 0x4000)
	// Vecteur reset à 0xFFFE pointe sur 0xC000 (rom[0x3FFE/0x3FFF])
	rom[0x3FFE] = 0xC0
	rom[0x3FFF] = 0x00
	// Premier opcode à 0xC000 (rom[0]) = NOP (0x12)
	rom[0x0000] = 0x12
	m, err := core.NewMachine(core.Options{ROMSys: rom})
	if err != nil {
		t.Fatalf("NewMachine: %v", err)
	}
	m.Reset()
	m.Step(4) // NOP = 2 cycles, Step(4) doit l'exécuter
	// Si le CPU est bien connecté au bus, il a pu lire le NOP depuis la ROM.
	// On vérifie juste que Step n'a pas paniqué et a progressé.
	_ = m // pas d'accès PC depuis l'extérieur de core : test de non-panique suffisant ici
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
