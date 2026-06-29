package gatearray

import "testing"

func TestKeyboardDefUsesInjectedBoundariesAndModifiers(t *testing.T) {
	var calls []struct {
		key   int
		shift bool
		ctrl  bool
	}
	def := keyboardDef{
		characterMax: 0x50, // > borne TO8D 0x4f, mais CAPSLOCK TO8D historique
		capsLockKey:  0x53, // différent de CAPSLOCK TO8D 0x50
		shiftKeys:    []int{0x10},
		ctrlKey:      0x11,
		handlePress: func(g *GateArray, key int, shiftPressed bool, ctrlPressed bool) {
			calls = append(calls, struct {
				key   int
				shift bool
				ctrl  bool
			}{key: key, shift: shiftPressed, ctrl: ctrlPressed})
			g.port[0x08] |= 0x01
		},
	}
	g := newWithKeyboard(nil, nil, def)

	g.SetKey(0x10, true) // fake SHIFT, volontairement hors indices modificateurs TO8D
	g.SetKey(0x11, true) // fake CTRL, volontairement hors indices modificateurs TO8D
	calls = nil
	g.capslock = false
	g.SetKey(0x50, true) // prouve que 0x50 vient de la def et n'est pas CAPSLOCK TO8D

	if len(calls) != 1 {
		t.Fatalf("handlePress calls = %d, want 1", len(calls))
	}
	if calls[0].key != 0x50 || !calls[0].shift || !calls[0].ctrl {
		t.Fatalf("handlePress = %+v, want key=0x50 shift=true ctrl=true", calls[0])
	}
	if g.capslock {
		t.Fatal("fake character key 0x50 should not toggle capslock")
	}
	if g.port[0x08]&0x01 == 0 {
		t.Fatal("fake handlePress should set E7C8 bit0")
	}

	g.SetKey(0x10, false)
	g.SetKey(0x11, false)
	g.SetKey(0x50, false)
	if g.port[0x08]&0x01 != 0 {
		t.Fatal("release should clear E7C8 once all fake character keys are released")
	}

	g.capslock = false
	g.SetKey(0x53, true)
	if !g.capslock {
		t.Fatal("fake capsLockKey should toggle capslock")
	}
}

func TestKeyboardDefRejectsOutOfRangeKey(t *testing.T) {
	var calls int
	def := keyboardDef{
		characterMax: 0x50,
		capsLockKey:  0x53,
		handlePress: func(g *GateArray, key int, shiftPressed bool, ctrlPressed bool) {
			calls++
		},
	}
	g := newWithKeyboard(nil, nil, def)

	g.SetKey(-1, true)
	g.SetKey(len(g.touche), true)
	if calls != 0 {
		t.Fatalf("out-of-range key called handlePress %d times, want 0", calls)
	}
}
