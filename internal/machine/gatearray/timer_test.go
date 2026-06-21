package gatearray_test

// Tests de timing IRQ déterministes du timer 6846 (#114) : timeout → assertion de
// ligne, durée d'impulsion, clear conditionnel, prescaler, latch. Le timer est
// avancé manuellement via OnInstructionCycles(c, &irq), comme le ferait le moteur.

import (
	"testing"

	"github.com/Lesur-ai/dcmo5/internal/machine"
	"github.com/Lesur-ai/dcmo5/internal/machine/gatearray"
)

// armTimer règle le latch à `latch`, recharge le compteur, et active/désactive le
// prescaler. Laisse le timer ACTIVÉ (countdown) au retour.
func armTimer(g *gatearray.GateArray, latch int, prescaler bool) {
	g.Write8(0xE7C6, byte(latch>>8))
	g.Write8(0xE7C7, byte(latch))
	ctrl := byte(0x01) // bit0=1 → recharge via Timercontrol (timer désactivé)
	if prescaler {
		ctrl |= 0x04
	}
	g.Write8(0xE7C5, ctrl)
	g.Write8(0xE7C5, ctrl&^0x01) // bit0=0 → timer activé
}

func TestTimerTimeoutAssertsIRQ(t *testing.T) {
	g := newGA()
	var irq machine.IRQLines
	armTimer(g, 8, false) // timer6846 = 8<<3 = 64 ; countdown par c<<3
	if irq.Pending() {
		t.Fatal("IRQ assertée avant tout timeout")
	}
	// 4 instructions de 2 cycles : 64 - 4*(2<<3) = 64 - 64 = 0 → timeout.
	for i := 0; i < 4; i++ {
		g.OnInstructionCycles(2, &irq)
	}
	if !irq.IsAsserted(machine.IRQTimer) {
		t.Error("IRQTimer non assertée après timeout")
	}
	if !irq.Pending() {
		t.Error("IRQ non pending après timeout")
	}
}

func TestTimerIRQPulseDuration(t *testing.T) {
	g := newGA()
	var irq machine.IRQLines
	armTimer(g, 8, false)
	for i := 0; i < 4; i++ {
		g.OnInstructionCycles(2, &irq)
	}
	if !irq.IsAsserted(machine.IRQTimer) {
		t.Fatal("préparation : pas d'IRQ après timeout")
	}
	// Désactiver le timer pour isoler la durée du signal (impulse = 100 cycles).
	g.Write8(0xE7C5, 0x01)
	g.OnInstructionCycles(50, &irq) // 100 → 50 : encore actif
	if !irq.IsAsserted(machine.IRQTimer) {
		t.Error("IRQ relâchée trop tôt (50 < 100 cycles)")
	}
	g.OnInstructionCycles(60, &irq) // 50 → -10 : expiré
	if irq.IsAsserted(machine.IRQTimer) {
		t.Error("IRQ timer non relâchée après expiration du signal")
	}
}

func TestIRQClearConditional(t *testing.T) {
	g := newGA()
	var irq machine.IRQLines
	g.TriggerKeyboardIRQ() // signal clavier long (500000 cycles)
	armTimer(g, 8, false)
	for i := 0; i < 4; i++ {
		g.OnInstructionCycles(2, &irq)
	}
	if !irq.IsAsserted(machine.IRQTimer) || !irq.IsAsserted(machine.IRQKeyboard) {
		t.Fatal("les deux sources (timer + clavier) devraient être assertées")
	}
	// Désactiver le timer et laisser expirer SON signal (100), pas le clavier.
	g.Write8(0xE7C5, 0x01)
	g.OnInstructionCycles(200, &irq)
	if irq.IsAsserted(machine.IRQTimer) {
		t.Error("IRQ timer devrait être relâchée (signal expiré)")
	}
	if !irq.IsAsserted(machine.IRQKeyboard) {
		t.Error("IRQ clavier devrait rester assertée (signal long)")
	}
	if !irq.Pending() {
		t.Error("IRQ globale devrait rester pending (source clavier active)")
	}
}

func TestTimerPrescaler(t *testing.T) {
	// Avec prescaler (e7c5 bit2), le countdown est par c (et non c<<3) : 8× plus
	// lent. Après 4×2 cycles, le timer (64) est loin du timeout.
	g := newGA()
	var irq machine.IRQLines
	armTimer(g, 8, true)
	for i := 0; i < 4; i++ {
		g.OnInstructionCycles(2, &irq)
	}
	if irq.IsAsserted(machine.IRQTimer) {
		t.Error("avec prescaler, pas de timeout attendu après 4×2 cycles")
	}
	// Contraste : sans prescaler, les mêmes 4×2 cycles déclenchent le timeout.
	g2 := newGA()
	var irq2 machine.IRQLines
	armTimer(g2, 8, false)
	for i := 0; i < 4; i++ {
		g2.OnInstructionCycles(2, &irq2)
	}
	if !irq2.IsAsserted(machine.IRQTimer) {
		t.Error("sans prescaler, timeout attendu après 4×2 cycles")
	}
}

func TestTimerLatchReadWrite(t *testing.T) {
	g := newGA()
	g.Write8(0xE7C6, 0x12) // latch MSB
	g.Write8(0xE7C7, 0x34) // latch LSB → latch = 0x1234
	g.Write8(0xE7C5, 0x01) // recharge : timer6846 = 0x1234<<3 = 0x91A0
	if v := g.Read8(0xE7C6); v != 0x12 {
		t.Errorf("e7c6 (timer MSB) = 0x%02X, want 0x12", v)
	}
	if v, want := g.Read8(0xE7C7), byte(0x91A0>>3&0xff); v != want {
		t.Errorf("e7c7 (timer LSB) = 0x%02X, want 0x%02X", v, want)
	}
}

func TestTimerDisabledNoIRQ(t *testing.T) {
	g := newGA()
	var irq machine.IRQLines
	g.Write8(0xE7C7, 0x08)
	g.Write8(0xE7C5, 0x01) // bit0=1 → timer désactivé (recharge à 64, pas de countdown)
	for i := 0; i < 100; i++ {
		g.OnInstructionCycles(10, &irq)
	}
	if irq.IsAsserted(machine.IRQTimer) {
		t.Error("timer désactivé ne doit jamais déclencher d'IRQ")
	}
}

func TestKeyboardIRQAcknowledge(t *testing.T) {
	g := newGA()
	var irq machine.IRQLines
	g.TriggerKeyboardIRQ()
	g.OnInstructionCycles(10, &irq)
	if !irq.IsAsserted(machine.IRQKeyboard) {
		t.Fatal("IRQ clavier devrait être assertée après TriggerKeyboardIRQ")
	}
	// Acquittement firmware : écriture e7c3 avec le bit p5 (0x20) effacé.
	g.Write8(0xE7C3, 0x00)
	g.OnInstructionCycles(1, &irq)
	if irq.IsAsserted(machine.IRQKeyboard) {
		t.Error("IRQ clavier non relâchée après acquittement e7c3 (bit 0x20 effacé)")
	}
}

func TestCSRCompositeRead(t *testing.T) {
	g := newGA()
	var irq machine.IRQLines
	// Sans IRQ : e7c0 lit 0.
	if v := g.Read8(0xE7C0); v != 0 {
		t.Errorf("e7c0 sans IRQ = 0x%02X, want 0x00", v)
	}
	// Après timeout timer : bit0 (timer) + bit7 (composite) armés.
	armTimer(g, 8, false)
	for i := 0; i < 4; i++ {
		g.OnInstructionCycles(2, &irq)
	}
	v := g.Read8(0xE7C0)
	if v&0x01 == 0 || v&0x80 == 0 {
		t.Errorf("e7c0 après timeout = 0x%02X, want bits 0 et 7 armés", v)
	}
}
