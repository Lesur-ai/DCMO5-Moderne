package app

import (
	"testing"

	"github.com/Lesur-ai/dcmo5/internal/keyboard"
	"github.com/hajimehoshi/ebiten/v2"
)

func pressedSet(keys ...ebiten.Key) func(ebiten.Key) bool {
	set := map[ebiten.Key]bool{}
	for _, k := range keys {
		set[k] = true
	}
	return func(k ebiten.Key) bool { return set[k] }
}

func TestLearnLiveKeys(t *testing.T) {
	learned := map[ebiten.Key]liveKey{}
	none := func(ebiten.Key) bool { return false }

	// Touche-caractère : apprise depuis le caractère décodé. Utiliser KeyE (rune
	// 'a' sur AZERTY MAC en position E, peu importe la rune réelle, le test
	// teste le mécanisme). KeyA n'est pas utilisable depuis J3a (réservée
	// joystick J2 gauche, exclue de l'apprentissage — cf. joystick.go).
	learnLiveKeys(keyboard.MO5Model(), learned, []ebiten.Key{ebiten.KeyE}, []rune{'a'}, none, false)
	mo5, shift, ok := keyboard.CharToMO5Key('a')
	if !ok {
		t.Fatal("CharToMO5Key('a') devrait être mappé")
	}
	if got, has := learned[ebiten.KeyE]; !has || got.mo5 != mo5 || got.shift != shift || got.r != 'a' {
		t.Errorf("learned[KeyE] = %+v, want {mo5:%d shift:%t r:'a'}", got, mo5, shift)
	}

	// Touche spéciale : NON apprise (reste positionnelle).
	learnLiveKeys(keyboard.MO5Model(), learned, []ebiten.Key{ebiten.KeySpace}, []rune{' '}, none, false)
	if _, has := learned[ebiten.KeySpace]; has {
		t.Error("KeySpace ne doit pas être apprise (touche spéciale positionnelle)")
	}

	// Sans frappe just-pressed : pas d'apprentissage.
	before := len(learned)
	learnLiveKeys(keyboard.MO5Model(), learned, nil, []rune{'x'}, none, false)
	if len(learned) != before {
		t.Error("aucun apprentissage attendu sans touche just-pressed")
	}

	// Inc J3a : touches réservées joystick (WASD, AltGr) ne sont JAMAIS apprises
	// QUAND le mode joystick clavier est activé (F12). Sans cette règle, chaque
	// mouvement joystick J2 taperait Z/Q/S/D en BASIC.
	learnLiveKeys(keyboard.MO5Model(), learned, []ebiten.Key{ebiten.KeyW}, []rune{'z'}, none, true /* joystick activé */)
	if _, has := learned[ebiten.KeyW]; has {
		t.Error("joystick clavier ON : KeyW (J2 nord) ne doit JAMAIS être apprise comme touche-caractère")
	}
	learnLiveKeys(keyboard.MO5Model(), learned, []ebiten.Key{ebiten.KeyAltRight}, []rune{}, none, true)
	if _, has := learned[ebiten.KeyAltRight]; has {
		t.Error("joystick clavier ON : KeyAltRight (J1 fire) ne doit JAMAIS être apprise")
	}

	// Toggle DÉSACTIVÉ (= défaut) : KeyW est apprise comme touche-caractère
	// normale (mode BASIC propre, joystick clavier inactif).
	delete(learned, ebiten.KeyW) // purge éventuel résidu
	learnLiveKeys(keyboard.MO5Model(), learned, []ebiten.Key{ebiten.KeyW}, []rune{'z'}, none, false /* joystick désactivé */)
	zMo5, _, _ := keyboard.CharToMO5Key('z')
	if got, has := learned[ebiten.KeyW]; !has || got.r != 'z' || got.mo5 != zMo5 {
		t.Errorf("joystick clavier OFF : KeyW devrait apprendre 'z' (got=%+v has=%v)", got, has)
	}
}

func TestLearnLiveKeys_ExcludesHeldRepeats(t *testing.T) {
	// A déjà appris ('a') et TENU (répétition OS) ; on presse B → chars=['a','b']
	// (répétition de A + nouveau b). B doit apprendre 'b', pas 'a'.
	aMo5, _, _ := keyboard.CharToMO5Key('a')
	learned := map[ebiten.Key]liveKey{ebiten.KeyA: {mo5: aMo5, shift: false, r: 'a'}}
	pressed := pressedSet(ebiten.KeyA) // A tenu
	learnLiveKeys(keyboard.MO5Model(), learned, []ebiten.Key{ebiten.KeyB}, []rune{'a', 'b'}, pressed, false)
	bMo5, _, _ := keyboard.CharToMO5Key('b')
	if got := learned[ebiten.KeyB]; got.r != 'b' || got.mo5 != bMo5 {
		t.Errorf("learned[KeyB] = %+v, want r:'b' mo5:%d (répétition de A exclue)", got, bMo5)
	}
}

func TestLearnLiveKeys_PurgesStaleOnNoChar(t *testing.T) {
	// Touche apprise puis re-pressée sans produire de caractère MO5 → purge.
	learned := map[ebiten.Key]liveKey{ebiten.KeyE: {mo5: 0x02, shift: false, r: 'e'}}
	learnLiveKeys(keyboard.MO5Model(), learned, []ebiten.Key{ebiten.KeyE}, nil, func(ebiten.Key) bool { return false }, false)
	if _, has := learned[ebiten.KeyE]; has {
		t.Error("association obsolète devrait être purgée quand aucun caractère MO5")
	}
}

func TestResolveKeys_HeldCharKey(t *testing.T) {
	mo5, _, _ := keyboard.CharToMO5Key('a')
	learned := map[ebiten.Key]liveKey{ebiten.KeyA: {mo5: mo5, shift: false}}

	// Touche apprise tenue → touche MO5 tenue.
	in := resolveKeys(keyboard.MO5Model(), pressedSet(ebiten.KeyA), learned, false, nil, false)
	if !in.Keys[mo5] {
		t.Errorf("touche MO5 %d devrait être tenue", mo5)
	}

	// Relâchée → non tenue.
	in = resolveKeys(keyboard.MO5Model(), pressedSet(), learned, false, nil, false)
	if in.Keys[mo5] {
		t.Error("touche relâchée ne doit pas être tenue")
	}
}

func TestResolveKeys_AntiDoubleShift(t *testing.T) {
	// Caractère MO5 sans shift, mais Shift physique tenu (cas AZERTY rangée
	// chiffres) → le Shift MO5 doit être SUPPRIMÉ (piloté par le caractère).
	mo5, _, _ := keyboard.CharToMO5Key('a')
	learned := map[ebiten.Key]liveKey{ebiten.KeyA: {mo5: mo5, shift: false}}
	in := resolveKeys(keyboard.MO5Model(), pressedSet(ebiten.KeyA, ebiten.KeyShiftLeft), learned, false, nil, false)
	if in.Keys[keyboard.Mo5KeyShift] {
		t.Error("Shift physique doit être ignoré quand une touche-caractère est tenue")
	}
	if !in.Keys[mo5] {
		t.Error("la touche-caractère doit rester tenue")
	}

	// Caractère MO5 qui EXIGE le shift (ex. '&' sur MO5) → Shift MO5 posé même
	// sans shift physique.
	mo5Amp, shiftAmp, ok := keyboard.CharToMO5Key('&')
	if !ok || !shiftAmp {
		t.Fatalf("'&' devrait exiger le shift MO5 (got shift=%t ok=%t)", shiftAmp, ok)
	}
	learned2 := map[ebiten.Key]liveKey{ebiten.KeyB: {mo5: mo5Amp, shift: true}}
	in = resolveKeys(keyboard.MO5Model(), pressedSet(ebiten.KeyB), learned2, false, nil, false)
	if !in.Keys[mo5Amp] || !in.Keys[keyboard.Mo5KeyShift] {
		t.Error("'&' tenu : touche MO5 + Shift MO5 attendus")
	}
}

func TestResolveKeys_TO9PLiveDigitUsesTO9PModel(t *testing.T) {
	learned := map[ebiten.Key]liveKey{}
	learnLiveKeys(keyboard.TO9PModel(), learned, []ebiten.Key{ebiten.KeyDigit1}, []rune{'1'}, pressedSet(), false)
	got, has := learned[ebiten.KeyDigit1]
	if !has {
		t.Fatal("KeyDigit1 devrait être apprise via le modèle TO9+")
	}
	to9Key, shift, ok := keyboard.TO9PModel().CharToKey('1')
	if !ok || !shift {
		t.Fatalf("TO9PModel.CharToKey('1') = key:%d shift:%t ok:%t, want shift true", to9Key, shift, ok)
	}
	if got.mo5 != to9Key || !got.shift {
		t.Fatalf("learned[KeyDigit1] = %+v, want key:%d shift:true", got, to9Key)
	}
	in := resolveKeys(keyboard.TO9PModel(), pressedSet(ebiten.KeyDigit1), learned, false, nil, false)
	if !in.Keys[to9Key] {
		t.Fatalf("touche TO9+ %d non tenue après résolution live", to9Key)
	}
	if !in.Keys[keyboard.TO9PModel().ShiftKey] {
		t.Fatal("Shift TO9+ doit être posé pour taper '1' en live")
	}
}

func TestResolveKeys_SuppressesAltGrWhenCharHeld(t *testing.T) {
	// Caractère tenu + AltGr physique (Ctrl+Alt) → ACC/CNT MO5 ne doivent PAS
	// fuiter (ex. AltGr+0 = '@' sur AZERTY : le caractère encode déjà tout).
	mo5, _, _ := keyboard.CharToMO5Key('@')
	learned := map[ebiten.Key]liveKey{ebiten.KeyDigit0: {mo5: mo5, shift: false, r: '@'}}
	in := resolveKeys(keyboard.MO5Model(), pressedSet(ebiten.KeyDigit0, ebiten.KeyAltLeft, ebiten.KeyControlLeft), learned, false, nil, false)
	if in.Keys[keyboard.Mo5KeyCNT] || in.Keys[mo5KeyACC] {
		t.Error("CNT/ACC physiques (AltGr) doivent être ignorés quand une touche-caractère est tenue")
	}
	if !in.Keys[mo5] {
		t.Error("la touche-caractère '@' doit rester tenue")
	}
}

func TestResolveKeys_PhysicalShiftPositionalWhenNoChar(t *testing.T) {
	// Aucune touche-caractère tenue → Shift physique posé positionnellement.
	in := resolveKeys(keyboard.MO5Model(), pressedSet(ebiten.KeyShiftLeft), map[ebiten.Key]liveKey{}, false, nil, false)
	if !in.Keys[keyboard.Mo5KeyShift] {
		t.Error("Shift physique seul doit poser le Shift MO5")
	}
}

func TestResolveKeys_SpecialKeysHeld(t *testing.T) {
	// Les touches spéciales (flèches) sont tenues positionnellement.
	in := resolveKeys(keyboard.MO5Model(), pressedSet(ebiten.KeyArrowRight), map[ebiten.Key]liveKey{}, false, nil, false)
	if !in.Keys[keyboard.MO5Model().SpecialKeys[int(ebiten.KeyArrowRight)]] {
		t.Error("flèche droite devrait être tenue")
	}
}

func TestResolveKeys_InjectingIgnoresLiveAndFiltersModifiers(t *testing.T) {
	mo5, _, _ := keyboard.CharToMO5Key('a')
	learned := map[ebiten.Key]liveKey{ebiten.KeyA: {mo5: mo5, shift: false}}
	// Pendant injection : touches apprises ignorées, Shift/CNT physiques filtrés,
	// tickKeys appliqués.
	in := resolveKeys(keyboard.MO5Model(), pressedSet(ebiten.KeyA, ebiten.KeyShiftLeft, ebiten.KeyControlLeft), learned, true, []int{0x12}, false)
	if in.Keys[mo5] {
		t.Error("touche apprise ne doit pas être appliquée pendant injection")
	}
	if in.Keys[keyboard.Mo5KeyShift] || in.Keys[keyboard.Mo5KeyCNT] {
		t.Error("Shift/CNT physiques doivent être filtrés pendant injection")
	}
	if !in.Keys[0x12] {
		t.Error("la touche injectée (tickKeys) doit être appliquée")
	}
}
