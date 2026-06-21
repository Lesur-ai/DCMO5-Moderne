package core_test

// fidelity_test.go — suite de non-régression déterministe.
//
// Toutes les ROM utilisées ici sont GÉNÉRÉES par le code de test.
// Aucune ROM Thomson MO5 copyright n'est utilisée ni embarquée.
//
// Invariant fondamental : même ROM + même nombre de cycles → même état machine.

import (
	"hash/fnv"
	"testing"

	"github.com/Lesur-ai/dcmo5/internal/core"
)

// ── ROM de test générées ──────────────────────────────────────────────────────

// makeNOPROM construit une ROM 16 Ko contenant :
//   - NOP (0x12) répété sur toute la plage 0xC000–0xFFF9
//   - Vecteurs 6809 au sommet (0xFFF0–0xFFFF) pointant tous sur 0xC000
func makeNOPROM() []byte {
	rom := make([]byte, 0x4000)
	for i := range rom {
		rom[i] = 0x12 // NOP
	}
	// Vecteurs (big-endian, offset dans rom = addr - 0xC000)
	setVec := func(addr uint16, target uint16) {
		off := addr - 0xC000
		rom[off] = byte(target >> 8)
		rom[off+1] = byte(target)
	}
	setVec(0xFFFE, 0xC000) // Reset → 0xC000
	setVec(0xFFFC, 0xC000) // NMI
	setVec(0xFFF8, 0xC000) // IRQ
	setVec(0xFFF6, 0xC000) // FIRQ
	setVec(0xFFFA, 0xC000) // SWI
	return rom
}

// makeCounterROM construit une ROM qui incrémente la RAM à 0x4000 en boucle.
//
//	0xC000: LDA $4000 (0xB6 0x40 0x00)   — 5 cycles
//	0xC003: INCA       (0x4C)             — 2 cycles
//	0xC004: STA $4000 (0xB7 0x40 0x00)   — 5 cycles
//	0xC007: BRA $C000 (0x20 0xF7)        — 3 cycles  (offset -9 de 0xC009)
//
// Total par itération : 15 cycles.
func makeCounterROM() []byte {
	rom := make([]byte, 0x4000)
	for i := range rom {
		rom[i] = 0x12
	}
	prog := []byte{
		0xB6, 0x40, 0x00, // LDA $4000
		0x4C,             // INCA
		0xB7, 0x40, 0x00, // STA $4000
		0x20, 0xF7, // BRA -9 → retour à 0xC000
	}
	copy(rom[0:], prog)
	rom[0x3FFE] = 0xC0
	rom[0x3FFF] = 0x00
	return rom
}

// checksumRAM utilise PhysicalRAMChecksum pour couvrir TOUTE la RAM physique,
// y compris la page vidéo inactive (non accessible via le bus CPU courant).
func checksumRAM(m *core.Machine) uint32 {
	return m.PhysicalRAMChecksum()
}

// checksumFramebuffer calcule un hash FNV-32 du framebuffer.
func checksumFramebuffer(m *core.Machine) uint32 {
	h := fnv.New32a()
	fb := m.Framebuffer()
	for _, px := range fb {
		h.Write([]byte{byte(px), byte(px >> 8), byte(px >> 16), byte(px >> 24)})
	}
	return h.Sum32()
}

// ── Tests déterminisme RAM ────────────────────────────────────────────────────

func TestFidelity_RAMChecksum_Deterministic(t *testing.T) {
	// Même ROM + même nombre de cycles → même checksum RAM sur toute plateforme.
	rom := makeNOPROM()

	run := func() uint32 {
		m, err := core.NewMachine(core.Options{ROMSys: rom})
		if err != nil {
			t.Fatalf("NewMachine: %v", err)
		}
		m.Reset()
		m.Step(1000) // 1000 cycles de NOP (500 instructions)
		return checksumRAM(m)
	}

	c1, c2 := run(), run()
	if c1 != c2 {
		t.Errorf("checksum RAM non déterministe: 0x%08X != 0x%08X", c1, c2)
	}
}

func TestFidelity_RAMChecksum_DifferentCycles(t *testing.T) {
	// Un nombre différent de cycles produit un checksum différent (test non trivial).
	rom := makeCounterROM()
	m1, _ := core.NewMachine(core.Options{ROMSys: rom})
	m2, _ := core.NewMachine(core.Options{ROMSys: rom})
	m1.Reset()
	m2.Reset()
	m1.Step(15) // 1 itération
	m2.Step(30) // 2 itérations
	c1, c2 := checksumRAM(m1), checksumRAM(m2)
	if c1 == c2 {
		t.Error("RAM après 15 et 30 cycles devrait différer (compteur incrémenté)")
	}
}

func TestFidelity_CounterROM_Increment(t *testing.T) {
	// Après 15 cycles (1 itération), RAM[0x4000] doit valoir 1.
	// Après 30 cycles (2 itérations), RAM[0x4000] doit valoir 2.
	rom := makeCounterROM()
	m, _ := core.NewMachine(core.Options{ROMSys: rom})
	m.Reset()
	m.Step(15)
	if v := m.Read8(0x4000); v != 1 {
		t.Errorf("après 15 cycles: RAM[0x4000] = %d, want 1", v)
	}
	m.Step(15)
	if v := m.Read8(0x4000); v != 2 {
		t.Errorf("après 30 cycles: RAM[0x4000] = %d, want 2", v)
	}
}

// ── Tests déterminisme framebuffer ────────────────────────────────────────────

func TestFidelity_FramebufferChecksum_Deterministic(t *testing.T) {
	rom := makeNOPROM()
	// Initialiser RAM vidéo avec un motif connu
	m, _ := core.NewMachine(core.Options{ROMSys: rom})
	m.Reset()
	// Remplir ligne 0 avec couleur 1 (rouge)
	for col := 0; col < 40; col++ {
		m.Write8(uint16(col), 0x11) // bg=1 rouge, fg=1 rouge
	}

	fb1 := checksumFramebuffer(m)
	fb2 := checksumFramebuffer(m)
	if fb1 != fb2 {
		t.Errorf("framebuffer non déterministe: 0x%08X != 0x%08X", fb1, fb2)
	}
}

func TestFidelity_FramebufferChecksum_RAMChange(t *testing.T) {
	// Modifier la RAM vidéo doit changer le checksum du framebuffer.
	rom := makeNOPROM()
	m, _ := core.NewMachine(core.Options{ROMSys: rom})
	m.Reset()
	fb0 := checksumFramebuffer(m)
	m.Write8(0x0000, 0xFF) // modifier couleur pixel
	fb1 := checksumFramebuffer(m)
	if fb0 == fb1 {
		t.Error("modification RAM vidéo devrait changer le checksum framebuffer")
	}
}

// ── Tests golden CPU avancés ──────────────────────────────────────────────────

func TestFidelity_NOPBurst_CycleCount(t *testing.T) {
	// NOP = 2 cycles. Après 100 cycles, 50 NOPs exécutés.
	// PC devrait être à 0xC000 + 50 = 0xC032.
	rom := makeNOPROM()
	m, _ := core.NewMachine(core.Options{ROMSys: rom})
	m.Reset()
	consumed := m.Step(100)
	// On accepte que Step consomme légèrement plus (instruction non coupable)
	if consumed < 100 || consumed > 102 {
		t.Errorf("Step(100) a consommé %d cycles, want 100-102", consumed)
	}
}

func TestFidelity_CounterROM_20Iterations(t *testing.T) {
	// 20 itérations × 15 cycles = 300 cycles. RAM[0x4000] = 20.
	rom := makeCounterROM()
	m, _ := core.NewMachine(core.Options{ROMSys: rom})
	m.Reset()
	m.Step(300)
	if v := m.Read8(0x4000); v != 20 {
		t.Errorf("après 300 cycles: RAM[0x4000] = %d, want 20", v)
	}
}

func TestFidelity_Reset_ClearsState(t *testing.T) {
	// Après Step + Reset, l'état est réinitialisé (compteur à zéro).
	rom := makeCounterROM()
	m, _ := core.NewMachine(core.Options{ROMSys: rom})
	m.Reset()
	m.Step(150) // 10 itérations → RAM[0x4000] = 10
	m.Reset()   // reset complet
	// Read8(0x4000) → physical RAM[0x6000] (offset +0x2000).
	// 0x6000 & 0x80 = 0 → pattern init = 0x00.
	if v := m.Read8(0x4000); v != 0x00 {
		t.Errorf("après Reset: RAM[0x4000] = 0x%02X, want 0x00 (pattern init, offset physique 0x6000)", v)
	}
}

func TestFidelity_FrameWidth_Pixels(t *testing.T) {
	// Le framebuffer contient exactement FrameWidth × FrameHeight pixels.
	rom := makeNOPROM()
	m, _ := core.NewMachine(core.Options{ROMSys: rom})
	m.Reset()
	fb := m.Framebuffer()
	want := core.FrameWidth * core.FrameHeight
	if len(fb) != want {
		t.Errorf("framebuffer: %d pixels, want %d", len(fb), want)
	}
}
