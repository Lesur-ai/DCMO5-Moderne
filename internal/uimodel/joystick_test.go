package uimodel

import (
	"testing"

	"github.com/Lesur-ai/dcmo5/internal/machine"
)

// testMapping fournit une KeyboardJoystickMapping fixe pour les tests. Les
// valeurs sont arbitraires mais distinctes : le test ne dépend pas du layout
// réel du clavier, juste de la cohérence du mapping.
var testMapping = KeyboardJoystickMapping{
	J1Up: 10, J1Down: 11, J1Left: 12, J1Right: 13, J1Fire: 14,
	J2Up: 20, J2Down: 21, J2Left: 22, J2Right: 23, J2Fire: 24,
}

// pressedSet construit une fonction `pressed` qui retourne true pour les
// KeyCode listées, false sinon. Utilisé pour simuler l'état clavier sans
// dépendre d'Ebitengine.
func pressedSet(keys ...KeyCode) func(KeyCode) bool {
	set := make(map[KeyCode]bool, len(keys))
	for _, k := range keys {
		set[k] = true
	}
	return func(k KeyCode) bool { return set[k] }
}

// TestJoystickFromKeys_NeutralWhenNothingPressed (J2b.T1) : aucune touche
// pressée → state IDENTIQUE à machine.NeutralJoystick. Garde-fou : si le défaut
// 0xFF/0xC0 dérapait (ex. on poserait par erreur un bit en logique normale),
// ce test détecterait. Aussi vérifie qu'on ne touche pas aux bits 0..5 de
// Action (= bits son OR'és par le hardware).
func TestJoystickFromKeys_NeutralWhenNothingPressed(t *testing.T) {
	got := JoystickFromKeys(testMapping, pressedSet())
	if got != machine.NeutralJoystick {
		t.Errorf("JoystickFromKeys() = %+v, want %+v (NeutralJoystick)", got, machine.NeutralJoystick)
	}
}

// TestJoystickFromKeys_DirectionsBitConvention (J2b.T2) : pour CHAQUE direction
// (J1 et J2, nord/sud/ouest/est, 8 cas), une seule touche pressée éteint le
// bit correspondant et SEUL ce bit. Anti-tautologie : on vérifie aussi qu'AUCUN
// autre bit n'est éteint (= équivalence stricte avec le want), pas juste « le
// bit est éteint ».
//
// Table figée par J1b — toute modification ici doit être répercutée dans
// internal/core/bus_test.go::TestBus_Joystick_BitConvention_Inverted et
// internal/machine/gatearray/io_test.go::TestSetJoystick_BitConvention_TO8D.
func TestJoystickFromKeys_DirectionsBitConvention(t *testing.T) {
	cases := []struct {
		name    string
		press   KeyCode
		wantPos uint8
		wantAct uint8
	}{
		{"J1 nord", testMapping.J1Up, 0xFE, 0xC0},
		{"J1 sud", testMapping.J1Down, 0xFD, 0xC0},
		{"J1 ouest", testMapping.J1Left, 0xFB, 0xC0},
		{"J1 est", testMapping.J1Right, 0xF7, 0xC0},
		{"J2 nord", testMapping.J2Up, 0xEF, 0xC0},
		{"J2 sud", testMapping.J2Down, 0xDF, 0xC0},
		{"J2 ouest", testMapping.J2Left, 0xBF, 0xC0},
		{"J2 est", testMapping.J2Right, 0x7F, 0xC0},
		{"J1 fire", testMapping.J1Fire, 0xFF, 0x80},
		{"J2 fire", testMapping.J2Fire, 0xFF, 0x40},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := JoystickFromKeys(testMapping, pressedSet(c.press))
			want := machine.JoystickInput{Position: c.wantPos, Action: c.wantAct}
			if got != want {
				t.Errorf("JoystickFromKeys avec %s = %+v, want %+v", c.name, got, want)
			}
		})
	}
}

// TestJoystickFromKeys_OpposingDirectionsCancel (J2b.T3) : règle « directions
// opposées s'annulent ». Si J1Up ET J1Down sont pressées simultanément, AUCUNE
// n'est posée (bits 0 et 1 restent à 1). Couverture des 4 paires d'opposés
// possibles (J1 N/S, J1 O/E, J2 N/S, J2 O/E) + cas mixte. Sans cette règle,
// l'utilisateur appuyant sur deux directions opposées (par erreur ou bug
// gamepad analogique) ferait clignoter l'état joystick côté CPU.
func TestJoystickFromKeys_OpposingDirectionsCancel(t *testing.T) {
	cases := []struct {
		name    string
		presses []KeyCode
		want    uint8 // wantPosition (action = 0xC0 dans tous les cas)
	}{
		{"J1 nord + sud → repos", []KeyCode{testMapping.J1Up, testMapping.J1Down}, 0xFF},
		{"J1 ouest + est → repos", []KeyCode{testMapping.J1Left, testMapping.J1Right}, 0xFF},
		{"J2 nord + sud → repos", []KeyCode{testMapping.J2Up, testMapping.J2Down}, 0xFF},
		{"J2 ouest + est → repos", []KeyCode{testMapping.J2Left, testMapping.J2Right}, 0xFF},
		// J1 nord seul + J2 sud seul : pas d'opposition, les deux bits éteints.
		{"J1 nord + J2 sud (indépendants)", []KeyCode{testMapping.J1Up, testMapping.J2Down}, 0xFE & 0xDF}, // = 0xDE
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := JoystickFromKeys(testMapping, pressedSet(c.presses...))
			want := machine.JoystickInput{Position: c.want, Action: 0xC0}
			if got != want {
				t.Errorf("JoystickFromKeys %s = %+v, want %+v", c.name, got, want)
			}
		})
	}
}

// TestJoystickFromKeys_CombinedDirectionsAndFire (J2b.T4) : un cas réaliste de
// gameplay J1 nord + J1 est diagonal + J1 fire. Tous les bits attendus sont
// éteints, AUCUN autre.
func TestJoystickFromKeys_CombinedDirectionsAndFire(t *testing.T) {
	got := JoystickFromKeys(testMapping, pressedSet(
		testMapping.J1Up, testMapping.J1Right, testMapping.J1Fire,
	))
	// Position : bit 0 (nord) + bit 3 (est) éteints → 0xFF & ^0x09 = 0xF6.
	// Action : bit 6 (J1 fire) éteint → 0xC0 & ^0x40 = 0x80.
	want := machine.JoystickInput{Position: 0xF6, Action: 0x80}
	if got != want {
		t.Errorf("JoystickFromKeys nord+est+fire = %+v, want %+v", got, want)
	}
}

// TestJoystickFromKeys_TwoPlayersSimultaneous (J2b.T5) : J1 et J2 indépendants
// peuvent être actifs en parallèle. J1 ouest + J2 est + J1 fire + J2 fire.
func TestJoystickFromKeys_TwoPlayersSimultaneous(t *testing.T) {
	got := JoystickFromKeys(testMapping, pressedSet(
		testMapping.J1Left, testMapping.J2Right,
		testMapping.J1Fire, testMapping.J2Fire,
	))
	// J1 ouest (bit 2) + J2 est (bit 7) → 0xFF & ^0x84 = 0x7B.
	// J1 fire (bit 6) + J2 fire (bit 7) → 0xC0 & ^0xC0 = 0x00.
	want := machine.JoystickInput{Position: 0x7B, Action: 0x00}
	if got != want {
		t.Errorf("JoystickFromKeys 2 joueurs = %+v, want %+v", got, want)
	}
}

// TestReservedKeys_ReturnsAllMappedKeys (J2b.T6) : ReservedKeys retourne les
// 10 touches du mapping. Couverture pour la couche app qui doit exclure ces
// touches du clavier émulation (anti-pollution BASIC sur ZQSD/AltGr/Shift).
func TestReservedKeys_ReturnsAllMappedKeys(t *testing.T) {
	got := ReservedKeys(testMapping)
	want := map[KeyCode]bool{
		testMapping.J1Up: true, testMapping.J1Down: true,
		testMapping.J1Left: true, testMapping.J1Right: true,
		testMapping.J1Fire: true,
		testMapping.J2Up:   true, testMapping.J2Down: true,
		testMapping.J2Left: true, testMapping.J2Right: true,
		testMapping.J2Fire: true,
	}
	if len(got) != len(want) {
		t.Errorf("ReservedKeys len = %d, want %d (les 10 touches du mapping)", len(got), len(want))
	}
	for _, k := range got {
		if !want[k] {
			t.Errorf("ReservedKeys contient %d non listé dans le mapping", k)
		}
	}
}
