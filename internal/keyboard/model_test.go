package keyboard

import "testing"

func TestMO5Model(t *testing.T) {
	m := MO5Model()
	if m.KeyCount != 58 {
		t.Errorf("KeyCount = %d, want 58", m.KeyCount)
	}
	if m.ShiftKey != Mo5KeyShift || m.CNTKey != Mo5KeyCNT || m.ENTKey != Mo5KeyENT {
		t.Errorf("indices modificateurs = (shift %#x, cnt %#x, ent %#x)", m.ShiftKey, m.CNTKey, m.ENTKey)
	}
	if !m.IsModifier(m.ShiftKey) || !m.IsModifier(m.CNTKey) || !m.IsModifier(m.ACCKey) {
		t.Error("IsModifier : Shift/CNT/ACC devraient être des modificateurs")
	}
	if m.IsModifier(0x00) {
		t.Error("IsModifier : une touche-caractère ne doit pas être un modificateur")
	}
	// CharToKey : minuscule/majuscule sans shift, caractère shifté avec shift.
	if k, shift, ok := m.CharToKey('A'); !ok || k != 0x2D || shift {
		t.Errorf("CharToKey('A') = (%#x,%v,%v), want (0x2D,false,true)", k, shift, ok)
	}
	if k, shift, ok := m.CharToKey('!'); !ok || k != 0x2F || !shift {
		t.Errorf("CharToKey('!') = (%#x,%v,%v), want (0x2F,true,true)", k, shift, ok)
	}
	if _, _, ok := m.CharToKey('€'); ok {
		t.Error("CharToKey('€') devrait échouer (pas d'équivalent MO5)")
	}
}
