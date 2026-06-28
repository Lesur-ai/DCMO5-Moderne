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

// TestMO5Model_ModifierKeys vérifie que ModifierKeys() retourne EXACTEMENT
// [SHIFT, CNT, ACC] pour MO5, dans cet ordre. C'est l'invariant que Host.tick
// consomme pour appliquer les modifs avant les caractères (Inc Kc).
func TestMO5Model_ModifierKeys(t *testing.T) {
	got := MO5Model().ModifierKeys()
	want := []int{Mo5KeyShift, Mo5KeyCNT, mo5KeyACC}
	if len(got) != len(want) {
		t.Fatalf("ModifierKeys MO5 = %v (len=%d), want %v (len=%d)", got, len(got), want, len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("ModifierKeys MO5 [%d] = %#x, want %#x", i, got[i], want[i])
		}
	}
}

// TestTO8DModel_ModifierKeys : même invariant que MO5 mais avec les scancodes
// TO8D (lus depuis dcto8dkeyb.c). Crucial pour que le gate-array latch les
// modifs avant le caractère ; sans ce test, un swap accidentel des indices
// passerait à travers (la classe Model est neutre vis-à-vis des valeurs).
func TestTO8DModel_ModifierKeys(t *testing.T) {
	got := TO8DModel().ModifierKeys()
	want := []int{to8dKeyShift, to8dKeyCNT, to8dKeyACC}
	if len(got) != len(want) {
		t.Fatalf("ModifierKeys TO8D = %v (len=%d), want %v (len=%d)", got, len(got), want, len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("ModifierKeys TO8D [%d] = %#x, want %#x", i, got[i], want[i])
		}
	}
}

// TestModel_ModifierKeys_OmitsNegativeACC : une machine qui n'a pas de touche
// ACC (ACCKey < 0) doit voir ModifierKeys() retourner [SHIFT, CNT] uniquement —
// pas -1 ni un indice douteux. Garde-fou de robustesse : Host.tick fait confiance
// au contenu du slice et l'utilise comme indice de in.Keys ; un -1 indexerait
// hors borne. ACCKey < 0 est documenté dans Model (« -1 si pas d'ACC »).
func TestModel_ModifierKeys_OmitsNegativeACC(t *testing.T) {
	m := &Model{
		KeyCount: 32,
		ShiftKey: 0x10,
		CNTKey:   0x11,
		ACCKey:   -1,
		ENTKey:   0x12,
	}
	got := m.ModifierKeys()
	want := []int{0x10, 0x11}
	if len(got) != len(want) {
		t.Fatalf("ModifierKeys avec ACCKey=-1 = %v (len=%d), want %v (len=%d)", got, len(got), want, len(want))
	}
	for i, k := range got {
		if k < 0 {
			t.Errorf("ModifierKeys [%d] = %d (négatif) — ACCKey=-1 ne doit JAMAIS figurer dans la liste", i, k)
		}
		if k != want[i] {
			t.Errorf("ModifierKeys [%d] = %#x, want %#x", i, k, want[i])
		}
	}
}
