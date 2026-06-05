package cpu6809_test

import (
	"testing"

	"github.com/Lesur-ai/dcmo5/internal/cpu6809"
)

// ── ADD / ADC ────────────────────────────────────────────────────────────────

func TestADDA_noFlags(t *testing.T) {
	// ADDA #0x10 avec A=0x20 → A=0x30, N=Z=V=C=0
	cpu, bus := newCPUWithProg(0x1000, 0x8B, 0x10)
	bus.mem[0xFFFE] = 0x10
	cpu.Reset()
	loadA(bus, 0x1000, 0x20, 0x8B, 0x10)
	cpu.Reset()
	cpu.Step()
	cpu.Step()
	s := cpu.Snapshot()
	if s.A != 0x30 {
		t.Errorf("ADDA: A = 0x%02X, want 0x30", s.A)
	}
	assertFlags(t, s, "ADDA normal", false, false, false, false)
}

func TestADDA_carryOverflow(t *testing.T) {
	// ADDA #1 avec A=0xFF → A=0x00, C=1, Z=1, V=0
	cpu, bus := makeStep(0x1000)
	bus.mem[0x1000] = 0x86 // LDA #0xFF
	bus.mem[0x1001] = 0xFF
	bus.mem[0x1002] = 0x8B // ADDA #1
	bus.mem[0x1003] = 0x01
	cpu.Reset()
	cpu.Step()
	cpu.Step()
	s := cpu.Snapshot()
	if s.A != 0x00 {
		t.Errorf("ADDA 0xFF+1: A = 0x%02X, want 0x00", s.A)
	}
	if s.CC&cpu6809.FlagC == 0 {
		t.Error("ADDA 0xFF+1: C devrait être positionné")
	}
	if s.CC&cpu6809.FlagZ == 0 {
		t.Error("ADDA 0xFF+1: Z devrait être positionné")
	}
}

func TestADDA_signedOverflow(t *testing.T) {
	// ADDA #1 avec A=0x7F → A=0x80, V=1, N=1
	cpu, bus := makeStep(0x1000)
	bus.mem[0x1000] = 0x86
	bus.mem[0x1001] = 0x7F
	bus.mem[0x1002] = 0x8B
	bus.mem[0x1003] = 0x01
	cpu.Reset()
	cpu.Step()
	cpu.Step()
	s := cpu.Snapshot()
	if s.A != 0x80 {
		t.Errorf("ADDA overflow: A = 0x%02X, want 0x80", s.A)
	}
	if s.CC&cpu6809.FlagV == 0 {
		t.Error("ADDA 0x7F+1: V devrait être positionné")
	}
	if s.CC&cpu6809.FlagN == 0 {
		t.Error("ADDA 0x7F+1: N devrait être positionné")
	}
}

func TestADDA_halfCarry(t *testing.T) {
	// ADDA #0x08 avec A=0x08 → half-carry (0x08+0x08 = 0x10, nibble carry)
	cpu, bus := makeStep(0x1000)
	bus.mem[0x1000] = 0x86
	bus.mem[0x1001] = 0x08
	bus.mem[0x1002] = 0x8B
	bus.mem[0x1003] = 0x08
	cpu.Reset()
	cpu.Step()
	cpu.Step()
	s := cpu.Snapshot()
	if s.CC&cpu6809.FlagH == 0 {
		t.Errorf("ADDA half-carry: H devrait être positionné, CC=0x%02X", s.CC)
	}
}

// ── SUBA / CMP ───────────────────────────────────────────────────────────────

func TestSUBA_normal(t *testing.T) {
	// SUBA #0x10 avec A=0x30 → A=0x20
	cpu, bus := makeStep(0x1000)
	bus.mem[0x1000] = 0x86
	bus.mem[0x1001] = 0x30
	bus.mem[0x1002] = 0x80 // SUBA imm
	bus.mem[0x1003] = 0x10
	cpu.Reset()
	cpu.Step()
	cpu.Step()
	s := cpu.Snapshot()
	if s.A != 0x20 {
		t.Errorf("SUBA: A = 0x%02X, want 0x20", s.A)
	}
}

func TestSUBA_borrow(t *testing.T) {
	// SUBA #1 avec A=0x00 → A=0xFF, C=1, N=1
	cpu, bus := makeStep(0x1000)
	bus.mem[0x1000] = 0x86
	bus.mem[0x1001] = 0x00
	bus.mem[0x1002] = 0x80
	bus.mem[0x1003] = 0x01
	cpu.Reset()
	cpu.Step()
	cpu.Step()
	s := cpu.Snapshot()
	if s.A != 0xFF {
		t.Errorf("SUBA borrow: A = 0x%02X, want 0xFF", s.A)
	}
	if s.CC&cpu6809.FlagC == 0 {
		t.Error("SUBA 0x00-1: C devrait être positionné")
	}
}

func TestSUBA_signedOverflow(t *testing.T) {
	// SUBA #1 avec A=0x80 → A=0x7F, V=1 (overflow signé)
	cpu, bus := makeStep(0x1000)
	bus.mem[0x1000] = 0x86
	bus.mem[0x1001] = 0x80
	bus.mem[0x1002] = 0x80
	bus.mem[0x1003] = 0x01
	cpu.Reset()
	cpu.Step()
	cpu.Step()
	s := cpu.Snapshot()
	if s.A != 0x7F {
		t.Errorf("SUBA overflow: A = 0x%02X, want 0x7F", s.A)
	}
	if s.CC&cpu6809.FlagV == 0 {
		t.Error("SUBA 0x80-1: V devrait être positionné")
	}
}

func TestCMPA_noChange(t *testing.T) {
	// CMPA #0x42 avec A=0x42 → Z=1, A inchangé
	cpu, bus := makeStep(0x1000)
	bus.mem[0x1000] = 0x86
	bus.mem[0x1001] = 0x42
	bus.mem[0x1002] = 0x81 // CMPA imm
	bus.mem[0x1003] = 0x42
	cpu.Reset()
	cpu.Step()
	cpu.Step()
	s := cpu.Snapshot()
	if s.A != 0x42 {
		t.Errorf("CMPA: A modifié = 0x%02X", s.A)
	}
	if s.CC&cpu6809.FlagZ == 0 {
		t.Error("CMPA equal: Z devrait être positionné")
	}
}

// ── SBCA ─────────────────────────────────────────────────────────────────────

func TestSBCA_withBorrow(t *testing.T) {
	// LDA #0x05 ; ADDA #1 (pour avoir C=0) ; ... compliqué.
	// Approche directe : SBCA sur A=0x10, opérande=0x05, C=0 → 0x10-0x05-0=0x0B
	// Puis tester avec C=1 : 0x10-0x05-1=0x0A
	cpu, bus := makeStep(0x1000)
	// LDA #0x00 ; SUBA #0x01 (→ C=1, A=0xFF) ; LDA #0x10 ; SBCA #0x05
	bus.mem[0x1000] = 0x86 // LDA #0
	bus.mem[0x1001] = 0x00
	bus.mem[0x1002] = 0x80 // SUBA #1 → C=1
	bus.mem[0x1003] = 0x01
	bus.mem[0x1004] = 0x86 // LDA #0x10
	bus.mem[0x1005] = 0x10
	bus.mem[0x1006] = 0x82 // SBCA #0x05
	bus.mem[0x1007] = 0x05
	cpu.Reset()
	cpu.Step() // LDA #0
	cpu.Step() // SUBA #1 → C=1
	cpu.Step() // LDA #0x10
	cpu.Step() // SBCA #0x05 → 0x10-0x05-1 = 0x0A
	s := cpu.Snapshot()
	if s.A != 0x0A {
		t.Errorf("SBCA avec C=1: A = 0x%02X, want 0x0A", s.A)
	}
}

func TestSBCA_wrapBorrow(t *testing.T) {
	// A=0, opérande=0xFF, C=1 → 0-0xFF-1 = 0-256 → A=0, C=1
	cpu, bus := makeStep(0x1000)
	bus.mem[0x1000] = 0x86
	bus.mem[0x1001] = 0x00
	bus.mem[0x1002] = 0x80 // SUBA #1 → C=1
	bus.mem[0x1003] = 0x01
	bus.mem[0x1004] = 0x86 // LDA #0x00
	bus.mem[0x1005] = 0x00
	bus.mem[0x1006] = 0x82 // SBCA #0xFF
	bus.mem[0x1007] = 0xFF
	cpu.Reset()
	cpu.Step()
	cpu.Step() // C=1 après ça
	cpu.Step()
	cpu.Step() // SBCA #0xFF avec C=1 : 0 - 0xFF - 1 = 0 - 256
	s := cpu.Snapshot()
	// résultat : uint16(0) - uint16(0xFF) - 1 = 0xFF00, uint8 = 0x00
	if s.A != 0x00 {
		t.Errorf("SBCA wrap: A = 0x%02X, want 0x00", s.A)
	}
	if s.CC&cpu6809.FlagC == 0 {
		t.Error("SBCA wrap: C devrait être positionné (borrow)")
	}
}

// ── AND / OR / EOR ───────────────────────────────────────────────────────────

func TestANDA(t *testing.T) {
	cpu, bus := makeStep(0x1000)
	bus.mem[0x1000] = 0x86
	bus.mem[0x1001] = 0xF0
	bus.mem[0x1002] = 0x84 // ANDA imm
	bus.mem[0x1003] = 0x0F
	cpu.Reset()
	cpu.Step()
	cpu.Step()
	s := cpu.Snapshot()
	if s.A != 0x00 {
		t.Errorf("ANDA: A = 0x%02X, want 0x00", s.A)
	}
	if s.CC&cpu6809.FlagZ == 0 {
		t.Error("ANDA 0: Z devrait être positionné")
	}
}

func TestORA(t *testing.T) {
	cpu, bus := makeStep(0x1000)
	bus.mem[0x1000] = 0x86
	bus.mem[0x1001] = 0xA0
	bus.mem[0x1002] = 0x8A // ORA imm
	bus.mem[0x1003] = 0x0F
	cpu.Reset()
	cpu.Step()
	cpu.Step()
	s := cpu.Snapshot()
	if s.A != 0xAF {
		t.Errorf("ORA: A = 0x%02X, want 0xAF", s.A)
	}
}

func TestEORA(t *testing.T) {
	cpu, bus := makeStep(0x1000)
	bus.mem[0x1000] = 0x86
	bus.mem[0x1001] = 0xFF
	bus.mem[0x1002] = 0x88 // EORA imm
	bus.mem[0x1003] = 0x0F
	cpu.Reset()
	cpu.Step()
	cpu.Step()
	s := cpu.Snapshot()
	if s.A != 0xF0 {
		t.Errorf("EORA: A = 0x%02X, want 0xF0", s.A)
	}
}

// ── INC / DEC / NEG / COM / CLR / TST ────────────────────────────────────────

func TestINCA_overflow(t *testing.T) {
	// INCA avec A=0x7F → A=0x80, V=1, N=1
	cpu, bus := makeStep(0x1000)
	bus.mem[0x1000] = 0x86
	bus.mem[0x1001] = 0x7F
	bus.mem[0x1002] = 0x4C // INCA
	cpu.Reset()
	cpu.Step()
	cpu.Step()
	s := cpu.Snapshot()
	if s.A != 0x80 {
		t.Errorf("INCA overflow: A = 0x%02X, want 0x80", s.A)
	}
	if s.CC&cpu6809.FlagV == 0 {
		t.Error("INCA 0x7F: V devrait être positionné")
	}
}

func TestDECA_overflow(t *testing.T) {
	// DECA avec A=0x80 → A=0x7F, V=1
	cpu, bus := makeStep(0x1000)
	bus.mem[0x1000] = 0x86
	bus.mem[0x1001] = 0x80
	bus.mem[0x1002] = 0x4A // DECA
	cpu.Reset()
	cpu.Step()
	cpu.Step()
	s := cpu.Snapshot()
	if s.A != 0x7F {
		t.Errorf("DECA overflow: A = 0x%02X, want 0x7F", s.A)
	}
	if s.CC&cpu6809.FlagV == 0 {
		t.Error("DECA 0x80: V devrait être positionné")
	}
}

func TestNEGA(t *testing.T) {
	// NEGA avec A=0x01 → A=0xFF, C=1, N=1
	cpu, bus := makeStep(0x1000)
	bus.mem[0x1000] = 0x86
	bus.mem[0x1001] = 0x01
	bus.mem[0x1002] = 0x40 // NEGA
	cpu.Reset()
	cpu.Step()
	cpu.Step()
	s := cpu.Snapshot()
	if s.A != 0xFF {
		t.Errorf("NEGA: A = 0x%02X, want 0xFF", s.A)
	}
	if s.CC&cpu6809.FlagC == 0 {
		t.Error("NEGA 0x01: C devrait être positionné")
	}
}

func TestNEGA_overflow(t *testing.T) {
	// NEGA avec A=0x80 → A=0x80, V=1 (NEG(0x80) = -(-128) = +128 déborde)
	cpu, bus := makeStep(0x1000)
	bus.mem[0x1000] = 0x86
	bus.mem[0x1001] = 0x80
	bus.mem[0x1002] = 0x40
	cpu.Reset()
	cpu.Step()
	cpu.Step()
	s := cpu.Snapshot()
	if s.A != 0x80 {
		t.Errorf("NEGA 0x80: A = 0x%02X, want 0x80", s.A)
	}
	if s.CC&cpu6809.FlagV == 0 {
		t.Error("NEGA 0x80: V devrait être positionné")
	}
}

func TestCOMA(t *testing.T) {
	// COMA avec A=0xAA → A=0x55, C=1
	cpu, bus := makeStep(0x1000)
	bus.mem[0x1000] = 0x86
	bus.mem[0x1001] = 0xAA
	bus.mem[0x1002] = 0x43 // COMA
	cpu.Reset()
	cpu.Step()
	cpu.Step()
	s := cpu.Snapshot()
	if s.A != 0x55 {
		t.Errorf("COMA: A = 0x%02X, want 0x55", s.A)
	}
	if s.CC&cpu6809.FlagC == 0 {
		t.Error("COMA: C devrait être positionné (toujours)")
	}
	if s.CC&cpu6809.FlagV != 0 {
		t.Error("COMA: V devrait être effacé (toujours)")
	}
}

func TestCLRA(t *testing.T) {
	// CLRA avec A=0xFF → A=0, Z=1, N=V=C=0
	cpu, bus := makeStep(0x1000)
	bus.mem[0x1000] = 0x86
	bus.mem[0x1001] = 0xFF
	bus.mem[0x1002] = 0x4F // CLRA
	cpu.Reset()
	cpu.Step()
	cpu.Step()
	s := cpu.Snapshot()
	if s.A != 0x00 {
		t.Errorf("CLRA: A = 0x%02X, want 0x00", s.A)
	}
	if s.CC&cpu6809.FlagZ == 0 {
		t.Error("CLRA: Z devrait être positionné")
	}
	if s.CC&(cpu6809.FlagN|cpu6809.FlagV|cpu6809.FlagC) != 0 {
		t.Errorf("CLRA: N/V/C devraient être effacés, CC=0x%02X", s.CC)
	}
}

// ── MUL ──────────────────────────────────────────────────────────────────────

func TestMUL(t *testing.T) {
	// LDA #0x0A ; LDB #0x0A ; MUL → D=100=0x0064
	cpu, bus := makeStep(0x1000)
	bus.mem[0x1000] = 0x86 // LDA #10
	bus.mem[0x1001] = 0x0A
	bus.mem[0x1002] = 0xC6 // LDB #10
	bus.mem[0x1003] = 0x0A
	bus.mem[0x1004] = 0x3D // MUL
	cpu.Reset()
	cpu.Step()
	cpu.Step()
	cpu.Step()
	s := cpu.Snapshot()
	d := uint16(s.A)<<8 | uint16(s.B)
	if d != 100 {
		t.Errorf("MUL 10×10: D = %d, want 100", d)
	}
}

func TestMUL_carryFlag(t *testing.T) {
	// MUL avec résultat bit7(B)=1 → C=1
	// A=0x01, B=0x80 → D=0x0080, bit7(B)=1 → C=1
	cpu, bus := makeStep(0x1000)
	bus.mem[0x1000] = 0x86
	bus.mem[0x1001] = 0x01
	bus.mem[0x1002] = 0xC6
	bus.mem[0x1003] = 0x80
	bus.mem[0x1004] = 0x3D
	cpu.Reset()
	cpu.Step()
	cpu.Step()
	cpu.Step()
	s := cpu.Snapshot()
	if s.CC&cpu6809.FlagC == 0 {
		t.Error("MUL: C devrait être positionné quand bit7(B)=1")
	}
	d := uint16(s.A)<<8 | uint16(s.B)
	if d != 0x0080 {
		t.Errorf("MUL 1×128: D = 0x%04X, want 0x0080", d)
	}
}

// ── Décalages ─────────────────────────────────────────────────────────────────

func TestLSRA(t *testing.T) {
	// LSRA avec A=0x81 → A=0x40, C=1, N=0
	cpu, bus := makeStep(0x1000)
	bus.mem[0x1000] = 0x86
	bus.mem[0x1001] = 0x81
	bus.mem[0x1002] = 0x44 // LSRA
	cpu.Reset()
	cpu.Step()
	cpu.Step()
	s := cpu.Snapshot()
	if s.A != 0x40 {
		t.Errorf("LSRA: A = 0x%02X, want 0x40", s.A)
	}
	if s.CC&cpu6809.FlagC == 0 {
		t.Error("LSRA 0x81: C devrait être positionné")
	}
}

func TestASLA_carry(t *testing.T) {
	// ASLA avec A=0x81 → A=0x02, C=1 (bit7 sorti)
	cpu, bus := makeStep(0x1000)
	bus.mem[0x1000] = 0x86
	bus.mem[0x1001] = 0x81
	bus.mem[0x1002] = 0x48 // ASLA
	cpu.Reset()
	cpu.Step()
	cpu.Step()
	s := cpu.Snapshot()
	if s.A != 0x02 {
		t.Errorf("ASLA: A = 0x%02X, want 0x02", s.A)
	}
	if s.CC&cpu6809.FlagC == 0 {
		t.Error("ASLA 0x81: C devrait être positionné")
	}
}

func TestROLA(t *testing.T) {
	// ROLA avec A=0x80, C=0 → A=0x00, C=1 (bit7 → C, ancien C → bit0)
	cpu, bus := makeStep(0x1000)
	bus.mem[0x1000] = 0x86
	bus.mem[0x1001] = 0x80
	bus.mem[0x1002] = 0x49 // ROLA
	cpu.Reset()
	cpu.Step()
	cpu.Step()
	s := cpu.Snapshot()
	if s.A != 0x00 {
		t.Errorf("ROLA: A = 0x%02X, want 0x00", s.A)
	}
	if s.CC&cpu6809.FlagC == 0 {
		t.Error("ROLA 0x80: C devrait être positionné")
	}
}

func TestABX(t *testing.T) {
	// LDB #0x10 ; LDX #0x1000 ; ABX → X=0x1010
	cpu, bus := makeStep(0x1000)
	bus.mem[0x1000] = 0xC6 // LDB #0x10
	bus.mem[0x1001] = 0x10
	bus.mem[0x1002] = 0x8E // LDX #0x1000
	bus.set16(0x1003, 0x1000)
	bus.mem[0x1005] = 0x3A // ABX
	cpu.Reset()
	cpu.Step()
	cpu.Step()
	cpu.Step()
	s := cpu.Snapshot()
	if s.X != 0x1010 {
		t.Errorf("ABX: X = 0x%04X, want 0x1010", s.X)
	}
}

func TestSUBD(t *testing.T) {
	// LDD #0x0200 ; SUBD #0x0100 → D=0x0100
	cpu, bus := makeStep(0x1000)
	bus.mem[0x1000] = 0xCC // LDD imm
	bus.set16(0x1001, 0x0200)
	bus.mem[0x1003] = 0x83 // SUBD imm
	bus.set16(0x1004, 0x0100)
	cpu.Reset()
	cpu.Step()
	cpu.Step()
	s := cpu.Snapshot()
	d := uint16(s.A)<<8 | uint16(s.B)
	if d != 0x0100 {
		t.Errorf("SUBD: D = 0x%04X, want 0x0100", d)
	}
}

func TestADDD(t *testing.T) {
	// LDD #0x0100 ; ADDD #0x0200 → D=0x0300
	cpu, bus := makeStep(0x1000)
	bus.mem[0x1000] = 0xCC // LDD imm
	bus.set16(0x1001, 0x0100)
	bus.mem[0x1003] = 0xC3 // ADDD imm
	bus.set16(0x1004, 0x0200)
	cpu.Reset()
	cpu.Step()
	cpu.Step()
	s := cpu.Snapshot()
	d := uint16(s.A)<<8 | uint16(s.B)
	if d != 0x0300 {
		t.Errorf("ADDD: D = 0x%04X, want 0x0300", d)
	}
}

func TestADDD_overflow(t *testing.T) {
	// LDD #0x7FFF ; ADDD #0x0001 → D=0x8000, V=1
	cpu, bus := makeStep(0x1000)
	bus.mem[0x1000] = 0xCC
	bus.set16(0x1001, 0x7FFF)
	bus.mem[0x1003] = 0xC3
	bus.set16(0x1004, 0x0001)
	cpu.Reset()
	cpu.Step()
	cpu.Step()
	s := cpu.Snapshot()
	d := uint16(s.A)<<8 | uint16(s.B)
	if d != 0x8000 {
		t.Errorf("ADDD overflow: D = 0x%04X, want 0x8000", d)
	}
	if s.CC&cpu6809.FlagV == 0 {
		t.Error("ADDD 0x7FFF+1: V devrait être positionné")
	}
}

// ── Helpers ──────────────────────────────────────────────────────────────────

func makeStep(pc uint16) (*cpu6809.CPU, *stubBus) {
	bus := &stubBus{}
	bus.set16(0xFFFE, pc)
	cpu := cpu6809.New(bus)
	cpu.Reset()
	return cpu, bus
}

func newCPUWithProg(pc uint16, prog ...uint8) (*cpu6809.CPU, *stubBus) {
	bus := &stubBus{}
	bus.set16(0xFFFE, pc)
	for i, b := range prog {
		bus.mem[int(pc)+i] = b
	}
	cpu := cpu6809.New(bus)
	cpu.Reset()
	return cpu, bus
}

func loadA(bus *stubBus, pc uint16, aVal, addOp, addArg uint8) {
	bus.mem[pc] = 0x86
	bus.mem[pc+1] = aVal
	bus.mem[pc+2] = addOp
	bus.mem[pc+3] = addArg
}

func assertFlags(t *testing.T, s cpu6809.Snapshot, label string, wantN, wantZ, wantV, wantC bool) {
	t.Helper()
	got := func(f uint8) bool { return s.CC&f != 0 }
	if got(cpu6809.FlagN) != wantN {
		t.Errorf("%s: N = %v, want %v", label, got(cpu6809.FlagN), wantN)
	}
	if got(cpu6809.FlagZ) != wantZ {
		t.Errorf("%s: Z = %v, want %v", label, got(cpu6809.FlagZ), wantZ)
	}
	if got(cpu6809.FlagV) != wantV {
		t.Errorf("%s: V = %v, want %v", label, got(cpu6809.FlagV), wantV)
	}
	if got(cpu6809.FlagC) != wantC {
		t.Errorf("%s: C = %v, want %v", label, got(cpu6809.FlagC), wantC)
	}
}
