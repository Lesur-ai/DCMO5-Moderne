package cpu6809_test

import (
	"testing"

	"github.com/Lesur-ai/dcmo5/internal/cpu6809"
)

// ── BRA / BRN ─────────────────────────────────────────────────────────────────

func TestBRA_taken(t *testing.T) {
	// BRA +2 : PC doit avancer de 2 au-delà de l'instruction
	cpu, bus := makeStep(0x1000)
	bus.mem[0x1000] = 0x20 // BRA
	bus.mem[0x1001] = 0x02 // +2
	cpu.Reset()
	cycles := cpu.Step()
	s := cpu.Snapshot()
	// PC après BRA : 0x1002 (après opcode+offset) + 2 = 0x1004
	if s.PC != 0x1004 {
		t.Errorf("BRA +2 : PC = 0x%04X, want 0x1004", s.PC)
	}
	if cycles != 3 {
		t.Errorf("BRA cycles = %d, want 3", cycles)
	}
}

func TestBRA_negative(t *testing.T) {
	// BRA -2 (0xFE) : branche en arrière
	cpu, bus := makeStep(0x1000)
	bus.mem[0x1000] = 0x20
	bus.mem[0x1001] = 0xFE // -2 en signé
	cpu.Reset()
	cpu.Step()
	s := cpu.Snapshot()
	// PC après fetch = 0x1002, -2 → 0x1000
	if s.PC != 0x1000 {
		t.Errorf("BRA -2 : PC = 0x%04X, want 0x1000", s.PC)
	}
}

// ── Bcc (conditions) ──────────────────────────────────────────────────────────

func TestBEQ_taken(t *testing.T) {
	// CMPA #0x42 avec A=0x42 → Z=1 → BEQ taken
	cpu, bus := makeStep(0x1000)
	bus.mem[0x1000] = 0x86 // LDA #0x42
	bus.mem[0x1001] = 0x42
	bus.mem[0x1002] = 0x81 // CMPA #0x42
	bus.mem[0x1003] = 0x42
	bus.mem[0x1004] = 0x27 // BEQ +4
	bus.mem[0x1005] = 0x04
	cpu.Reset()
	cpu.Step()
	cpu.Step()
	cpu.Step()
	s := cpu.Snapshot()
	if s.PC != 0x100A {
		t.Errorf("BEQ taken: PC = 0x%04X, want 0x100A", s.PC)
	}
}

func TestBEQ_notTaken(t *testing.T) {
	// CMPA #0x42 avec A=0x43 → Z=0 → BEQ not taken
	cpu, bus := makeStep(0x1000)
	bus.mem[0x1000] = 0x86
	bus.mem[0x1001] = 0x43
	bus.mem[0x1002] = 0x81
	bus.mem[0x1003] = 0x42
	bus.mem[0x1004] = 0x27
	bus.mem[0x1005] = 0x04
	cpu.Reset()
	cpu.Step()
	cpu.Step()
	cpu.Step()
	s := cpu.Snapshot()
	if s.PC != 0x1006 {
		t.Errorf("BEQ not taken: PC = 0x%04X, want 0x1006", s.PC)
	}
}

func TestBNE_taken(t *testing.T) {
	cpu, bus := makeStep(0x1000)
	bus.mem[0x1000] = 0x86
	bus.mem[0x1001] = 0x01
	bus.mem[0x1002] = 0x81 // CMPA #0x02 → Z=0
	bus.mem[0x1003] = 0x02
	bus.mem[0x1004] = 0x26 // BNE +2
	bus.mem[0x1005] = 0x02
	cpu.Reset()
	cpu.Step()
	cpu.Step()
	cpu.Step()
	s := cpu.Snapshot()
	if s.PC != 0x1008 {
		t.Errorf("BNE taken: PC = 0x%04X, want 0x1008", s.PC)
	}
}

func TestBCC_taken(t *testing.T) {
	// LDA #0x10 ; SUBA #0x05 → C=0 → BCC taken
	cpu, bus := makeStep(0x1000)
	bus.mem[0x1000] = 0x86
	bus.mem[0x1001] = 0x10
	bus.mem[0x1002] = 0x80 // SUBA #0x05
	bus.mem[0x1003] = 0x05
	bus.mem[0x1004] = 0x24 // BCC +2
	bus.mem[0x1005] = 0x02
	cpu.Reset()
	cpu.Step()
	cpu.Step()
	cpu.Step()
	s := cpu.Snapshot()
	if s.PC != 0x1008 {
		t.Errorf("BCC taken: PC = 0x%04X, want 0x1008", s.PC)
	}
}

func TestBCS_taken(t *testing.T) {
	// LDA #0x00 ; SUBA #0x01 → C=1 → BCS taken
	cpu, bus := makeStep(0x1000)
	bus.mem[0x1000] = 0x86
	bus.mem[0x1001] = 0x00
	bus.mem[0x1002] = 0x80 // SUBA #1
	bus.mem[0x1003] = 0x01
	bus.mem[0x1004] = 0x25 // BCS +2
	bus.mem[0x1005] = 0x02
	cpu.Reset()
	cpu.Step()
	cpu.Step()
	cpu.Step()
	s := cpu.Snapshot()
	if s.PC != 0x1008 {
		t.Errorf("BCS taken: PC = 0x%04X, want 0x1008", s.PC)
	}
}

func TestBMI_taken(t *testing.T) {
	// LDA #0x80 → N=1 → BMI taken
	cpu, bus := makeStep(0x1000)
	bus.mem[0x1000] = 0x86
	bus.mem[0x1001] = 0x80
	bus.mem[0x1002] = 0x2B // BMI +2
	bus.mem[0x1003] = 0x02
	cpu.Reset()
	cpu.Step()
	cpu.Step()
	s := cpu.Snapshot()
	if s.PC != 0x1006 {
		t.Errorf("BMI taken: PC = 0x%04X, want 0x1006", s.PC)
	}
}

func TestBPL_taken(t *testing.T) {
	// LDA #0x01 → N=0 → BPL taken
	cpu, bus := makeStep(0x1000)
	bus.mem[0x1000] = 0x86
	bus.mem[0x1001] = 0x01
	bus.mem[0x1002] = 0x2A // BPL +2
	bus.mem[0x1003] = 0x02
	cpu.Reset()
	cpu.Step()
	cpu.Step()
	s := cpu.Snapshot()
	if s.PC != 0x1006 {
		t.Errorf("BPL taken: PC = 0x%04X, want 0x1006", s.PC)
	}
}

func TestBGE_taken(t *testing.T) {
	// N=V=0 → BGE taken. LDA #0x01 ; CMPA #0x00 → N=0, V=0
	cpu, bus := makeStep(0x1000)
	bus.mem[0x1000] = 0x86
	bus.mem[0x1001] = 0x01
	bus.mem[0x1002] = 0x81 // CMPA #0
	bus.mem[0x1003] = 0x00
	bus.mem[0x1004] = 0x2C // BGE +2
	bus.mem[0x1005] = 0x02
	cpu.Reset()
	cpu.Step()
	cpu.Step()
	cpu.Step()
	s := cpu.Snapshot()
	if s.PC != 0x1008 {
		t.Errorf("BGE taken: PC = 0x%04X, want 0x1008", s.PC)
	}
}

func TestBGT_taken(t *testing.T) {
	// Z=0, N=V=0 → BGT taken. LDA #0x02 ; CMPA #0x01
	cpu, bus := makeStep(0x1000)
	bus.mem[0x1000] = 0x86
	bus.mem[0x1001] = 0x02
	bus.mem[0x1002] = 0x81
	bus.mem[0x1003] = 0x01
	bus.mem[0x1004] = 0x2E // BGT +2
	bus.mem[0x1005] = 0x02
	cpu.Reset()
	cpu.Step()
	cpu.Step()
	cpu.Step()
	s := cpu.Snapshot()
	if s.PC != 0x1008 {
		t.Errorf("BGT taken: PC = 0x%04X, want 0x1008", s.PC)
	}
}

// ── LBRA ──────────────────────────────────────────────────────────────────────

func TestLBRA(t *testing.T) {
	// LBRA +0x0100 : saut long
	cpu, bus := makeStep(0x1000)
	bus.mem[0x1000] = 0x16 // LBRA
	bus.set16(0x1001, 0x0100)
	cpu.Reset()
	cycles := cpu.Step()
	s := cpu.Snapshot()
	// PC après fetch = 0x1003, + 0x100 = 0x1103
	if s.PC != 0x1103 {
		t.Errorf("LBRA +0x100: PC = 0x%04X, want 0x1103", s.PC)
	}
	if cycles != 5 {
		t.Errorf("LBRA cycles = %d, want 5", cycles)
	}
}

// ── JSR / RTS ──────────────────────────────────────────────────────────────────

func TestJSR_RTS(t *testing.T) {
	// Programme : JSR $2000 ; <ret addr> ; ... ; $2000: RTS
	cpu, bus := makeStep(0x1000)
	// Mettre S à 0x0100 via LDS imm (0x10CE)
	bus.mem[0x1000] = 0x10 // LDS prefix
	bus.mem[0x1001] = 0xCE
	bus.set16(0x1002, 0x0100) // S = 0x0100
	bus.mem[0x1004] = 0xBD    // JSR ext
	bus.set16(0x1005, 0x2000)
	bus.mem[0x2000] = 0x39 // RTS
	cpu.Reset()
	cpu.Step() // LDS #0x0100
	cpu.Step() // JSR $2000
	s := cpu.Snapshot()
	if s.PC != 0x2000 {
		t.Errorf("JSR: PC = 0x%04X, want 0x2000", s.PC)
	}
	// La pile doit contenir l'adresse de retour (0x1007)
	retHi := bus.mem[s.S]
	retLo := bus.mem[s.S+1]
	retAddr := uint16(retHi)<<8 | uint16(retLo)
	if retAddr != 0x1007 {
		t.Errorf("JSR ret addr sur pile = 0x%04X, want 0x1007", retAddr)
	}
	cpu.Step() // RTS
	s = cpu.Snapshot()
	if s.PC != 0x1007 {
		t.Errorf("RTS: PC = 0x%04X, want 0x1007", s.PC)
	}
}

// ── RTI ───────────────────────────────────────────────────────────────────────

func TestRTI_partialStack(t *testing.T) {
	// RTI avec E=0 : dépile CC + PC uniquement (6 cycles)
	cpu, bus := makeStep(0x1000)
	// Mettre S à 0x0100
	bus.mem[0x1000] = 0x10
	bus.mem[0x1001] = 0xCE
	bus.set16(0x1002, 0x00FE)
	// Empiler manuellement : CC=0x00 (E=0), PC=0x2000
	bus.mem[0x00FE] = 0x00    // CC (E=0)
	bus.set16(0x00FF, 0x2000) // PC
	bus.mem[0x1004] = 0x3B    // RTI
	cpu.Reset()
	cpu.Step()           // LDS
	cycles := cpu.Step() // RTI
	s := cpu.Snapshot()
	if s.PC != 0x2000 {
		t.Errorf("RTI (E=0): PC = 0x%04X, want 0x2000", s.PC)
	}
	if cycles != 6 {
		t.Errorf("RTI (E=0) cycles = %d, want 6", cycles)
	}
}

func TestRTI_fullStack(t *testing.T) {
	// RTI avec E=1 : dépile CC+A+B+DP+X+Y+U+PC (15 cycles)
	cpu, bus := makeStep(0x1000)
	bus.mem[0x1000] = 0x10
	bus.mem[0x1001] = 0xCE
	bus.set16(0x1002, 0x00F4)
	// Empiler : CC=0x80 (E=1), A=0x11, B=0x22, DP=0x33, X=0x1234, Y=0x5678, U=0x9ABC, PC=0x3000
	var sp uint16 = 0x00F4
	bus.mem[sp] = 0x80       // CC (E=1)
	bus.mem[sp+1] = 0x11     // A
	bus.mem[sp+2] = 0x22     // B
	bus.mem[sp+3] = 0x33     // DP
	bus.set16(sp+4, 0x1234)  // X
	bus.set16(sp+6, 0x5678)  // Y
	bus.set16(sp+8, 0x9ABC)  // U
	bus.set16(sp+10, 0x3000) // PC
	bus.mem[0x1004] = 0x3B   // RTI
	cpu.Reset()
	cpu.Step()           // LDS
	cycles := cpu.Step() // RTI
	s := cpu.Snapshot()
	if s.PC != 0x3000 {
		t.Errorf("RTI (E=1): PC = 0x%04X, want 0x3000", s.PC)
	}
	if s.A != 0x11 {
		t.Errorf("RTI (E=1): A = 0x%02X, want 0x11", s.A)
	}
	if s.X != 0x1234 {
		t.Errorf("RTI (E=1): X = 0x%04X, want 0x1234", s.X)
	}
	if cycles != 15 {
		t.Errorf("RTI (E=1) cycles = %d, want 15", cycles)
	}
}

// ── Interruptions hardware ────────────────────────────────────────────────────

func TestNMI(t *testing.T) {
	cpu, bus := makeStep(0x1000)
	// Vecteur NMI = 0x3000
	bus.set16(0xFFFC, 0x3000)
	// S = 0x0200
	bus.mem[0x1000] = 0x10
	bus.mem[0x1001] = 0xCE
	bus.set16(0x1002, 0x0200)
	bus.mem[0x1004] = 0x12 // NOP (pour que le PC soit propre)
	cpu.Reset()
	cpu.Step() // LDS
	cpu.Step() // NOP
	cpu.NMI()
	s := cpu.Snapshot()
	if s.PC != 0x3000 {
		t.Errorf("NMI: PC = 0x%04X, want 0x3000", s.PC)
	}
	if s.CC&cpu6809.FlagE == 0 {
		t.Error("NMI: E devrait être positionné")
	}
	if s.CC&cpu6809.FlagI == 0 {
		t.Error("NMI: I devrait être masqué")
	}
}

func TestIRQ_masked(t *testing.T) {
	// IRQ quand FlagI=1 : ignorée
	cpu, bus := makeStep(0x1000)
	bus.set16(0xFFF8, 0x4000)
	bus.mem[0x1000] = 0x12 // NOP
	cpu.Reset()            // CC = 0x50 (I=1)
	cpu.Step()
	pcBefore := cpu.Snapshot().PC
	cpu.IRQ()
	if cpu.Snapshot().PC != pcBefore {
		t.Error("IRQ masquée devrait ne pas modifier PC")
	}
}

func TestIRQ_accepted(t *testing.T) {
	// IRQ quand FlagI=0 : acceptée, charge vecteur 0xFFF8
	cpu, bus := makeStep(0x1000)
	bus.set16(0xFFF8, 0x4000)
	// LDS, puis ANDCC #~I pour démasquer IRQ
	bus.mem[0x1000] = 0x10
	bus.mem[0x1001] = 0xCE
	bus.set16(0x1002, 0x0200)
	bus.mem[0x1004] = 0x1C           // ANDCC
	bus.mem[0x1005] = ^cpu6809.FlagI // efface I
	cpu.Reset()
	cpu.Step() // LDS
	cpu.Step() // ANDCC
	cpu.IRQ()
	s := cpu.Snapshot()
	if s.PC != 0x4000 {
		t.Errorf("IRQ accepted: PC = 0x%04X, want 0x4000", s.PC)
	}
}

func TestFIRQ_partial(t *testing.T) {
	// FIRQ : empile seulement CC + PC (E=0)
	cpu, bus := makeStep(0x1000)
	bus.set16(0xFFF6, 0x5000)
	bus.mem[0x1000] = 0x10
	bus.mem[0x1001] = 0xCE
	bus.set16(0x1002, 0x0200)
	bus.mem[0x1004] = 0x1C // ANDCC : efface F
	bus.mem[0x1005] = ^cpu6809.FlagF
	cpu.Reset()
	cpu.Step() // LDS
	cpu.Step() // ANDCC
	sBefore := cpu.Snapshot()
	cpu.FIRQ()
	s := cpu.Snapshot()
	if s.PC != 0x5000 {
		t.Errorf("FIRQ: PC = 0x%04X, want 0x5000", s.PC)
	}
	// FIRQ empile CC (1 octet) + PC (2 octets) = 3 octets
	if s.S != sBefore.S-3 {
		t.Errorf("FIRQ: S = 0x%04X, want 0x%04X (3 octets empilés)", s.S, sBefore.S-3)
	}
	if s.CC&cpu6809.FlagE != 0 {
		t.Error("FIRQ: E devrait être efface (état partiel)")
	}
}

// ── SWI ───────────────────────────────────────────────────────────────────────

func TestSWI(t *testing.T) {
	// SWI : charge vecteur 0xFFFA, empile tout, I+F masqués
	cpu, bus := makeStep(0x1000)
	bus.set16(0xFFFA, 0x6000)
	bus.mem[0x1000] = 0x10
	bus.mem[0x1001] = 0xCE
	bus.set16(0x1002, 0x0200)
	bus.mem[0x1004] = 0x3F // SWI
	cpu.Reset()
	cpu.Step() // LDS
	cpu.Step() // SWI
	s := cpu.Snapshot()
	if s.PC != 0x6000 {
		t.Errorf("SWI: PC = 0x%04X, want 0x6000", s.PC)
	}
	if s.CC&cpu6809.FlagI == 0 {
		t.Error("SWI: I devrait être masqué")
	}
}

// ── BSR ───────────────────────────────────────────────────────────────────────

func TestBSR(t *testing.T) {
	// LDS #0x0100 ; BSR +0x10 : retour = 0x1006, PC = 0x1006+0x10 = 0x1016
	cpu, bus := makeStep(0x1000)
	bus.mem[0x1000] = 0x10
	bus.mem[0x1001] = 0xCE
	bus.set16(0x1002, 0x0100)
	bus.mem[0x1004] = 0x8D // BSR
	bus.mem[0x1005] = 0x10 // +16
	cpu.Reset()
	cpu.Step() // LDS
	cpu.Step() // BSR
	s := cpu.Snapshot()
	if s.PC != 0x1016 {
		t.Errorf("BSR +16: PC = 0x%04X, want 0x1016", s.PC)
	}
	// Retour sur pile = 0x1006
	retAddr := uint16(bus.mem[s.S])<<8 | uint16(bus.mem[s.S+1])
	if retAddr != 0x1006 {
		t.Errorf("BSR ret addr = 0x%04X, want 0x1006", retAddr)
	}
}

func TestBSR_zero(t *testing.T) {
	// BSR 0 : doit atterrir sur l'instruction suivante (pas sur l'octet offset)
	cpu, bus := makeStep(0x1000)
	bus.mem[0x1000] = 0x10
	bus.mem[0x1001] = 0xCE
	bus.set16(0x1002, 0x0100)
	bus.mem[0x1004] = 0x8D // BSR
	bus.mem[0x1005] = 0x00 // offset 0
	cpu.Reset()
	cpu.Step() // LDS
	cpu.Step() // BSR +0
	s := cpu.Snapshot()
	// PC après fetch offset = 0x1006, + 0 = 0x1006
	if s.PC != 0x1006 {
		t.Errorf("BSR 0: PC = 0x%04X, want 0x1006", s.PC)
	}
}
