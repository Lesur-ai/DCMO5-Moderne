package uimodel

import (
	"testing"

	"github.com/Lesur-ai/dcmo5/internal/machine"
)

// TestMergeJoysticks_BothNeutral (J2c.T1) : NeutralJoystick + NeutralJoystick
// reste NeutralJoystick. Garde-fou : sans cette propriété, l'absence d'appui
// produirait un état dégradé (par exemple si on faisait un OR au lieu d'un AND).
func TestMergeJoysticks_BothNeutral(t *testing.T) {
	got := MergeJoysticks(machine.NeutralJoystick, machine.NeutralJoystick)
	if got != machine.NeutralJoystick {
		t.Errorf("Merge(neutral, neutral) = %+v, want %+v", got, machine.NeutralJoystick)
	}
}

// TestMergeJoysticks_OneSidePressed (J2c.T2) : une source seule (l'autre neutre)
// est transmise telle quelle. Vérifie qu'AND avec 0xFF/0xC0 ne modifie pas la
// source active — propriété essentielle de l'élément neutre.
func TestMergeJoysticks_OneSidePressed(t *testing.T) {
	cases := []struct {
		name string
		src  machine.JoystickInput
	}{
		{"J1 nord", machine.JoystickInput{Position: 0xFE, Action: 0xC0}},
		{"J1 fire", machine.JoystickInput{Position: 0xFF, Action: 0x80}},
		{"J2 sud + J2 fire", machine.JoystickInput{Position: 0xDF, Action: 0x40}},
	}
	for _, c := range cases {
		t.Run(c.name+" sur a", func(t *testing.T) {
			got := MergeJoysticks(c.src, machine.NeutralJoystick)
			if got != c.src {
				t.Errorf("Merge(%+v, neutral) = %+v, want %+v (a doit être préservé)", c.src, got, c.src)
			}
		})
		t.Run(c.name+" sur b", func(t *testing.T) {
			got := MergeJoysticks(machine.NeutralJoystick, c.src)
			if got != c.src {
				t.Errorf("Merge(neutral, %+v) = %+v, want %+v (b doit être préservé)", c.src, got, c.src)
			}
		})
	}
}

// TestMergeJoysticks_DifferentPlayersCombine (J2c.T3) : J1 actif sur source A +
// J2 actif sur source B → résultat combiné (les deux joueurs visibles). Cas
// d'usage typique : clavier joue J1, gamepad joue J2.
func TestMergeJoysticks_DifferentPlayersCombine(t *testing.T) {
	keyboardJ1 := machine.JoystickInput{Position: 0xFE, Action: 0x80} // J1 nord + fire
	gamepadJ2 := machine.JoystickInput{Position: 0xDF, Action: 0x40}  // J2 sud + fire
	got := MergeJoysticks(keyboardJ1, gamepadJ2)
	want := machine.JoystickInput{
		Position: 0xFE & 0xDF, // = 0xDE
		Action:   0x80 & 0x40, // = 0x00
	}
	if got != want {
		t.Errorf("Merge(keyboardJ1, gamepadJ2) = %+v, want %+v", got, want)
	}
}

// TestMergeJoysticks_SamePlayerOR (J2c.T4) : deux sources pour le MÊME joueur
// (clavier + gamepad J1 simultanément) s'OR-isent : si l'une ou l'autre pose
// le bit nord, il est posé dans le résultat. Permet à l'utilisateur d'utiliser
// les deux interfaces en même temps sans surprise.
func TestMergeJoysticks_SamePlayerOR(t *testing.T) {
	keyboardJ1Nord := machine.JoystickInput{Position: 0xFE, Action: 0xC0}
	gamepadJ1Fire := machine.JoystickInput{Position: 0xFF, Action: 0x80}
	got := MergeJoysticks(keyboardJ1Nord, gamepadJ1Fire)
	want := machine.JoystickInput{
		Position: 0xFE, // nord du clavier
		Action:   0x80, // fire du gamepad
	}
	if got != want {
		t.Errorf("Merge(keyboardJ1Nord, gamepadJ1Fire) = %+v, want %+v", got, want)
	}
}

// TestMergeJoysticks_Associative (J2c.T5) : Merge est associatif, on peut
// composer 3+ sources par appels successifs. Cas d'usage : clavier + gamepadJ1
// + gamepadJ2 fusionnés dans App.Update (J3a/J5a). Sans cette propriété, l'ordre
// d'appel pourrait altérer le résultat.
func TestMergeJoysticks_Associative(t *testing.T) {
	a := machine.JoystickInput{Position: 0xFE, Action: 0xC0} // J1 nord
	b := machine.JoystickInput{Position: 0xDF, Action: 0xC0} // J2 sud
	c := machine.JoystickInput{Position: 0xFF, Action: 0x40} // J2 fire

	left := MergeJoysticks(MergeJoysticks(a, b), c)
	right := MergeJoysticks(a, MergeJoysticks(b, c))
	if left != right {
		t.Errorf("Merge n'est pas associatif : (a∧b)∧c = %+v ≠ a∧(b∧c) = %+v", left, right)
	}
	// Vérifie aussi que le résultat est cohérent (tous les bits appuyés visibles).
	want := machine.JoystickInput{
		Position: 0xFE & 0xDF & 0xFF, // = 0xDE
		Action:   0xC0 & 0xC0 & 0x40, // = 0x40
	}
	if left != want {
		t.Errorf("Merge(a, b, c) = %+v, want %+v", left, want)
	}
}

// TestMergeJoysticks_Commutative (J2c.T6) : Merge est commutatif (AND est
// commutatif). L'ordre des arguments est sans importance, propriété utile
// pour la couche app qui peut composer dans n'importe quel ordre.
func TestMergeJoysticks_Commutative(t *testing.T) {
	a := machine.JoystickInput{Position: 0xFE, Action: 0xC0}
	b := machine.JoystickInput{Position: 0xDF, Action: 0x40}
	if MergeJoysticks(a, b) != MergeJoysticks(b, a) {
		t.Error("Merge n'est pas commutatif")
	}
}

// TestMergeJoysticks_OppositeDirections (D5.T1) : opposés inter-sources. Si le
// clavier publie J1 nord (bit 0=0) et le gamepad publie J1 sud (bit 1=0), le
// résultat a les DEUX bits à 0 — la machine voit ↑+↓ simultanément. C'est un
// choix de design délibéré (D5 séance design 29/06/2026) : pas de cancel
// post-merge. Le hardware Thomson réel ne cancellait pas les opposés sur le port
// physique. Ce test DOCUMENTE le comportement retenu ; l'inverser nécessiterait
// une décision explicite de changement d'architecture.
func TestMergeJoysticks_OppositeDirections(t *testing.T) {
	keyboardJ1Nord := machine.JoystickInput{Position: 0xFE, Action: 0xC0} // J1 bit0=0 (nord)
	gamepadJ1Sud := machine.JoystickInput{Position: 0xFD, Action: 0xC0}   // J1 bit1=0 (sud)
	got := MergeJoysticks(keyboardJ1Nord, gamepadJ1Sud)
	// AND brut : 0xFE & 0xFD = 0xFC (bits 0 ET 1 à 0 = nord+sud simultanés)
	want := machine.JoystickInput{Position: 0xFC, Action: 0xC0}
	if got != want {
		t.Errorf("Merge(J1Nord, J1Sud) = %+v, want %+v (opposés doivent passer, pas être annulés)", got, want)
	}
	// Vérifie aussi est+ouest (J1 bits 2+3)
	keyboardJ1Ouest := machine.JoystickInput{Position: 0xFB, Action: 0xC0} // J1 bit2=0
	gamepadJ1Est := machine.JoystickInput{Position: 0xF7, Action: 0xC0}    // J1 bit3=0
	got2 := MergeJoysticks(keyboardJ1Ouest, gamepadJ1Est)
	want2 := machine.JoystickInput{Position: 0xF3, Action: 0xC0} // bits 2+3 à 0
	if got2 != want2 {
		t.Errorf("Merge(J1Ouest, J1Est) = %+v, want %+v (opposés doivent passer)", got2, want2)
	}
}
