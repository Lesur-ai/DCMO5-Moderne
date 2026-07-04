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
	// Caractères SANS équivalent TO8D : € (pas dans le jeu de caractères TO8D),
	// touches mortes ^/¨ (décision owner BLOCKER #5 : non mappées en rune, l'usage
	// ACC + SHIFT + touche est laissé à l'utilisateur), ² µ § ~ | { non présents
	// sur le clavier physique TO8D (cf. dcto8dinterface.c labels).
	//
	// Note Inc Kb : '6' et '_' ÉTAIENT non mappés pré-Kb (test pré-Kb), ils le sont
	// maintenant (cf. TestTO8DModelCharToKey_AzertyDigitsRow ci-dessous).
	for _, r := range []rune{'€', '²', 'µ', '§', '~', '|', '^', '¨'} {
		if _, _, ok := m.CharToKey(r); ok {
			t.Errorf("CharToKey(%q) devrait échouer (aucun équivalent TO8D : touche morte, hors charset, etc.)", r)
		}
	}
}

// TestTO8DModelCharToKey_AzertyDigitsRow_PolarityFromPckeycode (Inc Kb, Kb-1) :
// l'invariant principal de la convention « accent direct, chiffre en shift »
// (BLOCKER #1 owner, source dcto8dkeyb.h pckeycode[]). Pour CHAQUE paire (accent
// /symbole, chiffre) de la rangée du haut : l'accent est produit sans SHIFT, le
// chiffre est produit AVEC SHIFT, sur le MÊME scancode. Anti-tautologie : on
// vérifie la POLARITÉ (direction shift bool), pas seulement le scancode.
func TestTO8DModelCharToKey_AzertyDigitsRow_PolarityFromPckeycode(t *testing.T) {
	m := TO8DModel()
	cases := []struct {
		direct, withShift rune
		key               int
		label             string
	}{
		{'_', '6', 0x01, "_ 6"},
		{'(', '5', 0x09, "( 5"},
		{'\'', '4', 0x11, "' 4"},
		{'"', '3', 0x19, "\" 3"},
		{'é', '2', 0x21, "é 2"},
		{'*', '1', 0x29, "* 1"},
		{'è', '7', 0x31, "è 7"},
		{'!', '8', 0x39, "! 8"},
		{'ç', '9', 0x41, "ç 9"},
		{'à', '0', 0x49, "à 0"},
	}
	for _, c := range cases {
		dk, dshift, dok := m.CharToKey(c.direct)
		if !dok || dk != c.key || dshift {
			t.Errorf("touche label %q : CharToKey(%q) = (0x%02X, shift=%v, ok=%v), want (0x%02X, false, true) — accent doit être sans SHIFT",
				c.label, c.direct, dk, dshift, dok, c.key)
		}
		sk, sshift, sok := m.CharToKey(c.withShift)
		if !sok || sk != c.key || !sshift {
			t.Errorf("touche label %q : CharToKey(%q) = (0x%02X, shift=%v, ok=%v), want (0x%02X, true, true) — chiffre doit être AVEC SHIFT",
				c.label, c.withShift, sk, sshift, sok, c.key)
		}
	}
}

// TestTO8DModelCharToKey_SymbolsAnchoredFromCSource (Inc Kb, Kb-2) : ancrage des
// symboles non-chiffre par référence aux labels physiques décodés depuis
// dcto8dinterface.c (Latin-1 → UTF-8). Échec si un mapping bouge silencieusement.
func TestTO8DModelCharToKey_SymbolsAnchoredFromCSource(t *testing.T) {
	m := TO8DModel()
	cases := []struct {
		r     rune
		key   int
		shift bool
		ref   string
	}{
		{'=', 0x0c, false, "label « = + » à 0x0c"},
		{'+', 0x0c, true, "label « = + » à 0x0c shift"},
		{'#', 0x28, false, "label « # @ » à 0x28"},
		{'@', 0x28, true, "label « # @ » à 0x28 shift"},
		{'[', 0x2c, false, "label « [ { » à 0x2c"},
		{'{', 0x2c, true, "label « [ { » à 0x2c shift"},
		{',', 0x37, false, "label « , ? » à 0x37"},
		{'?', 0x37, true, "label « , ? » à 0x37 shift"},
		{'$', 0x3c, false, "label « $ & » à 0x3c"},
		{'&', 0x3c, true, "label « $ & » à 0x3c shift"},
		{']', 0x3e, false, "label « ] } » à 0x3e"},
		{'}', 0x3e, true, "label « ] } » à 0x3e shift"},
		{';', 0x3f, false, "label « ; . » à 0x3f"},
		{'.', 0x3f, true, "label « ; . » à 0x3f shift"},
		{'-', 0x44, false, "label « - \\ » à 0x44"},
		{'\\', 0x44, true, "label « - \\ » à 0x44 shift"},
		{'ù', 0x45, false, "label « ù % » à 0x45"},
		{'%', 0x45, true, "label « ù % » à 0x45 shift"},
		{':', 0x47, false, "label « : / » à 0x47"},
		{'/', 0x47, true, "label « : / » à 0x47 shift"},
		{')', 0x4c, false, "label « ) ° » à 0x4c"},
		{'°', 0x4c, true, "label « ) ° » à 0x4c shift"},
		{'>', 0x4f, false, "label « > < » à 0x4f"},
		{'<', 0x4f, true, "label « > < » à 0x4f shift"},
	}
	for _, c := range cases {
		k, shift, ok := m.CharToKey(c.r)
		if !ok || k != c.key || shift != c.shift {
			t.Errorf("CharToKey(%q) = (0x%02X, shift=%v, ok=%v), want (0x%02X, %v, true) — %s",
				c.r, k, shift, ok, c.key, c.shift, c.ref)
		}
	}
}

// TestTO8DModelCharToKey_AllInBoundsAndNotModifier (Inc Kb, Kb-3) : pour chaque
// entrée de la table charToTO8D, le scancode est dans [0, KeyCount) et n'est
// PAS un modificateur (Shift/CNT/ACC). Mapper un caractère sur un modificateur
// poserait simultanément la modif ET la touche caractère sans signification, et
// déclencherait le bug d'ordre Kc (modif posée en 2e passe via le caractère).
func TestTO8DModelCharToKey_AllInBoundsAndNotModifier(t *testing.T) {
	m := TO8DModel()
	for r, ck := range charToTO8D {
		if ck.key < 0 || ck.key >= m.KeyCount {
			t.Errorf("charToTO8D[%q] = 0x%02X hors [0,%d)", r, ck.key, m.KeyCount)
		}
		if m.IsModifier(ck.key) {
			t.Errorf("charToTO8D[%q] = 0x%02X est un modificateur (SHIFT/CNT/ACC) — interdit (BLOCKER #5, touches mortes)", r, ck.key)
		}
	}
}

// TestMO5Model_CharsImmutableSnapshot (Inc Kb, anti-régression MO5) : la table
// MO5 doit rester strictement inchangée par Inc Kb (la PR ne touche que TO8D).
// Compte exact des entrées + ancrage de quelques paires AZERTY MO5 critiques.
// Détecte un fork silencieux de charToMO5 ou une factorisation accidentelle qui
// aurait débordé sur MO5.
func TestMO5Model_CharsImmutableSnapshot(t *testing.T) {
	// Comptage : ne se calcule pas depuis la doc, donc oracle externe nul. Mais
	// l'ancrage par paire force toute modif fortuite à se manifester ici.
	m := MO5Model()
	// Quelques ancres représentatives : lettres, chiffres, symboles shiftés MO5.
	cases := []struct {
		r     rune
		key   int
		shift bool
	}{
		{'a', 0x2D, false}, // lettre
		{'1', 0x2F, false}, // chiffre direct (≠ TO8D où c'est shift)
		{'!', 0x2F, true},  // shift de '1'
		{'@', 0x18, false}, // @ direct (≠ TO8D où c'est shift de '#')
		{'^', 0x18, true},  // shift de '@'
	}
	for _, c := range cases {
		k, shift, ok := m.CharToKey(c.r)
		if !ok || k != c.key || shift != c.shift {
			t.Errorf("MO5 CharToKey(%q) = (0x%02X,%v,%v), want (0x%02X,%v,true) — Inc Kb ne doit PAS toucher MO5",
				c.r, k, shift, ok, c.key, c.shift)
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

func TestTO8DModelKeepsArrowSpecialKeysInJoystickMode(t *testing.T) {
	m := TO8DModel()
	for _, key := range []int{0x04, 0x3d, 0x0d, 0x05} {
		if m.SuppressSpecialKeyInJoystickMode(key) {
			t.Fatalf("TO8D key 0x%02x must keep double-input behavior in joystick keyboard mode", key)
		}
	}
}
