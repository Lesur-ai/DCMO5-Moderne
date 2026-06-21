package core_test

import (
	"testing"

	"github.com/Lesur-ai/dcmo5/internal/core"
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
	want := core.FrameWidth * core.FrameHeight
	if len(fb) != want {
		t.Errorf("Framebuffer len = %d, want %d", len(fb), want)
	}
}

func TestMachineCPUBusConnection(t *testing.T) {
	// Vérifie que le CPU est bien connecté au bus machine : un programme ROM
	// stocke une valeur en RAM, lisible via m.Read8.
	//
	// Programme ROM à 0xC000 :
	//   LDA #0x42   (0x86 0x42)  — charge une valeur connue
	//   STA $4000   (0xB7 0x40 0x00) — écrit en RAM user (CPU 0x4000)
	//   NOP         (0x12)
	rom := make([]byte, 0x4000)
	rom[0x3FFE] = 0xC0 // vecteur reset hi
	rom[0x3FFF] = 0x00 // vecteur reset lo → PC=0xC000
	rom[0x0000] = 0x86 // LDA imm
	rom[0x0001] = 0x42
	rom[0x0002] = 0xB7 // STA ext
	rom[0x0003] = 0x40
	rom[0x0004] = 0x00
	rom[0x0005] = 0x12 // NOP
	m, err := core.NewMachine(core.Options{ROMSys: rom})
	if err != nil {
		t.Fatalf("NewMachine: %v", err)
	}
	m.Reset()
	m.Step(20) // 3 instructions : LDA(2) + STA(5) + NOP(2) = 9 cycles
	// La RAM user CPU 0x4000 doit contenir 0x42 si CPU et bus sont connectés.
	if v := m.Read8(0x4000); v != 0x42 {
		t.Errorf("CPU/bus connection: RAM[0x4000] = 0x%02X, want 0x42", v)
	}
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
