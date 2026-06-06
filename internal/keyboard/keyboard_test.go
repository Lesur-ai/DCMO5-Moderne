package keyboard

// keyboard_test.go — traduction caractère→MO5 et injecteur (purs, headless).

import "testing"

func TestCharToMO5Key_Digits(t *testing.T) {
	for _, c := range []struct {
		r   rune
		key int
	}{{'1', 0x2F}, {'0', 0x1E}, {'2', 0x27}} {
		key, shift, ok := CharToMO5Key(c.r)
		if !ok || key != c.key || shift {
			t.Errorf("CharToMO5Key(%q) = (0x%02X,%v,%v), want (0x%02X,false,true)", c.r, key, shift, ok, c.key)
		}
	}
}

// TestCharToMO5Key_ShiftedSymbols : le cas qui motivait la saisie par caractère.
func TestCharToMO5Key_ShiftedSymbols(t *testing.T) {
	for _, c := range []struct {
		r   rune
		key int
	}{{'"', 0x27}, {'?', 0x24}, {':', 0x2C}, {'!', 0x2F}, {'%', 0x0F}, {';', 0x2E}} {
		key, shift, ok := CharToMO5Key(c.r)
		if !ok || key != c.key || !shift {
			t.Errorf("CharToMO5Key(%q) = (0x%02X,%v,%v), want (0x%02X,true,true)", c.r, key, shift, ok, c.key)
		}
	}
}

func TestCharToMO5Key_Letters(t *testing.T) {
	for _, r := range []rune{'a', 'A'} {
		key, shift, ok := CharToMO5Key(r)
		if !ok || key != 0x2D || shift {
			t.Errorf("CharToMO5Key(%q) = (0x%02X,%v,%v), want (0x2D,false,true)", r, key, shift, ok)
		}
	}
}

func TestCharToMO5Key_NewlineIsENT(t *testing.T) {
	for _, r := range []rune{'\n', '\r'} {
		key, shift, ok := CharToMO5Key(r)
		if !ok || key != 0x34 || shift {
			t.Errorf("CharToMO5Key(%q) = (0x%02X,%v,%v), want ENT 0x34", r, key, shift, ok)
		}
	}
}

func TestCharToMO5Key_Unknown(t *testing.T) {
	if _, _, ok := CharToMO5Key('€'); ok {
		t.Error("CharToMO5Key('€') devrait retourner ok=false")
	}
}

func TestInjector_HoldThenGap(t *testing.T) {
	ki := NewInjector(2, 1)
	ki.Enqueue('a') // 0x2D
	if got := ki.Tick(); len(got) != 1 || got[0] != 0x2D {
		t.Fatalf("frame 1: got %v, want [0x2D]", got)
	}
	if got := ki.Tick(); len(got) != 1 || got[0] != 0x2D {
		t.Fatalf("frame 2: got %v, want [0x2D]", got)
	}
	if got := ki.Tick(); got != nil {
		t.Fatalf("frame 3 (gap): got %v, want nil", got)
	}
	if got := ki.Tick(); got != nil {
		t.Fatalf("frame 4 (idle): got %v, want nil", got)
	}
}

func TestInjector_ShiftEmitted(t *testing.T) {
	ki := NewInjector(1, 1)
	ki.Enqueue('"') // 0x27 + SHIFT
	got := ki.Tick()
	hasKey, hasShift := false, false
	for _, k := range got {
		if k == 0x27 {
			hasKey = true
		}
		if k == Mo5KeyShift {
			hasShift = true
		}
	}
	if !hasKey || !hasShift {
		t.Errorf("frappe '\"': got %v, want [0x27, 0x%02X]", got, Mo5KeyShift)
	}
}

// TestInjector_EnqueueString vérifie l'expansion d'une séquence, dont \n→ENT et
// la normalisation CRLF (un seul ENT).
func TestInjector_EnqueueString(t *testing.T) {
	ki := NewInjector(1, 1)
	ki.EnqueueString("A\r\nB\nC")
	want := []int{0x2D, 0x34, 0x22, 0x34, 0x32} // A, ENT, B, ENT, C
	if ki.Pending() != len(want) {
		t.Fatalf("Pending = %d, want %d", ki.Pending(), len(want))
	}
	if len(ki.queue) != len(want) {
		t.Fatalf("queue len=%d, want %d", len(ki.queue), len(want))
	}
	for i, w := range want {
		if ki.queue[i].key != w {
			t.Errorf("frappe %d: key=0x%02X, want 0x%02X", i, ki.queue[i].key, w)
		}
	}
}

func TestInjector_QueueBounded(t *testing.T) {
	ki := NewInjector(DefaultHoldFrames, DefaultGapFrames)
	ki.queueMax = 4
	for i := 0; i < 100; i++ {
		ki.Enqueue('a')
	}
	if len(ki.queue) > ki.queueMax {
		t.Errorf("file non bornée: len=%d, want <= %d", len(ki.queue), ki.queueMax)
	}
}

func TestInjector_IdleReturnsNil(t *testing.T) {
	ki := NewInjector(DefaultHoldFrames, DefaultGapFrames)
	if got := ki.Tick(); got != nil {
		t.Errorf("injecteur vide: Tick() = %v, want nil", got)
	}
	if ki.Pending() != 0 {
		t.Errorf("injecteur vide: Pending = %d, want 0", ki.Pending())
	}
}
