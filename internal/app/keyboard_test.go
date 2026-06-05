package app

// keyboard_test.go — tests de la saisie caractère MO5 (logique pure, headless).

import "testing"

func TestCharToMO5Key_Digits(t *testing.T) {
	cases := []struct {
		r     rune
		key   int
		shift bool
	}{
		{'1', 0x2F, false},
		{'0', 0x1E, false},
		{'2', 0x27, false},
	}
	for _, c := range cases {
		key, shift, ok := CharToMO5Key(c.r)
		if !ok || key != c.key || shift != c.shift {
			t.Errorf("CharToMO5Key(%q) = (0x%02X,%v,%v), want (0x%02X,%v,true)",
				c.r, key, shift, ok, c.key, c.shift)
		}
	}
}

// TestCharToMO5Key_ShiftedSymbols vérifie le cas qui motivait P9.1 :
// pouvoir saisir les caractères obtenus avec Shift (« " », « ? », « : »…).
func TestCharToMO5Key_ShiftedSymbols(t *testing.T) {
	cases := []struct {
		r   rune
		key int
	}{
		{'"', 0x27}, // Shift+2
		{'?', 0x24}, // Shift+/
		{':', 0x2C}, // Shift+*
		{'!', 0x2F}, // Shift+1
		{'%', 0x0F}, // Shift+5
		{';', 0x2E}, // Shift++
	}
	for _, c := range cases {
		key, shift, ok := CharToMO5Key(c.r)
		if !ok || key != c.key || !shift {
			t.Errorf("CharToMO5Key(%q) = (0x%02X,%v,%v), want (0x%02X,true,true)",
				c.r, key, shift, ok, c.key)
		}
	}
}

func TestCharToMO5Key_Letters(t *testing.T) {
	// Majuscule et minuscule pointent vers la même touche, sans shift.
	for _, r := range []rune{'a', 'A'} {
		key, shift, ok := CharToMO5Key(r)
		if !ok || key != 0x2D || shift {
			t.Errorf("CharToMO5Key(%q) = (0x%02X,%v,%v), want (0x2D,false,true)", r, key, shift, ok)
		}
	}
}

func TestCharToMO5Key_Unknown(t *testing.T) {
	// Un caractère sans équivalent MO5 doit être rejeté proprement.
	if _, _, ok := CharToMO5Key('€'); ok {
		t.Error("CharToMO5Key('€') devrait retourner ok=false")
	}
}

// TestKeyInjector_HoldThenGap vérifie qu'une frappe est maintenue holdFrames
// puis relâchée gapFrames avant la frappe suivante.
func TestKeyInjector_HoldThenGap(t *testing.T) {
	ki := newKeyInjector(2, 1) // hold=2, gap=1
	ki.Enqueue('a')            // touche 0x2D, pas de shift

	// Frame 1 et 2 : touche pressée
	if got := ki.Tick(); len(got) != 1 || got[0] != 0x2D {
		t.Fatalf("frame 1: got %v, want [0x2D]", got)
	}
	if got := ki.Tick(); len(got) != 1 || got[0] != 0x2D {
		t.Fatalf("frame 2: got %v, want [0x2D]", got)
	}
	// Frame 3 : gap (rien pressé)
	if got := ki.Tick(); got != nil {
		t.Fatalf("frame 3 (gap): got %v, want nil", got)
	}
	// Frame 4 : repos
	if got := ki.Tick(); got != nil {
		t.Fatalf("frame 4 (idle): got %v, want nil", got)
	}
}

// TestKeyInjector_ShiftEmitted vérifie qu'un caractère shifté presse SHIFT.
func TestKeyInjector_ShiftEmitted(t *testing.T) {
	ki := newKeyInjector(1, 1)
	ki.Enqueue('"') // touche 0x27 + SHIFT

	got := ki.Tick()
	hasKey, hasShift := false, false
	for _, k := range got {
		if k == 0x27 {
			hasKey = true
		}
		if k == mo5KeyShift {
			hasShift = true
		}
	}
	if !hasKey || !hasShift {
		t.Errorf("frappe '\"': got %v, want [0x27, 0x%02X]", got, mo5KeyShift)
	}
}

// TestKeyInjector_Sequence vérifie que deux frappes se jouent en série.
func TestKeyInjector_Sequence(t *testing.T) {
	ki := newKeyInjector(1, 1)
	ki.Enqueue('a') // 0x2D
	ki.Enqueue('b') // 0x22

	if ki.Pending() != 2 {
		t.Fatalf("Pending = %d, want 2", ki.Pending())
	}
	if got := ki.Tick(); len(got) != 1 || got[0] != 0x2D { // 'a' hold
		t.Fatalf("frame 1: got %v, want [0x2D]", got)
	}
	if got := ki.Tick(); got != nil { // gap
		t.Fatalf("frame 2 (gap): got %v, want nil", got)
	}
	if got := ki.Tick(); len(got) != 1 || got[0] != 0x22 { // 'b' hold
		t.Fatalf("frame 3: got %v, want [0x22]", got)
	}
}

func TestKeyInjector_IdleReturnsNil(t *testing.T) {
	ki := newKeyInjector(defaultKeyHoldFrames, defaultKeyGapFrames)
	if got := ki.Tick(); got != nil {
		t.Errorf("injecteur vide: Tick() = %v, want nil", got)
	}
	if ki.Pending() != 0 {
		t.Errorf("injecteur vide: Pending = %d, want 0", ki.Pending())
	}
}

// TestKeyInjector_QueueBounded vérifie que la file ne croît pas sans limite :
// au-delà de queueMax, les frappes les plus anciennes sont abandonnées.
func TestKeyInjector_QueueBounded(t *testing.T) {
	ki := newKeyInjector(defaultKeyHoldFrames, defaultKeyGapFrames)
	ki.queueMax = 4
	// Enfiler 100 caractères valides : la file doit rester bornée.
	for i := 0; i < 100; i++ {
		ki.Enqueue('a')
	}
	if len(ki.queue) > ki.queueMax {
		t.Errorf("file non bornée: len=%d, want <= %d", len(ki.queue), ki.queueMax)
	}
}
