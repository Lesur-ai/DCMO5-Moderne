package cpu6809_test

import (
	"testing"

	"github.com/Lesur-ai/dcmo5/internal/cpu6809"
)

// golden_test.go — séquences déterministes pour valider la conformité du CPU.
//
// Les résultats attendus sont calculés depuis la spec 6809 officielle,
// indépendamment du code C de référence. Chaque test documente son programme
// et les invariants vérifiés.

// ── Boot sequence ─────────────────────────────────────────────────────────────

// TestGolden_Boot valide la séquence de démarrage : le CPU charge le vecteur
// reset et exécute les premières instructions correctement.
//
// Programme à 0x2000 :
//
//	LDA #0x42      (0x86 0x42)      — 2 cycles
//	STA $0050      (0x97 0x50)      — 4 cycles  [direct, DP=0]
//	NOP            (0x12)           — 2 cycles
//
// Invariants : A=0x42, mem[0x0050]=0x42, PC=0x2005, 8 cycles totaux.
func TestGolden_Boot(t *testing.T) {
	bus := &stubBus{}
	bus.set16(0xFFFE, 0x2000) // vecteur reset
	bus.mem[0x2000] = 0x86    // LDA imm
	bus.mem[0x2001] = 0x42
	bus.mem[0x2002] = 0x97 // STA direct
	bus.mem[0x2003] = 0x50
	bus.mem[0x2004] = 0x12 // NOP

	cpu := cpu6809.New(bus)
	cpu.Reset()

	totalCycles := 0
	for i := 0; i < 3; i++ {
		totalCycles += cpu.Step()
	}

	s := cpu.Snapshot()
	if s.A != 0x42 {
		t.Errorf("Boot: A = 0x%02X, want 0x42", s.A)
	}
	if bus.mem[0x0050] != 0x42 {
		t.Errorf("Boot: mem[0x0050] = 0x%02X, want 0x42", bus.mem[0x0050])
	}
	if s.PC != 0x2005 {
		t.Errorf("Boot: PC = 0x%04X, want 0x2005", s.PC)
	}
	if totalCycles != 8 {
		t.Errorf("Boot: cycles = %d, want 8 (LDA:2 + STA:4 + NOP:2)", totalCycles)
	}
}

// ── Boucle compteur ───────────────────────────────────────────────────────────

// TestGolden_CounterLoop valide une boucle de décompte classique.
//
// Programme à 0x1000 :
//
//	LDB #3     (0xC6 0x03)   — 2 cycles
//	loop:
//	DECB       (0x5A)        — 2 cycles
//	BNE loop   (0x26 0xFD)   — 3 cycles (taken) / 3 (not taken)
//
// Après 3 itérations : B=0, Z=1, PC=0x1005 (après le BNE non pris).
// Cycles : 2 + 3*(2+3) - (3-3) = 2 + 15 = 17
// (3 DECB × 2cy = 6, 2 BNE taken × 3cy = 6, 1 BNE not-taken × 3cy = 3) + LDB:2 = 17
func TestGolden_CounterLoop(t *testing.T) {
	bus := &stubBus{}
	bus.set16(0xFFFE, 0x1000)
	bus.mem[0x1000] = 0xC6 // LDB imm
	bus.mem[0x1001] = 0x03
	bus.mem[0x1002] = 0x5A // DECB
	bus.mem[0x1003] = 0x26 // BNE
	bus.mem[0x1004] = 0xFD // offset -3 (revient à 0x1002)

	cpu := cpu6809.New(bus)
	cpu.Reset()

	totalCycles := cpu.Step() // LDB #3
	// 3 itérations : DECB + BNE
	for i := 0; i < 3; i++ {
		totalCycles += cpu.Step() // DECB
		totalCycles += cpu.Step() // BNE
	}

	s := cpu.Snapshot()
	if s.B != 0 {
		t.Errorf("CounterLoop: B = %d, want 0", s.B)
	}
	if s.CC&cpu6809.FlagZ == 0 {
		t.Error("CounterLoop: Z devrait être positionné après boucle")
	}
	if s.PC != 0x1005 {
		t.Errorf("CounterLoop: PC = 0x%04X, want 0x1005", s.PC)
	}
	// 2 + 3*2 + 2*3 + 3 = 2 + 6 + 6 + 3 = 17
	if totalCycles != 17 {
		t.Errorf("CounterLoop: cycles = %d, want 17", totalCycles)
	}
}

// ── Appel de sous-routine ─────────────────────────────────────────────────────

// TestGolden_SubroutineCall valide un appel JSR + retour RTS avec pile.
//
// Programme à 0x1000 :
//
//	LDS #0x0300    (0x10CE 0x0300)  — 4 cycles  [LDS imm préfixe 0x10]
//	LDA #0x10      (0x86 0x10)      — 2 cycles
//	JSR $0x2000    (0xBD 0x2000)    — 8 cycles
//	LDA #0xFF      (0x86 0xFF)      — 2 cycles  [never reached in this test]
//
// Sous-routine à 0x2000 :
//
//	ADDA #0x05     (0x8B 0x05)      — 2 cycles
//	RTS            (0x39)           — 5 cycles
//
// Invariants : A=0x15, PC=0x100A (après JSR 3 octets), cycles=4+2+8+2+5=21
func TestGolden_SubroutineCall(t *testing.T) {
	bus := &stubBus{}
	bus.set16(0xFFFE, 0x1000)
	// LDS #0x0300 (préfixe 0x10 + 0xCE)
	bus.mem[0x1000] = 0x10
	bus.mem[0x1001] = 0xCE
	bus.set16(0x1002, 0x0300)
	// LDA #0x10
	bus.mem[0x1004] = 0x86
	bus.mem[0x1005] = 0x10
	// JSR $0x2000
	bus.mem[0x1006] = 0xBD
	bus.set16(0x1007, 0x2000)
	// après JSR (adresse retour = 0x1009)
	bus.mem[0x1009] = 0x86 // LDA #0xFF (jamais atteint dans ce test)
	bus.mem[0x100A] = 0xFF

	// sous-routine
	bus.mem[0x2000] = 0x8B // ADDA imm
	bus.mem[0x2001] = 0x05
	bus.mem[0x2002] = 0x39 // RTS

	cpu := cpu6809.New(bus)
	cpu.Reset()

	totalCycles := 0
	totalCycles += cpu.Step() // LDS
	totalCycles += cpu.Step() // LDA #0x10
	totalCycles += cpu.Step() // JSR
	totalCycles += cpu.Step() // ADDA #5
	totalCycles += cpu.Step() // RTS

	s := cpu.Snapshot()
	if s.A != 0x15 {
		t.Errorf("Subroutine: A = 0x%02X, want 0x15", s.A)
	}
	if s.PC != 0x1009 {
		t.Errorf("Subroutine: PC = 0x%04X, want 0x1009", s.PC)
	}
	if totalCycles != 21 {
		t.Errorf("Subroutine: cycles = %d, want 21 (LDS:4+LDA:2+JSR:8+ADDA:2+RTS:5)", totalCycles)
	}
}

// ── Déterminisme : même entrée → même sortie ──────────────────────────────────

// TestGolden_Determinism vérifie que deux CPU indépendants avec le même bus
// et le même programme produisent des snapshots identiques.
func TestGolden_Determinism(t *testing.T) {
	prog := func() *stubBus {
		bus := &stubBus{}
		bus.set16(0xFFFE, 0x1000)
		bus.mem[0x1000] = 0x86 // LDA #0xAB
		bus.mem[0x1001] = 0xAB
		bus.mem[0x1002] = 0xC6 // LDB #0xCD
		bus.mem[0x1003] = 0xCD
		bus.mem[0x1004] = 0x3D // MUL
		return bus
	}

	runN := func(n int) cpu6809.Snapshot {
		bus := prog()
		cpu := cpu6809.New(bus)
		cpu.Reset()
		for i := 0; i < n; i++ {
			cpu.Step()
		}
		return cpu.Snapshot()
	}

	s1 := runN(3)
	s2 := runN(3)
	if s1 != s2 {
		t.Errorf("Déterminisme: snapshots différents après même programme\ns1=%+v\ns2=%+v", s1, s2)
	}
}

// ── Invariant bus : Read8/Write8 symétriques ──────────────────────────────────

// TestGolden_BusRoundtrip vérifie que Store + Load retourne la valeur originale.
func TestGolden_BusRoundtrip(t *testing.T) {
	bus := &stubBus{}
	bus.set16(0xFFFE, 0x1000)
	// LDA #0x42 ; STA $3000 ; LDA #0x00 ; LDA $3000 (extended)
	bus.mem[0x1000] = 0x86 // LDA #0x42
	bus.mem[0x1001] = 0x42
	bus.mem[0x1002] = 0xB7 // STA ext
	bus.set16(0x1003, 0x3000)
	bus.mem[0x1005] = 0x86 // LDA #0x00 (écrase A)
	bus.mem[0x1006] = 0x00
	bus.mem[0x1007] = 0xB6 // LDA ext (recharge depuis mémoire)
	bus.set16(0x1008, 0x3000)

	cpu := cpu6809.New(bus)
	cpu.Reset()
	for i := 0; i < 4; i++ {
		cpu.Step()
	}
	s := cpu.Snapshot()
	if s.A != 0x42 {
		t.Errorf("BusRoundtrip: A = 0x%02X, want 0x42", s.A)
	}
}

// ── Séquence multi-instructions avec comptage précis ─────────────────────────

// TestGolden_CycleCount valide le comptage de cycles sur une séquence mixte.
//
// LDA #0x01  (2) + ADDA #0x01 (2) × 4 + NOP (2) = 12 cycles
func TestGolden_CycleCount(t *testing.T) {
	bus := &stubBus{}
	bus.set16(0xFFFE, 0x1000)
	bus.mem[0x1000] = 0x86 // LDA #1
	bus.mem[0x1001] = 0x01
	for i := 0; i < 4; i++ {
		bus.mem[0x1002+i*2] = 0x8B // ADDA imm
		bus.mem[0x1003+i*2] = 0x01
	}
	bus.mem[0x100A] = 0x12 // NOP

	cpu := cpu6809.New(bus)
	cpu.Reset()

	total := 0
	for i := 0; i < 6; i++ {
		total += cpu.Step()
	}

	s := cpu.Snapshot()
	if s.A != 5 {
		t.Errorf("CycleCount: A = %d, want 5", s.A)
	}
	if total != 12 {
		t.Errorf("CycleCount: total cycles = %d, want 12", total)
	}
}

// ── Opcodes illégaux : pas de comportement silencieux ─────────────────────────

// TestGolden_IllegalOpcode vérifie que les opcodes illégaux ne paniquent pas
// et consomment au moins 1 cycle (NOP implicite).
func TestGolden_IllegalOpcode(t *testing.T) {
	bus := &stubBus{}
	bus.set16(0xFFFE, 0x1000)
	bus.mem[0x1000] = 0x01 // opcode illégal sur 6809 (pas dans le set)

	cpu := cpu6809.New(bus)
	cpu.Reset()

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("opcode illégal 0x01 a causé une panique : %v", r)
		}
	}()
	cycles := cpu.Step()
	if cycles < 1 {
		t.Errorf("opcode illégal: cycles = %d, want >= 1", cycles)
	}
}
