package cpu6809_test

import (
	"testing"

	"github.com/Lesur-ai/dcmo5/internal/cpu6809"
)

// ── stubBus ──────────────────────────────────────────────────────────────────

type stubBus struct {
	mem [0x10000]uint8
}

func (b *stubBus) Read8(addr uint16) uint8     { return b.mem[addr] }
func (b *stubBus) Write8(addr uint16, v uint8) { b.mem[addr] = v }
func (b *stubBus) set16(addr uint16, v uint16) {
	b.mem[addr] = uint8(v >> 8)
	b.mem[addr+1] = uint8(v)
}

var _ cpu6809.Bus = (*stubBus)(nil)

// ── Reset ────────────────────────────────────────────────────────────────────

func TestResetLoadsVector(t *testing.T) {
	bus := &stubBus{}
	bus.set16(0xFFFE, 0x1234)
	cpu := cpu6809.New(bus)
	cpu.Reset()
	s := cpu.Snapshot()
	if s.PC != 0x1234 {
		t.Errorf("PC = 0x%04X, want 0x1234", s.PC)
	}
}

func TestResetCC(t *testing.T) {
	bus := &stubBus{}
	cpu := cpu6809.New(bus)
	cpu.Reset()
	s := cpu.Snapshot()
	if s.CC&cpu6809.FlagI == 0 {
		t.Errorf("FlagI non positionné après Reset : CC=0x%02X", s.CC)
	}
	if s.CC&cpu6809.FlagF == 0 {
		t.Errorf("FlagF non positionné après Reset : CC=0x%02X", s.CC)
	}
	if s.CC != cpu6809.ResetCC {
		t.Errorf("CC après Reset = 0x%02X, want 0x%02X", s.CC, cpu6809.ResetCC)
	}
}

func TestSnapshotStable(t *testing.T) {
	bus := &stubBus{}
	cpu := cpu6809.New(bus)
	cpu.Reset()
	s1, s2 := cpu.Snapshot(), cpu.Snapshot()
	if s1 != s2 {
		t.Error("deux Snapshots successifs sans Step doivent être identiques")
	}
}

// ── Registre D = A<<8|B ──────────────────────────────────────────────────────

func TestRegisterD(t *testing.T) {
	// On valide D via Snapshot après un reset (A=B=0 → D=0)
	bus := &stubBus{}
	cpu := cpu6809.New(bus)
	cpu.Reset()
	s := cpu.Snapshot()
	d := uint16(s.A)<<8 | uint16(s.B)
	if d != 0 {
		t.Errorf("D = 0x%04X, want 0x0000 après Reset", d)
	}
}

// ── Flags ────────────────────────────────────────────────────────────────────

func TestFlagConstants(t *testing.T) {
	tests := []struct {
		name string
		flag uint8
		bit  int
	}{
		{"C", cpu6809.FlagC, 0},
		{"V", cpu6809.FlagV, 1},
		{"Z", cpu6809.FlagZ, 2},
		{"N", cpu6809.FlagN, 3},
		{"I", cpu6809.FlagI, 4},
		{"H", cpu6809.FlagH, 5},
		{"F", cpu6809.FlagF, 6},
		{"E", cpu6809.FlagE, 7},
	}
	for _, tt := range tests {
		if tt.flag != 1<<tt.bit {
			t.Errorf("Flag%s = 0x%02X, want 0x%02X", tt.name, tt.flag, 1<<tt.bit)
		}
	}
}

func TestFlagsNoOverlap(t *testing.T) {
	flags := []uint8{cpu6809.FlagC, cpu6809.FlagV, cpu6809.FlagZ, cpu6809.FlagN,
		cpu6809.FlagI, cpu6809.FlagH, cpu6809.FlagF, cpu6809.FlagE}
	seen := uint8(0)
	for _, f := range flags {
		if seen&f != 0 {
			t.Errorf("flag 0x%02X chevauche un flag précédent", f)
		}
		seen |= f
	}
}

// ── Modes d'adressage ────────────────────────────────────────────────────────

// newCPUAt crée un CPU dont le PC pointe sur addr, avec le bus donné.
func newCPUAt(bus *stubBus, pc uint16) *cpu6809.CPU {
	bus.set16(0xFFFE, pc)
	cpu := cpu6809.New(bus)
	cpu.Reset()
	return cpu
}

func TestAddrImmediate(t *testing.T) {
	bus := &stubBus{}
	bus.mem[0x1000] = 0x42
	cpu := newCPUAt(bus, 0x1000)
	r := cpu.AddrImmediate(1)
	if r.Addr != 0x1000 {
		t.Errorf("AddrImmediate addr = 0x%04X, want 0x1000", r.Addr)
	}
	// PC avancé de 1
	s := cpu.Snapshot()
	if s.PC != 0x1001 {
		t.Errorf("PC après AddrImmediate(1) = 0x%04X, want 0x1001", s.PC)
	}
}

func TestAddrDirect(t *testing.T) {
	bus := &stubBus{}
	// DP = 0x20, offset = 0x30 → effective = 0x2030
	// On ne peut pas écrire DP directement, on passe par Reset + manipulation
	// via le stub : on simule DP=0x20 en préchargeant via un test indirect.
	// Pour ce test, on vérifie juste la formule DP:byte.
	// On place le CPU avec DP=0 (reset), offset=0x55 → addr=0x0055.
	bus.mem[0x1000] = 0x55
	cpu := newCPUAt(bus, 0x1000)
	r := cpu.AddrDirect()
	if r.Addr != 0x0055 {
		t.Errorf("AddrDirect addr = 0x%04X, want 0x0055", r.Addr)
	}
}

func TestAddrExtended(t *testing.T) {
	bus := &stubBus{}
	bus.set16(0x1000, 0xABCD)
	cpu := newCPUAt(bus, 0x1000)
	r := cpu.AddrExtended()
	if r.Addr != 0xABCD {
		t.Errorf("AddrExtended addr = 0x%04X, want 0xABCD", r.Addr)
	}
	s := cpu.Snapshot()
	if s.PC != 0x1002 {
		t.Errorf("PC après AddrExtended = 0x%04X, want 0x1002", s.PC)
	}
}

func TestAddrIndexed5BitOffset(t *testing.T) {
	// Post-byte 0x00 : X + 0 (5-bit offset = 0, bit7=0)
	bus := &stubBus{}
	bus.mem[0x1000] = 0x00 // post-byte : X+0
	cpu := newCPUAt(bus, 0x1000)
	// X=0 après reset, addr = X+0 = 0
	r := cpu.AddrIndexed()
	if r.Addr != 0x0000 {
		t.Errorf("AddrIndexed 5-bit +0 = 0x%04X, want 0x0000", r.Addr)
	}
}

func TestAddrIndexedRPostInc(t *testing.T) {
	// Post-byte 0x81 : ,R++ (X post-increment par 2, Extra=3)
	bus := &stubBus{}
	bus.mem[0x1000] = 0x81
	cpu := newCPUAt(bus, 0x1000)
	// X=0 après reset
	r := cpu.AddrIndexed()
	if r.Addr != 0x0000 {
		t.Errorf("AddrIndexed ,X++ addr = 0x%04X, want 0x0000", r.Addr)
	}
	if r.Extra != 3 {
		t.Errorf("AddrIndexed ,X++ Extra = %d, want 3", r.Extra)
	}
	// X doit avoir avancé de 2
	s := cpu.Snapshot()
	if s.X != 0x0002 {
		t.Errorf("X après ,X++ = 0x%04X, want 0x0002", s.X)
	}
}

func TestAddrIndexedPreDec(t *testing.T) {
	// Post-byte 0x83 : ,--R (pré-décrémente X de 2, Extra=3)
	bus := &stubBus{}
	bus.mem[0x1000] = 0x83
	cpu := newCPUAt(bus, 0x1000)
	// X=0 → après --X (2) = 0xFFFE
	r := cpu.AddrIndexed()
	if r.Addr != 0xFFFE {
		t.Errorf("AddrIndexed ,--X addr = 0x%04X, want 0xFFFE", r.Addr)
	}
	if r.Extra != 3 {
		t.Errorf("AddrIndexed ,--X Extra = %d, want 3", r.Extra)
	}
}

func TestAddrIndexedExtendedIndirect(t *testing.T) {
	// Post-byte 0x9F : [word] — lit word à PC, puis déréférence
	bus := &stubBus{}
	bus.mem[0x1000] = 0x9F    // post-byte
	bus.set16(0x1001, 0x2000) // word à déréférencer
	bus.set16(0x2000, 0x3456) // valeur pointée
	cpu := newCPUAt(bus, 0x1000)
	r := cpu.AddrIndexed()
	if r.Addr != 0x3456 {
		t.Errorf("AddrIndexed [word] addr = 0x%04X, want 0x3456", r.Addr)
	}
	if r.Extra != 5 {
		t.Errorf("AddrIndexed [word] Extra = %d, want 5", r.Extra)
	}
}

func TestAddrIndexedInvalidPostbyte(t *testing.T) {
	// Post-bytes invalides (ex: 0x90) : comportement = [,R] (déréférence read16(*X))
	// Ref: dc6809emul.c case 0x90 → W = Mgetw(*r), n=3
	bus := &stubBus{}
	bus.mem[0x1000] = 0x90    // post-byte invalide, registre X (bits 6:5 = 00)
	bus.set16(0x0000, 0x9ABC) // *X=0 → lit 0x9ABC
	cpu := newCPUAt(bus, 0x1000)
	r := cpu.AddrIndexed()
	if r.Addr != 0x9ABC {
		t.Errorf("AddrIndexed invalide 0x90 addr = 0x%04X, want 0x9ABC (déréférence [,X])", r.Addr)
	}
	if r.Extra != 3 {
		t.Errorf("AddrIndexed invalide 0x90 Extra = %d, want 3", r.Extra)
	}
}

func TestAddrIndexedRdirect(t *testing.T) {
	// Post-byte 0x94 : [,R] — déréférence *R
	bus := &stubBus{}
	bus.mem[0x1000] = 0x94    // post-byte : [,X]
	bus.set16(0x0000, 0x5678) // *X=0 → lit 0x5678
	cpu := newCPUAt(bus, 0x1000)
	r := cpu.AddrIndexed()
	if r.Addr != 0x5678 {
		t.Errorf("AddrIndexed [,X] addr = 0x%04X, want 0x5678", r.Addr)
	}
	if r.Extra != 3 {
		t.Errorf("AddrIndexed [,X] Extra = %d, want 3", r.Extra)
	}
}
