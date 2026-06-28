package app_test

import (
	"strings"
	"testing"

	"github.com/Lesur-ai/dcmo5/internal/app" // init() peuple keyboard.*Model().SpecialKeys
	"github.com/Lesur-ai/dcmo5/internal/keyboard"
	"github.com/Lesur-ai/dcmo5/internal/machine"
	_ "github.com/Lesur-ai/dcmo5/internal/machine/mo5"  // enregistre profil mo5
	_ "github.com/Lesur-ai/dcmo5/internal/machine/to8d" // enregistre profil to8d
	"github.com/hajimehoshi/ebiten/v2"
)

func TestKeyMappingNoDuplicates(t *testing.T) {
	seen := map[int]string{}
	// Doublons légitimes : SHIFT (0x38), CNT (0x35), ACC (0x36) ont chacun une
	// variante gauche et droite sur le même index MO5.
	legit := map[int]bool{0x38: true, 0x35: true, 0x36: true}
	for eKey, mo5Key := range app.KeyMapping() {
		if legit[mo5Key] {
			continue
		}
		if prev, dup := seen[mo5Key]; dup {
			t.Errorf("doublon MO5 0x%02X: %v et %v", mo5Key, prev, eKey)
		}
		seen[mo5Key] = eKey.String()
	}
}

func TestKeyMappingValidRange(t *testing.T) {
	for eKey, mo5Key := range app.KeyMapping() {
		if mo5Key < 0 || mo5Key >= keyboard.MO5Model().KeyCount {
			t.Errorf("touche %v → index MO5 %d hors-bornes [0,%d)", eKey, mo5Key, keyboard.MO5Model().KeyCount)
		}
	}
}

// ── Inc Ka : SpecialKeys data-driven ─────────────────────────────────────────
//
// Les tests suivants ancrent les invariants de la table SpecialKeys par MACHINE
// (peuplée par internal/app/keyboard_init.go au load du paquet). Garde-fous :
// non-régression MO5 (ancrage des 18 entrées par valeur), validité TO8D
// (ENTER ≠ ESPACE — c'est le bug observé pré-Ka), test paramétré sur
// machine.Profiles() pour anticiper TO9+.

// TestSpecialKeys_MO5_AnchoredVerbatim : ancrage par VALEUR des 18 entrées MO5
// transférées depuis l'ancienne var keyMapping (app.go:983-1002 pré-Ka).
// Toute modification accidentelle de ces valeurs fera échouer ce test. Sert
// d'oracle externe contre une régression MO5 silencieuse.
func TestSpecialKeys_MO5_AnchoredVerbatim(t *testing.T) {
	sp := keyboard.MO5Model().SpecialKeys
	if sp == nil {
		t.Fatal("MO5Model().SpecialKeys est nil — keyboard_init.go n'a pas tourné")
	}
	want := map[ebiten.Key]int{
		ebiten.KeySpace:        0x20,
		ebiten.KeyEnter:        0x34,
		ebiten.KeyBackspace:    0x01,
		ebiten.KeyInsert:       0x09,
		ebiten.KeyDelete:       0x33,
		ebiten.KeyHome:         0x11,
		ebiten.KeyArrowRight:   0x19,
		ebiten.KeyArrowLeft:    0x29,
		ebiten.KeyArrowDown:    0x21,
		ebiten.KeyArrowUp:      0x31,
		ebiten.KeyShiftLeft:    0x38,
		ebiten.KeyShiftRight:   0x38,
		ebiten.KeyControlLeft:  0x35,
		ebiten.KeyControlRight: 0x35,
		ebiten.KeyAltLeft:      0x36,
		ebiten.KeyAltRight:     0x36,
		ebiten.KeyTab:          0x39,
		ebiten.KeyEnd:          0x37,
	}
	if len(sp) != len(want) {
		t.Errorf("MO5 SpecialKeys a %d entrées, want %d", len(sp), len(want))
	}
	for k, v := range want {
		got, ok := sp[int(k)]
		if !ok {
			t.Errorf("MO5 SpecialKeys: %v absent (want 0x%02X)", k, v)
			continue
		}
		if got != v {
			t.Errorf("MO5 SpecialKeys[%v] = 0x%02X, want 0x%02X", k, got, v)
		}
	}
}

// TestSpecialKeys_TO8D_CoreKeys : les touches CRITIQUES de TO8D doivent avoir
// les scancodes attendus depuis dcto8dkeyb.h. C'est ce test qui prouve que le
// bug initial est résolu : ENTER = 0x46 (≠ 0x34 ESPACE), SHIFT = 0x51, CNT =
// 0x53, ACC = 0x14, Ent pad (0x36) ≠ Enter principal (0x46).
func TestSpecialKeys_TO8D_CoreKeys(t *testing.T) {
	sp := keyboard.TO8DModel().SpecialKeys
	if sp == nil {
		t.Fatal("TO8DModel().SpecialKeys est nil — keyboard_init.go n'a pas tourné")
	}
	type want struct {
		key  ebiten.Key
		code int
		why  string
	}
	cases := []want{
		{ebiten.KeyEnter, 0x46, "ENTER → ENT principale TO8D (bug pré-Ka : 0x34 = ESPACE)"},
		{ebiten.KeySpace, 0x34, "ESPACE"},
		{ebiten.KeyShiftLeft, 0x51, "SHIFT (to8dKeyShift) — KeyShift gauche"},
		{ebiten.KeyShiftRight, 0x51, "SHIFT — KeyShift droit pointe sur le MÊME scancode 0x51 (1ère passe ModifierKeys)"},
		{ebiten.KeyControlLeft, 0x53, "CNT (to8dKeyCNT)"},
		{ebiten.KeyAltLeft, 0x14, "ACC (to8dKeyACC, accent)"},
		{ebiten.KeyArrowUp, 0x04, "flèche haut"},
		{ebiten.KeyArrowDown, 0x3d, "flèche bas"},
		{ebiten.KeyArrowLeft, 0x0d, "flèche gauche"},
		{ebiten.KeyArrowRight, 0x05, "flèche droite"},
		{ebiten.KeyBackspace, 0x16, "EFF (effacement)"},
		{ebiten.KeyDelete, 0x06, "RAZ (remise à zéro)"},
		{ebiten.KeyInsert, 0x0e, "INS"},
		{ebiten.KeyEnd, 0x30, "STOP"},
		{ebiten.KeyKPEnter, 0x36, "Ent pad ≠ ENT principale (0x46)"},
	}
	for _, c := range cases {
		got, ok := sp[int(c.key)]
		if !ok {
			t.Errorf("TO8D SpecialKeys: %v absent (want 0x%02X — %s)", c.key, c.code, c.why)
			continue
		}
		if got != c.code {
			t.Errorf("TO8D SpecialKeys[%v] = 0x%02X, want 0x%02X (%s)", c.key, got, c.code, c.why)
		}
	}
	// Invariant critique : Ent pad et ENT principale ont des scancodes distincts.
	if sp[int(ebiten.KeyKPEnter)] == sp[int(ebiten.KeyEnter)] {
		t.Errorf("TO8D : KeyKPEnter et KeyEnter pointent sur le même scancode 0x%02X — ils doivent être distincts", sp[int(ebiten.KeyEnter)])
	}
}

// TestSpecialKeys_TO8D_NumpadComplete : les 13 touches numpad sont mappées et
// pointent sur les scancodes TO8D numpad du gate-array (dcto8dkeyb.h labels
// « N pad » / « Ent pad »). Couvre BLOCKER #4 owner.
func TestSpecialKeys_TO8D_NumpadComplete(t *testing.T) {
	sp := keyboard.TO8DModel().SpecialKeys
	want := map[ebiten.Key]int{
		ebiten.KeyKP0:       0x1e,
		ebiten.KeyKP1:       0x15,
		ebiten.KeyKP2:       0x25,
		ebiten.KeyKP3:       0x4e,
		ebiten.KeyKP4:       0x1d,
		ebiten.KeyKP5:       0x2d,
		ebiten.KeyKP6:       0x2e,
		ebiten.KeyKP7:       0x1c,
		ebiten.KeyKP8:       0x24,
		ebiten.KeyKP9:       0x35,
		ebiten.KeyKPDecimal: 0x26,
		ebiten.KeyKPEnter:   0x36,
	}
	for k, v := range want {
		got, ok := sp[int(k)]
		if !ok {
			t.Errorf("TO8D numpad: %v absent (want 0x%02X)", k, v)
			continue
		}
		if got != v {
			t.Errorf("TO8D SpecialKeys[%v] = 0x%02X, want 0x%02X (numpad)", k, got, v)
		}
	}
	// Tous distincts entre eux (pas de collision dans le numpad).
	seen := map[int]ebiten.Key{}
	for k, v := range want {
		if prev, dup := seen[v]; dup {
			t.Errorf("TO8D numpad : 0x%02X partagé par %v et %v — pas de collision attendue", v, prev, k)
		}
		seen[v] = k
	}
}

// modelsUnderTest est la liste des modèles testés en paramétré. Quand TO9+
// sera ajouté (v2.1), ajouter ici son modèle ET son keyboard_init.go : ces
// tests détecteront alors automatiquement les invariants manquants.
func modelsUnderTest(t *testing.T) []struct {
	id    string
	model *keyboard.Model
} {
	t.Helper()
	// Garde-fou : machine.Profiles() doit contenir au moins mo5 et to8d. Si la
	// liste change, ce test guidera l'ajout au tableau ci-dessus.
	ids := map[string]bool{}
	for _, p := range machine.Profiles() {
		ids[p.ID] = true
	}
	for _, want := range []string{"mo5", "to8d"} {
		if !ids[want] {
			t.Fatalf("machine.Profiles() ne contient pas le profil %q — registry cassée", want)
		}
	}
	return []struct {
		id    string
		model *keyboard.Model
	}{
		{"mo5", keyboard.MO5Model()},
		{"to8d", keyboard.TO8DModel()},
	}
}

// TestSpecialKeys_AllInBounds_AllModels : pour CHAQUE machine, toute valeur de
// SpecialKeys doit être dans [0, KeyCount). Détecte un copier-coller MO5→TO8D
// avec un scancode hors borne (ex. KeyCount MO5=58, KeyCount TO8D=84 ; un code
// TO8D 0x83 dans la table MO5 pointerait hors borne).
func TestSpecialKeys_AllInBounds_AllModels(t *testing.T) {
	for _, mt := range modelsUnderTest(t) {
		m := mt.model
		if m.SpecialKeys == nil {
			t.Errorf("modèle %s : SpecialKeys nil — keyboard_init.go ne couvre pas cette machine", mt.id)
			continue
		}
		for hostKey, machineKey := range m.SpecialKeys {
			if machineKey < 0 || machineKey >= m.KeyCount {
				t.Errorf("modèle %s : SpecialKeys[%v] = 0x%02X hors [0,%d) — copier-coller incorrect ?",
					mt.id, ebiten.Key(hostKey), machineKey, m.KeyCount)
			}
		}
	}
}

// TestSpecialKeys_NoCharacterKeys_AllModels : aucune touche caractère hôte
// (KeyA..KeyZ, Key0..Key9) ne doit apparaître dans SpecialKeys d'aucune machine.
// Mapper une lettre/chiffre en positionnel casse l'indépendance de layout
// (AZERTY/QWERTY) : les caractères passent par CharToKey + l'injecteur, qui
// utilise le layout OS. Garde-fou critique anti-régression, généralisé.
func TestSpecialKeys_NoCharacterKeys_AllModels(t *testing.T) {
	letters := []ebiten.Key{
		ebiten.KeyA, ebiten.KeyB, ebiten.KeyC, ebiten.KeyD, ebiten.KeyE,
		ebiten.KeyF, ebiten.KeyG, ebiten.KeyH, ebiten.KeyI, ebiten.KeyJ,
		ebiten.KeyK, ebiten.KeyL, ebiten.KeyM, ebiten.KeyN, ebiten.KeyO,
		ebiten.KeyP, ebiten.KeyQ, ebiten.KeyR, ebiten.KeyS, ebiten.KeyT,
		ebiten.KeyU, ebiten.KeyV, ebiten.KeyW, ebiten.KeyX, ebiten.KeyY, ebiten.KeyZ,
	}
	digits := []ebiten.Key{
		ebiten.Key0, ebiten.Key1, ebiten.Key2, ebiten.Key3, ebiten.Key4,
		ebiten.Key5, ebiten.Key6, ebiten.Key7, ebiten.Key8, ebiten.Key9,
	}
	for _, mt := range modelsUnderTest(t) {
		if mt.model.SpecialKeys == nil {
			continue
		}
		for _, k := range append(letters, digits...) {
			if v, found := mt.model.SpecialKeys[int(k)]; found {
				t.Errorf("modèle %s : touche caractère %v mappée en positionnel (→0x%02X) — casse l'indépendance de layout (AZERTY→QWERTY)",
					mt.id, k, v)
			}
		}
	}
}

// TestSpecialKeys_ModifierConsistency_AllModels : pour chaque machine, les DEUX
// touches hôte (gauche ET droite) d'un modificateur doivent pointer sur l'indice
// retourné par ModifierKeys() (= m.ShiftKey, m.CNTKey, m.ACCKey). Garde-fou contre
// régression Kc : si KeyShiftRight pointe sur un AUTRE scancode (ex. 0x52 sur TO8D
// physique) qui n'est PAS dans ModifierKeys(), Host.tick l'applique en 2e passe
// après les caractères → le gate-array latch le caractère sans shift. Codex P2
// signalé sur Ka initial : ce test est l'oracle qui empêche le retour du bug.
func TestSpecialKeys_ModifierConsistency_AllModels(t *testing.T) {
	for _, mt := range modelsUnderTest(t) {
		m := mt.model
		if m.SpecialKeys == nil {
			continue
		}
		// Pour chaque modificateur : les deux touches hôte (Left ET Right) doivent
		// pointer sur l'indice modificateur du modèle. C'est ce qui garantit que
		// la 1ère passe de Host.tick (modifs via ModifierKeys()) les pose toutes.
		mods := []struct {
			left, right ebiten.Key
			want        int
			label       string
			active      bool
		}{
			{ebiten.KeyShiftLeft, ebiten.KeyShiftRight, m.ShiftKey, "ShiftKey", true},
			{ebiten.KeyControlLeft, ebiten.KeyControlRight, m.CNTKey, "CNTKey", true},
			{ebiten.KeyAltLeft, ebiten.KeyAltRight, m.ACCKey, "ACCKey", m.ACCKey >= 0},
		}
		for _, mod := range mods {
			if !mod.active {
				continue
			}
			for _, side := range []struct {
				k     ebiten.Key
				label string
			}{{mod.left, "Left"}, {mod.right, "Right"}} {
				if v := m.SpecialKeys[int(side.k)]; v != mod.want {
					t.Errorf("modèle %s : SpecialKeys[%v]=0x%02X ≠ Model.%s=0x%02X — réintroduit le bug d'ordre Kc (modif en 2e passe)",
						mt.id, side.k, v, mod.label, mod.want)
				}
			}
		}
	}
}

// TestKeyMapping_NoCharacterKeys est une garde anti-régression : les touches de
// caractère (lettres, chiffres) ne doivent JAMAIS être mappées en positionnel.
// Elles passent par la saisie caractère (CharToMO5Key + AppendInputChars), qui
// respecte le layout OS. Les mapper ici réintroduirait le bug AZERTY→QWERTY :
// un « a » AZERTY serait lu à la position « q » d'un clavier QWERTY.
func TestKeyMapping_NoCharacterKeys(t *testing.T) {
	letters := []ebiten.Key{
		ebiten.KeyA, ebiten.KeyB, ebiten.KeyC, ebiten.KeyD, ebiten.KeyE,
		ebiten.KeyF, ebiten.KeyG, ebiten.KeyH, ebiten.KeyI, ebiten.KeyJ,
		ebiten.KeyK, ebiten.KeyL, ebiten.KeyM, ebiten.KeyN, ebiten.KeyO,
		ebiten.KeyP, ebiten.KeyQ, ebiten.KeyR, ebiten.KeyS, ebiten.KeyT,
		ebiten.KeyU, ebiten.KeyV, ebiten.KeyW, ebiten.KeyX, ebiten.KeyY, ebiten.KeyZ,
	}
	digits := []ebiten.Key{
		ebiten.Key0, ebiten.Key1, ebiten.Key2, ebiten.Key3, ebiten.Key4,
		ebiten.Key5, ebiten.Key6, ebiten.Key7, ebiten.Key8, ebiten.Key9,
	}
	mapping := app.KeyMapping()
	for _, k := range append(letters, digits...) {
		if mo5, found := mapping[k]; found {
			t.Errorf("touche caractère %v mappée en positionnel (→0x%02X) : "+
				"casse l'indépendance de layout (AZERTY→QWERTY)", k, mo5)
		}
	}
}

// ── TitleForState ─────────────────────────────────────────────────────────────

func TestTitle_Normal(t *testing.T) {
	got := app.TitleForState(false, false, "mo5.rom", "", "")
	if !strings.Contains(got, "mo5.rom") {
		t.Errorf("titre normal: %q ne contient pas 'mo5.rom'", got)
	}
	if strings.Contains(got, "PAUSE") || strings.Contains(got, "manquante") {
		t.Errorf("titre normal ne doit pas contenir PAUSE/manquante: %q", got)
	}
}

func TestTitle_ROMmissing(t *testing.T) {
	got := app.TitleForState(true, false, "", "", "")
	if !strings.Contains(got, "manquante") {
		t.Errorf("ROM manquante: %q ne contient pas 'manquante'", got)
	}
}

func TestTitle_Paused(t *testing.T) {
	got := app.TitleForState(false, true, "mo5.rom", "", "")
	if !strings.Contains(got, "[PAUSE]") {
		t.Errorf("pause: %q ne contient pas '[PAUSE]'", got)
	}
}

func TestTitle_WithTape(t *testing.T) {
	got := app.TitleForState(false, false, "mo5.rom", "jeu.k7", "")
	if !strings.Contains(got, "jeu.k7") {
		t.Errorf("avec tape: %q ne contient pas 'jeu.k7'", got)
	}
}

func TestTitle_PausedROMmissing(t *testing.T) {
	got := app.TitleForState(true, true, "", "", "")
	if !strings.Contains(got, "manquante") || !strings.Contains(got, "[PAUSE]") {
		t.Errorf("pause+ROM manquante: %q", got)
	}
}
