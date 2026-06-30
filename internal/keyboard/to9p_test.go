package keyboard

import "testing"

func TestTO9PModelStructure(t *testing.T) {
	m := TO9PModel()
	if m == TO8DModel() {
		t.Fatal("TO9PModel must be a distinct singleton from TO8DModel")
	}
	if m.KeyCount != 84 {
		t.Errorf("KeyCount = %d, want 84", m.KeyCount)
	}
	if m.ShiftKey != 0x51 || m.CNTKey != 0x53 || m.ACCKey != 0x14 || m.ENTKey != 0x46 {
		t.Errorf("modifiers = shift 0x%02X / cnt 0x%02X / acc 0x%02X / ent 0x%02X, want 0x51/0x53/0x14/0x46",
			m.ShiftKey, m.CNTKey, m.ACCKey, m.ENTKey)
	}
}

func TestTO9PModelCharToKeyAnchoredToDCTO9PLabels(t *testing.T) {
	m := TO9PModel()
	cases := []struct {
		r     rune
		key   int
		shift bool
		ref   string
	}{
		{'y', 0x02, false, "dcto9pkeyb.h to9pkey[0x02] label Y"},
		{'Y', 0x02, false, "DCTO9P capslock/shift turns key 0x02 into ASCII 0x59"},
		{'_', 0x01, false, "dcto9pemulation.c to9key[0x01] = 0x5f"},
		{'6', 0x01, true, "dcto9pemulation.c to9key[0x51] = 0x36"},
		{' ', 0x34, false, "dcto9pkeyb.h to9pkey[0x34] label ESPACE"},
		{'\n', 0x46, false, "dcto9pkeyb.h to9pkey[0x46] label ENT"},
		{'\r', 0x46, false, "CRLF normalized to ENT"},
	}
	for _, c := range cases {
		k, shift, ok := m.CharToKey(c.r)
		if !ok || k != c.key || shift != c.shift {
			t.Errorf("CharToKey(%q) = (0x%02X, shift=%v, ok=%v), want (0x%02X, %v, true) -- %s",
				c.r, k, shift, ok, c.key, c.shift, c.ref)
		}
	}
}

func TestTO9PModelModifierKeys(t *testing.T) {
	got := TO9PModel().ModifierKeys()
	want := []int{0x51, 0x53, 0x14}
	if len(got) != len(want) {
		t.Fatalf("ModifierKeys TO9+ = %v (len=%d), want %v", got, len(got), want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("ModifierKeys TO9+ [%d] = 0x%02X, want 0x%02X", i, got[i], want[i])
		}
	}
}
