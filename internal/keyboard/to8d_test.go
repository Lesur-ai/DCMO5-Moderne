package keyboard

import "testing"

func TestTO8DModelStructure(t *testing.T) {
	m := TO8DModel()
	if m.KeyCount != 84 {
		t.Errorf("KeyCount = %d, want 84", m.KeyCount)
	}
	if m.ShiftKey != 0x51 || m.CNTKey != 0x53 || m.ACCKey != 0x14 || m.ENTKey != 0x46 {
		t.Errorf("modificateurs = shift 0x%02X / cnt 0x%02X / acc 0x%02X / ent 0x%02X, want 0x51/0x53/0x14/0x46",
			m.ShiftKey, m.CNTKey, m.ACCKey, m.ENTKey)
	}
}

func TestTO8DModelCharToKey(t *testing.T) {
	m := TO8DModel()
	cases := []struct {
		r     rune
		key   int
		shift bool
	}{
		{'a', 0x2a, false}, {'A', 0x2a, false}, // insensible à la casse (comme MO5)
		{'y', 0x02, false}, {'m', 0x4b, false}, {'p', 0x4a, false}, {'z', 0x22, false},
		{' ', 0x34, false},  // ESPACE
		{'\n', 0x46, false}, // ENT
		{'\r', 0x46, false}, // ENT (CRLF)
	}
	for _, c := range cases {
		k, shift, ok := m.CharToKey(c.r)
		if !ok || k != c.key || shift != c.shift {
			t.Errorf("CharToKey(%q) = (0x%02X,%v,%v), want (0x%02X,%v,true)", c.r, k, shift, ok, c.key, c.shift)
		}
	}
}

func TestTO8DModelCharToKeyUnmapped(t *testing.T) {
	m := TO8DModel()
	// Chiffres/symboles : table déférée au #118 ; '€' : aucun équivalent TO8D.
	for _, r := range []rune{'6', '€', '_'} {
		if _, _, ok := m.CharToKey(r); ok {
			t.Errorf("CharToKey(%q) devrait échouer (table chiffres/symboles déférée au #118)", r)
		}
	}
}

func TestTO8DModelIsModifier(t *testing.T) {
	m := TO8DModel()
	for _, k := range []int{0x51, 0x53, 0x14} { // SHIFT, CNT, ACC
		if !m.IsModifier(k) {
			t.Errorf("IsModifier(0x%02X) = false, want true", k)
		}
	}
	if m.IsModifier(0x02) { // touche-lettre 'Y' : pas un modificateur
		t.Error("IsModifier(0x02) = true, want false")
	}
}
