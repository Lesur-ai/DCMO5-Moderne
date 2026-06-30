package gatearray_test

import (
	"testing"

	"github.com/Lesur-ai/dcmo5/internal/machine"
	"github.com/Lesur-ai/dcmo5/internal/machine/gatearray"
)

func newTO9PGA() *gatearray.GateArray {
	return gatearray.NewTO9P(romMonPattern(), romBasicPattern())
}

func TestTO9PKeyPressPublishesASCIIAndDoesNotUseTO8DMonitorPath(t *testing.T) {
	g := newTO9PGA()
	var irq machine.IRQLines
	selectSysBank1(t, g)
	before := g.Read8(0xF0F8)

	g.SetKey(keyY, true) // capslock true at hard reset -> shifted ASCII 'Y'

	if v := g.Read8(0xE7DE); v != 0x01 {
		t.Fatalf("TO9+ pending flag E7DE = 0x%02X, want 0x01", v)
	}
	if v := g.Read8(0xE7DF); v != 0x59 {
		t.Fatalf("TO9+ ASCII E7DF = 0x%02X, want 0x59 ('Y')", v)
	}
	if v := g.Read8(0xE7DE); v != 0x00 {
		t.Fatalf("TO9+ pending flag E7DE after E7DF read = 0x%02X, want 0x00", v)
	}
	if after := g.Read8(0xF0F8); after != before {
		t.Fatalf("TO9+ must not mutate TO8D monitor scancode path: F0F8 before=0x%02X after=0x%02X", before, after)
	}
	g.OnInstructionCycles(1, &irq)
	if irq.IsAsserted(machine.IRQKeyboard) {
		t.Fatal("TO9+ keyboard path must not raise TO8D keyboard IRQ")
	}
}

func TestTO9PAndTO8DKeyboardPathsDivergeObservably(t *testing.T) {
	to8 := newGA()
	to9 := newTO9PGA()

	selectSysBank1(t, to8)
	selectSysBank1(t, to9)

	to8.SetKey(keyY, true)
	to9.SetKey(keyY, true)

	if v := to8.Read8(0xF0F8); v != 0x82 {
		t.Fatalf("TO8D keyY monitor scancode = 0x%02X, want 0x82", v)
	}
	if v := to9.Read8(0xE7DF); v != 0x59 {
		t.Fatalf("TO9+ keyY ASCII = 0x%02X, want 0x59", v)
	}
	if v := to9.Read8(0xF0F8); v != 0x60 {
		t.Fatalf("TO9+ monitor path mutated to 0x%02X, want untouched 0x60", v)
	}
}

func TestTO9PShiftChangesPublishedASCII(t *testing.T) {
	g := newTO9PGA()
	g.SetKey(keyCapsLock, true) // capslock off to isolate SHIFT.
	g.SetKey(keyUnderscore6, true)
	if v := g.Read8(0xE7DF); v != 0x5f {
		t.Fatalf("TO9+ key '_ 6' without shift = 0x%02X, want 0x5F ('_')", v)
	}

	for _, shift := range []int{keyShiftL, keyShiftR} {
		g = newTO9PGA()
		g.SetKey(keyCapsLock, true) // capslock off to isolate SHIFT.
		g.SetKey(shift, true)
		g.SetKey(keyUnderscore6, true)
		if v := g.Read8(0xE7DF); v != 0x36 {
			t.Fatalf("TO9+ key '_ 6' with shift 0x%02X = 0x%02X, want 0x36 ('6')", shift, v)
		}
	}
}

func TestTO9PCtrlTransformsASCII(t *testing.T) {
	g := newTO9PGA()
	g.SetKey(keyCapsLock, true) // capslock off: keyY publishes lowercase 'y' first.
	g.SetKey(keyCNT, true)
	g.SetKey(keyY, true)
	if v := g.Read8(0xE7DF); v != 0x19 {
		t.Fatalf("TO9+ CNT+y = 0x%02X, want 0x19", v)
	}

	g = newTO9PGA()
	g.SetKey(keyCNT, true)
	g.SetKey(keyUnderscore6, true)
	if v := g.Read8(0xE7DF); v != 0x1f {
		t.Fatalf("TO9+ CNT+_ = 0x%02X, want 0x1F", v)
	}
}

func TestTO9PModifierAloneDoesNotPublishASCII(t *testing.T) {
	for _, mod := range []int{keyShiftL, keyShiftR, keyCNT} {
		g := newTO9PGA()
		g.SetKey(mod, true)
		if v := g.Read8(0xE7DE); v != 0x00 {
			t.Errorf("TO9+ modifier 0x%02X alone E7DE = 0x%02X, want 0x00", mod, v)
		}
		if v := g.Read8(0xE7DF); v != 0x00 {
			t.Errorf("TO9+ modifier 0x%02X alone E7DF = 0x%02X, want 0x00", mod, v)
		}
	}
}
