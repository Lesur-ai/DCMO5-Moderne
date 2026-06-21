package gatearray_test

// Tests du clavier TO8D (#116). Boîte noire : SetKey / Read8 / Write8 /
// OnInstructionCycles. Réf C : dcto8demulation.c TO8key (l.134-164).
//
// Conventions de fidélité vérifiées :
//   - touche[k] : 0x00 = enfoncée, 0x80 = relâchée ;
//   - scancode + bit SHIFT écrit à l'offset FIXE 0x30F8 (banque système 1),
//     indicateur CTRL en 0x3125 ;
//   - relecture via Read8(0xF0F8 / 0xF125) APRÈS sélection de la banque système 1
//     (E7C3 bit4). La banque 1 est pré-remplie 0x60 (romMonPattern) : un test qui
//     attend une valeur ≠ 0x60 prouve une écriture réelle (RED sans implémentation) ;
//   - capslock = true au hard reset → la 1re lettre prend déjà le bit 0x80 ;
//   - IRQ clavier (CP1) levée sur FRONT, observable après OnInstructionCycles ;
//   - acquittement par E7C3 bit 0x20 effacé (déjà posé au #114).

import (
	"testing"

	"github.com/Lesur-ai/dcmo5/internal/machine"
	"github.com/Lesur-ai/dcmo5/internal/machine/gatearray"
)

const (
	keyUnderscore6 = 0x01 // " _ 6" : touche NON-lettre (insensible au capslock)
	keyY           = 0x02 // "Y"    : lettre (sensible au capslock)
	keyACC         = 0x14 // "ACC"  : touche normale côté TO8key (pas un modificateur)
	keyCapsLock    = 0x50
	keyShiftL      = 0x51
	keyShiftR      = 0x52
	keyCNT         = 0x53
)

// selectSysBank1 sélectionne la banque système 1 (E7C3 bit4) afin de relire la RAM
// moniteur via Read8(0xF0F8 / 0xF125). À appeler AVANT la frappe : l'écriture E7C3
// avec le bit 0x20 effacé acquitte aussi l'IRQ clavier (sans effet si aucune frappe).
func selectSysBank1(t *testing.T, g *gatearray.GateArray) {
	t.Helper()
	g.Write8(0xE7C3, 0x10)
	if v := g.Read8(0xF0F8); v != 0x60 {
		t.Fatalf("préparation : banque système 1 non sélectionnée (Read8(0xF0F8)=0x%02X, want pré-fill 0x60)", v)
	}
}

func TestTO8DKeyPressWritesScancodeAndRaisesIRQ(t *testing.T) {
	g := newGA()
	var irq machine.IRQLines
	selectSysBank1(t, g)
	g.SetKey(keyUnderscore6, true) // NON-lettre : capslock sans effet → scancode brut
	if v := g.Read8(0xF0F8); v != keyUnderscore6 {
		t.Errorf("scancode (0x30F8) = 0x%02X, want 0x%02X", v, keyUnderscore6)
	}
	if v := g.Read8(0xE7C8); v&0x01 == 0 {
		t.Errorf("E7C8 = 0x%02X, bit0 (touche enfoncée) attendu", v)
	}
	g.OnInstructionCycles(1, &irq)
	if !irq.IsAsserted(machine.IRQKeyboard) {
		t.Error("IRQ clavier non assertée après appui")
	}
}

func TestTO8DCapsLockInitialUppercasesLetter(t *testing.T) {
	g := newGA()
	selectSysBank1(t, g)
	g.SetKey(keyY, true) // lettre, capslock = true au reset → bit 0x80
	if v := g.Read8(0xF0F8); v != keyY|0x80 {
		t.Errorf("lettre (capslock initial) = 0x%02X, want 0x%02X", v, keyY|0x80)
	}
}

func TestTO8DCapsLockOffLetterHasNoBit(t *testing.T) {
	g := newGA()
	selectSysBank1(t, g)
	g.SetKey(keyCapsLock, true) // 0x50 → bascule capslock (true → false)
	g.SetKey(keyY, true)
	if v := g.Read8(0xF0F8); v != keyY {
		t.Errorf("lettre (capslock off) = 0x%02X, want 0x%02X", v, keyY)
	}
}

func TestTO8DCapsLockNeverAffectsNonLetter(t *testing.T) {
	g := newGA()
	selectSysBank1(t, g)
	// capslock = true au reset ; 0x01 n'est pas une lettre → jamais de bit 0x80.
	g.SetKey(keyUnderscore6, true)
	if v := g.Read8(0xF0F8); v != keyUnderscore6 {
		t.Errorf("non-lettre (capslock on) = 0x%02X, want 0x%02X", v, keyUnderscore6)
	}
}

func TestTO8DShiftSetsBit80(t *testing.T) {
	for _, shift := range []int{keyShiftL, keyShiftR} {
		g := newGA()
		selectSysBank1(t, g)
		g.SetKey(keyCapsLock, true) // capslock off pour isoler l'effet du SHIFT
		g.SetKey(shift, true)       // modificateur AVANT le caractère (contrat d'ordre)
		g.SetKey(keyUnderscore6, true)
		if v := g.Read8(0xF0F8); v != keyUnderscore6|0x80 {
			t.Errorf("SHIFT 0x%02X + non-lettre = 0x%02X, want 0x%02X", shift, v, keyUnderscore6|0x80)
		}
	}
}

func TestTO8DCtrlFlag(t *testing.T) {
	// CNT maintenu puis lettre : 0x3125 = 1.
	g := newGA()
	selectSysBank1(t, g)
	g.SetKey(keyCNT, true)
	g.SetKey(keyUnderscore6, true)
	if v := g.Read8(0xF125); v != 1 {
		t.Errorf("CTRL (0x3125) avec CNT = 0x%02X, want 0x01", v)
	}
	// Sans CNT : 0x3125 = 0 (et non le pré-fill 0x60) → prouve l'écriture.
	g2 := newGA()
	selectSysBank1(t, g2)
	g2.SetKey(keyUnderscore6, true)
	if v := g2.Read8(0xF125); v != 0 {
		t.Errorf("CTRL (0x3125) sans CNT = 0x%02X, want 0x00", v)
	}
}

func TestTO8DModifierAloneNoScancodeNoIRQ(t *testing.T) {
	for _, mod := range []int{keyShiftL, keyShiftR, keyCNT} {
		g := newGA()
		var irq machine.IRQLines
		selectSysBank1(t, g)
		g.SetKey(mod, true)
		if v := g.Read8(0xF0F8); v != 0x60 {
			t.Errorf("modificateur 0x%02X seul : 0x30F8 = 0x%02X, want pré-fill 0x60 (aucune écriture)", mod, v)
		}
		g.OnInstructionCycles(1, &irq)
		if irq.IsAsserted(machine.IRQKeyboard) {
			t.Errorf("modificateur 0x%02X seul ne doit pas lever d'IRQ", mod)
		}
	}
}

func TestTO8DCapsLockKeyTogglesWithoutScancodeOrIRQ(t *testing.T) {
	g := newGA()
	var irq machine.IRQLines
	selectSysBank1(t, g)
	g.SetKey(keyCapsLock, true) // bascule, mais 0x50 > 0x4F → pas de scancode, pas d'IRQ
	if v := g.Read8(0xF0F8); v != 0x60 {
		t.Errorf("CAPSLOCK (0x50) : 0x30F8 = 0x%02X, want pré-fill 0x60", v)
	}
	g.OnInstructionCycles(1, &irq)
	if irq.IsAsserted(machine.IRQKeyboard) {
		t.Error("CAPSLOCK (0x50) ne doit pas lever d'IRQ")
	}
	// Effet observable du toggle : capslock true → false → lettre suivante sans 0x80.
	g.SetKey(keyY, true)
	if v := g.Read8(0xF0F8); v != keyY {
		t.Errorf("après bascule CAPSLOCK, lettre = 0x%02X, want 0x%02X (capslock off)", v, keyY)
	}
}

func TestTO8DAccIsNormalKey(t *testing.T) {
	g := newGA()
	var irq machine.IRQLines
	selectSysBank1(t, g)
	g.SetKey(keyACC, true) // 0x14 ≤ 0x4F → touche normale (réf C : aucun cas spécial)
	if v := g.Read8(0xF0F8); v != keyACC {
		t.Errorf("ACC (0x14) scancode = 0x%02X, want 0x%02X (touche normale)", v, keyACC)
	}
	g.OnInstructionCycles(1, &irq)
	if !irq.IsAsserted(machine.IRQKeyboard) {
		t.Error("ACC (0x14) doit lever l'IRQ (touche normale)")
	}
}

func TestTO8DKey4FIsNormalBoundary(t *testing.T) {
	// 0x4F (« > < ») est la DERNIÈRE touche alphanumérique (réf C : if(n > 0x4f) return).
	// Verrou anti-mutation : un fix fautif « if n >= 0x4f { return } » la traiterait à
	// tort comme un modificateur (sans scancode). 0x4F n'est pas une lettre → pas de 0x80.
	g := newGA()
	var irq machine.IRQLines
	selectSysBank1(t, g)
	g.SetKey(0x4F, true)
	if v := g.Read8(0xF0F8); v != 0x4F {
		t.Errorf("scancode 0x4F = 0x%02X, want 0x4F (touche normale, borne haute)", v)
	}
	if v := g.Read8(0xE7C8); v&0x01 == 0 {
		t.Errorf("E7C8 bit0 attendu pour 0x4F (0x%02X)", v)
	}
	g.OnInstructionCycles(1, &irq)
	if !irq.IsAsserted(machine.IRQKeyboard) {
		t.Error("0x4F doit lever l'IRQ (touche normale)")
	}
	g.SetKey(0x4F, false) // relâchement → libération de E7C8
	if v := g.Read8(0xE7C8); v&0x01 != 0 {
		t.Errorf("E7C8 bit0 doit s'effacer après relâchement de 0x4F (0x%02X)", v)
	}
}

func TestTO8DReleaseAllClearsE7C8(t *testing.T) {
	g := newGA()
	g.SetKey(keyUnderscore6, true)
	g.SetKey(keyY, true)
	if v := g.Read8(0xE7C8); v&0x01 == 0 {
		t.Fatalf("E7C8 bit0 attendu avec deux touches enfoncées (0x%02X)", v)
	}
	g.SetKey(keyUnderscore6, false) // une touche reste (keyY) → bit0 maintenu
	if v := g.Read8(0xE7C8); v&0x01 == 0 {
		t.Errorf("E7C8 bit0 ne doit pas s'effacer tant qu'une touche reste enfoncée (0x%02X)", v)
	}
	g.SetKey(keyY, false) // toutes relâchées → bit0 = 0
	if v := g.Read8(0xE7C8); v&0x01 != 0 {
		t.Errorf("E7C8 bit0 doit s'effacer quand toutes les touches sont relâchées (0x%02X)", v)
	}
}

func TestTO8DKeyboardIRQFrontOnly(t *testing.T) {
	g := newGA()
	var irq machine.IRQLines
	g.SetKey(keyUnderscore6, true)
	g.OnInstructionCycles(1, &irq)
	if !irq.IsAsserted(machine.IRQKeyboard) {
		t.Fatal("IRQ attendue après le premier front")
	}
	g.Write8(0xE7C3, 0x00) // acquittement (bit 0x20 effacé)
	g.OnInstructionCycles(1, &irq)
	if irq.IsAsserted(machine.IRQKeyboard) {
		t.Fatal("IRQ devrait être relâchée après acquittement")
	}
	g.SetKey(keyUnderscore6, true) // ré-application idempotente (pas de front) → pas de nouvelle IRQ
	g.OnInstructionCycles(1, &irq)
	if irq.IsAsserted(machine.IRQKeyboard) {
		t.Error("aucune nouvelle IRQ sans transition (front) — ré-appui idempotent")
	}
}

func TestTO8DReleaseThenRepressReraisesIRQ(t *testing.T) {
	g := newGA()
	var irq machine.IRQLines
	g.SetKey(keyUnderscore6, true)
	g.OnInstructionCycles(1, &irq)
	g.Write8(0xE7C3, 0x00) // ack
	g.OnInstructionCycles(1, &irq)
	if irq.IsAsserted(machine.IRQKeyboard) {
		t.Fatal("préparation : IRQ devrait être relâchée après ack")
	}
	g.SetKey(keyUnderscore6, false) // relâche (front)
	g.SetKey(keyUnderscore6, true)  // ré-appui (front) → nouvelle IRQ
	g.OnInstructionCycles(1, &irq)
	if !irq.IsAsserted(machine.IRQKeyboard) {
		t.Error("un nouveau front (relâche puis ré-appui) doit relever l'IRQ")
	}
}

func TestTO8DAcknowledgeReleasesIRQ(t *testing.T) {
	g := newGA()
	var irq machine.IRQLines
	g.SetKey(keyUnderscore6, true)
	g.OnInstructionCycles(1, &irq)
	if !irq.IsAsserted(machine.IRQKeyboard) {
		t.Fatal("IRQ attendue après appui")
	}
	g.Write8(0xE7C3, 0x00) // acquittement firmware (bit 0x20 effacé)
	g.OnInstructionCycles(1, &irq)
	if irq.IsAsserted(machine.IRQKeyboard) {
		t.Error("IRQ non relâchée après acquittement E7C3")
	}
}

func TestTO8DScancodeNotClearedOnRelease(t *testing.T) {
	g := newGA()
	selectSysBank1(t, g)
	g.SetKey(keyUnderscore6, true)
	if v := g.Read8(0xF0F8); v != keyUnderscore6 {
		t.Fatalf("scancode après appui = 0x%02X, want 0x%02X", v, keyUnderscore6)
	}
	g.SetKey(keyUnderscore6, false) // relâche : ne réécrit pas 0x30F8
	if v := g.Read8(0xF0F8); v != keyUnderscore6 {
		t.Errorf("scancode après relâche = 0x%02X, want 0x%02X (inchangé)", v, keyUnderscore6)
	}
}
