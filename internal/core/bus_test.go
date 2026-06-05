package core_test

import (
	"testing"

	"github.com/Lesur-ai/dcmo5/internal/core"
)

func newMachine(t *testing.T) *core.Machine {
	t.Helper()
	m, err := core.NewMachine(core.Options{})
	if err != nil {
		t.Fatalf("NewMachine: %v", err)
	}
	return m
}

// ── RAM vidéo ─────────────────────────────────────────────────────────────────

func TestBus_VideoRAM_colors(t *testing.T) {
	// Adresses 0x0000–0x1FFF : RAM vidéo couleurs (page 0 par défaut).
	m := newMachine(t)
	m.Write8(0x0000, 0xAB)
	if v := m.Read8(0x0000); v != 0xAB {
		t.Errorf("RAM vidéo 0x0000: got 0x%02X, want 0xAB", v)
	}
	m.Write8(0x1FFF, 0xCD)
	if v := m.Read8(0x1FFF); v != 0xCD {
		t.Errorf("RAM vidéo 0x1FFF: got 0x%02X, want 0xCD", v)
	}
}

func TestBus_VideoRAM_forms(t *testing.T) {
	// Adresses 0x2000–0x3FFF : RAM vidéo formes.
	m := newMachine(t)
	m.Write8(0x2000, 0x55)
	if v := m.Read8(0x2000); v != 0x55 {
		t.Errorf("RAM vidéo formes 0x2000: got 0x%02X, want 0x55", v)
	}
}

// ── RAM utilisateur ───────────────────────────────────────────────────────────

func TestBus_UserRAM(t *testing.T) {
	m := newMachine(t)
	m.Write8(0x4000, 0x11)
	m.Write8(0x9FFF, 0x22)
	if v := m.Read8(0x4000); v != 0x11 {
		t.Errorf("RAM user 0x4000: got 0x%02X, want 0x11", v)
	}
	if v := m.Read8(0x9FFF); v != 0x22 {
		t.Errorf("RAM user 0x9FFF: got 0x%02X, want 0x22", v)
	}
}

// ── ROM système read-only ─────────────────────────────────────────────────────

func TestBus_ROMsys_readonly(t *testing.T) {
	rom := make([]byte, 0x4000)
	rom[0] = 0xAA      // offset 0 → adresse 0xC000
	rom[0x3FFF] = 0xBB // offset 0x3FFF → adresse 0xFFFF
	m, _ := core.NewMachine(core.Options{ROMSys: rom})

	if v := m.Read8(0xC000); v != 0xAA {
		t.Errorf("ROM sys 0xC000: got 0x%02X, want 0xAA", v)
	}
	if v := m.Read8(0xFFFF); v != 0xBB {
		t.Errorf("ROM sys 0xFFFF: got 0x%02X, want 0xBB", v)
	}
	// Écriture ignorée
	m.Write8(0xC000, 0x00)
	if v := m.Read8(0xC000); v != 0xAA {
		t.Errorf("ROM sys après écriture: got 0x%02X (doit rester 0xAA)", v)
	}
}

// ── Port 0xA7C0 ───────────────────────────────────────────────────────────────

func TestBus_Port_A7C0_penbutton(t *testing.T) {
	m := newMachine(t)
	m.SetPen(10, 20, false)
	v := m.Read8(0xA7C0)
	if v&0x80 == 0 {
		t.Errorf("port 0xA7C0 bit7 devrait être 1 toujours, got 0x%02X", v)
	}
	if v&0x20 != 0 {
		t.Errorf("port 0xA7C0 penbutton=false : bit5 devrait être 0, got 0x%02X", v)
	}
	m.SetPen(10, 20, true)
	v = m.Read8(0xA7C0)
	if v&0x20 == 0 {
		t.Errorf("port 0xA7C0 penbutton=true : bit5 devrait être 1, got 0x%02X", v)
	}
}

// ── Clavier via port 0xA7C1 ───────────────────────────────────────────────────

func TestBus_Keyboard_pressedKey(t *testing.T) {
	m := newMachine(t)
	// Port[1] encode la colonne dans les bits 7:1.
	// On écrit dans port[1] pour sélectionner la colonne 5 (=bits 7:1 → 0x0A).
	m.Write8(0xA7C1, 0x0A) // colonne = 5 (0x0A >> 1 = 5)
	// Touche 5 relâchée par défaut → bit 0 = 0x80
	v := m.Read8(0xA7C1)
	if v&0x80 == 0 {
		t.Errorf("touche 5 relâchée : bit0 devrait être 0x80, got 0x%02X", v)
	}
	// Appuyer sur la touche 5
	m.SetKey(core.Key(5), true)
	v = m.Read8(0xA7C1)
	if v&0x80 != 0 {
		t.Errorf("touche 5 pressée : bit0 devrait être 0, got 0x%02X", v)
	}
}

// ── Joystick via port 0xA7CC/0xA7CD ──────────────────────────────────────────

func TestBus_Joystick_read(t *testing.T) {
	m := newMachine(t)
	// Activer le mode joystick via port[0x0E] bit2
	m.Write8(0xA7CE, 0x04) // port[0x0E] = 4 → mode joystick actif
	m.SetJoystick(core.JoystickInput{Position: 0xF0, Action: 0xC0})
	if v := m.Read8(0xA7CC); v != 0xF0 {
		t.Errorf("joystick position: got 0x%02X, want 0xF0", v)
	}
	m.Write8(0xA7CF, 0x04) // port[0x0F] = 4 → mode action actif
	if v := m.Read8(0xA7CD); v != 0xC0 {
		t.Errorf("joystick action: got 0x%02X, want 0xC0", v)
	}
}

// ── RAM vidéo page switch ─────────────────────────────────────────────────────

func TestBus_VideoRAM_pageSwitch(t *testing.T) {
	m := newMachine(t)
	// Écrire valeur distincte sur chaque page à l'adresse 0x0100
	m.Write8(0xA7C0, 0x00) // port[0] bit0=0 → page 0
	m.Write8(0x0100, 0xAA)
	m.Write8(0xA7C0, 0x01) // port[0] bit0=1 → page 1
	m.Write8(0x0100, 0xBB)
	// Relire page 0
	m.Write8(0xA7C0, 0x00)
	if v := m.Read8(0x0100); v != 0xAA {
		t.Errorf("page 0 après switch: got 0x%02X, want 0xAA", v)
	}
	// Relire page 1
	m.Write8(0xA7C0, 0x01)
	if v := m.Read8(0x0100); v != 0xBB {
		t.Errorf("page 1 après switch: got 0x%02X, want 0xBB", v)
	}
}

// ── Reset initialise le pattern RAM ──────────────────────────────────────────

func TestMachine_ResetRAMPattern(t *testing.T) {
	m := newMachine(t)
	// Après reset : ram[0x0000] = 0x00 (index&0x80 == 0)
	if v := m.Read8(0x0000); v != 0x00 {
		t.Errorf("RAM[0]: got 0x%02X, want 0x00", v)
	}
	// ram[0x0080] = 0xFF (index&0x80 != 0)
	if v := m.Read8(0x0080); v != 0xFF {
		t.Errorf("RAM[0x80]: got 0x%02X, want 0xFF", v)
	}
}
