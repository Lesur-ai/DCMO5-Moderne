package uimodel

import (
	"testing"

	"github.com/Lesur-ai/dcmo5/internal/machine"
)

// TestJoystickFromGamepad_NotConnected (J4a.T1) : un slot vide (Connected=false)
// produit NeutralJoystick, indépendamment des autres champs du snapshot. Garde-
// fou crucial : si l'app construit un snapshot mais oublie de poser Connected
// après une déconnexion gamepad, le joueur ne resterait pas figé sur la dernière
// direction — il revient au repos.
func TestJoystickFromGamepad_NotConnected(t *testing.T) {
	// Snapshot délibérément « actif » mais marqué non connecté : tous les
	// champs sont ignorés.
	snap := GamepadSnapshot{
		Connected:  false,
		DPadUp:     true,
		DPadLeft:   true,
		LeftStickX: -1.0,
		LeftStickY: -1.0,
		FireA:      true,
		FireB:      true,
	}
	got := JoystickFromGamepad(snap, 0.3, PlayerOne)
	if got != machine.NeutralJoystick {
		t.Errorf("Connected=false: JoystickFromGamepad = %+v, want NeutralJoystick (slot vide doit être neutre malgré inputs)", got)
	}
}

// TestJoystickFromGamepad_ConnectedNeutralStick (J4a.T2) : connecté mais aucun
// input → NeutralJoystick. Détecte une régression où le simple fait d'être
// connecté poserait un bit (par ex. un offset oublié).
func TestJoystickFromGamepad_ConnectedNeutralStick(t *testing.T) {
	snap := GamepadSnapshot{Connected: true}
	got := JoystickFromGamepad(snap, 0.3, PlayerOne)
	if got != machine.NeutralJoystick {
		t.Errorf("connecté sans input: JoystickFromGamepad = %+v, want NeutralJoystick", got)
	}
}

// TestJoystickFromGamepad_DPadDirections_BitConvention (J4a.T3) : pour CHAQUE
// direction DPad (J1 et J2 séparément), une seule direction pressée éteint
// le bit attendu et SEUL ce bit. Table MIROIR de uimodel.JoystickFromKeys
// (J2b) et internal/core/bus_test.go (J1b) — toute divergence ici signalerait
// une rupture de parité entre clavier et gamepad.
func TestJoystickFromGamepad_DPadDirections_BitConvention(t *testing.T) {
	cases := []struct {
		name    string
		snap    GamepadSnapshot
		player  int
		wantPos uint8
		wantAct uint8
	}{
		{"J1 DPad nord", GamepadSnapshot{Connected: true, DPadUp: true}, PlayerOne, 0xFE, 0xC0},
		{"J1 DPad sud", GamepadSnapshot{Connected: true, DPadDown: true}, PlayerOne, 0xFD, 0xC0},
		{"J1 DPad ouest", GamepadSnapshot{Connected: true, DPadLeft: true}, PlayerOne, 0xFB, 0xC0},
		{"J1 DPad est", GamepadSnapshot{Connected: true, DPadRight: true}, PlayerOne, 0xF7, 0xC0},
		{"J2 DPad nord", GamepadSnapshot{Connected: true, DPadUp: true}, PlayerTwo, 0xEF, 0xC0},
		{"J2 DPad sud", GamepadSnapshot{Connected: true, DPadDown: true}, PlayerTwo, 0xDF, 0xC0},
		{"J2 DPad ouest", GamepadSnapshot{Connected: true, DPadLeft: true}, PlayerTwo, 0xBF, 0xC0},
		{"J2 DPad est", GamepadSnapshot{Connected: true, DPadRight: true}, PlayerTwo, 0x7F, 0xC0},
		{"J1 FireA seul", GamepadSnapshot{Connected: true, FireA: true}, PlayerOne, 0xFF, 0x80},
		{"J1 FireB seul", GamepadSnapshot{Connected: true, FireB: true}, PlayerOne, 0xFF, 0x80},
		{"J2 FireA seul", GamepadSnapshot{Connected: true, FireA: true}, PlayerTwo, 0xFF, 0x40},
		{"J2 FireB seul", GamepadSnapshot{Connected: true, FireB: true}, PlayerTwo, 0xFF, 0x40},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := JoystickFromGamepad(c.snap, 0.3, c.player)
			want := machine.JoystickInput{Position: c.wantPos, Action: c.wantAct}
			if got != want {
				t.Errorf("%s = %+v, want %+v", c.name, got, want)
			}
		})
	}
}

// TestJoystickFromGamepad_StickDeadzone (J4a.T4) : valeurs stick juste en-deçà
// du deadzone NE déclenchent PAS la direction (= repos). Juste au-delà,
// déclenchement. Ancre la sémantique du seuil — sans ce test, une régression
// inverse (< vs ≤, par exemple) passerait inaperçue.
func TestJoystickFromGamepad_StickDeadzone(t *testing.T) {
	const dz = 0.3
	cases := []struct {
		name    string
		x, y    float64
		wantPos uint8
	}{
		{"stick sous deadzone Y négatif", 0.0, -0.29, 0xFF},
		{"stick au deadzone Y négatif", 0.0, -0.30, 0xFF}, // < strict, donc -0.30 reste neutre
		{"stick au-delà deadzone Y négatif", 0.0, -0.31, 0xFE},
		{"stick au-delà deadzone Y positif", 0.0, +0.31, 0xFD},
		{"stick au-delà deadzone X négatif", -0.31, 0.0, 0xFB},
		{"stick au-delà deadzone X positif", +0.31, 0.0, 0xF7},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			snap := GamepadSnapshot{Connected: true, LeftStickX: c.x, LeftStickY: c.y}
			got := JoystickFromGamepad(snap, dz, PlayerOne)
			want := machine.JoystickInput{Position: c.wantPos, Action: 0xC0}
			if got != want {
				t.Errorf("%s = %+v, want %+v", c.name, got, want)
			}
		})
	}
}

// TestJoystickFromGamepad_DPadOrStick (J4a.T5) : DPad OR stick — si l'une des
// deux sources indique la direction, le bit est posé. Cas mixte typique :
// utilisateur qui mélange DPad + petits mouvements stick (ex. Switch Pro).
func TestJoystickFromGamepad_DPadOrStick(t *testing.T) {
	// DPad up + stick neutre → bit nord posé.
	snap := GamepadSnapshot{Connected: true, DPadUp: true}
	got := JoystickFromGamepad(snap, 0.3, PlayerOne)
	want := machine.JoystickInput{Position: 0xFE, Action: 0xC0}
	if got != want {
		t.Errorf("DPad up seul = %+v, want %+v", got, want)
	}

	// DPad neutre + stick au-delà deadzone → bit nord posé.
	snap = GamepadSnapshot{Connected: true, LeftStickY: -0.8}
	got = JoystickFromGamepad(snap, 0.3, PlayerOne)
	if got != want {
		t.Errorf("stick up seul = %+v, want %+v", got, want)
	}

	// DPad up + stick up (redondant) → bit nord posé, pas posé deux fois.
	snap = GamepadSnapshot{Connected: true, DPadUp: true, LeftStickY: -0.8}
	got = JoystickFromGamepad(snap, 0.3, PlayerOne)
	if got != want {
		t.Errorf("DPad + stick up (OR) = %+v, want %+v", got, want)
	}
}

// TestJoystickFromGamepad_OpposingDirectionsCancel (J4a.T6) : J1 nord + J1 sud
// simultanés via DPad → repos (cohérent J2b uimodel.JoystickFromKeys). Idem
// pour stick analogique poussé sur axe avec valeur exacte 0 (mais opposés
// d'axes différents par DPad).
func TestJoystickFromGamepad_OpposingDirectionsCancel(t *testing.T) {
	cases := []struct {
		name    string
		snap    GamepadSnapshot
		wantPos uint8
	}{
		// DPad opposés s'annulent.
		{"DPad up + down", GamepadSnapshot{Connected: true, DPadUp: true, DPadDown: true}, 0xFF},
		{"DPad left + right", GamepadSnapshot{Connected: true, DPadLeft: true, DPadRight: true}, 0xFF},
		// DPad up combiné avec stick down (= opposés via sources différentes) : s'annulent aussi.
		{"DPad up + stick down", GamepadSnapshot{Connected: true, DPadUp: true, LeftStickY: 0.8}, 0xFF},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := JoystickFromGamepad(c.snap, 0.3, PlayerOne)
			want := machine.JoystickInput{Position: c.wantPos, Action: 0xC0}
			if got != want {
				t.Errorf("%s = %+v, want %+v", c.name, got, want)
			}
		})
	}
}

// TestJoystickFromGamepad_FireOR (J4a.T7) : FireA OR FireB. Quelle que soit la
// convention du gamepad (Xbox A = bas, Switch Pro A = droite, PS ✕ = bas),
// l'utilisateur appuie sur l'un des deux boutons frontaux et le bit fire est
// posé. Test des 4 combinaisons (00, 01, 10, 11).
func TestJoystickFromGamepad_FireOR(t *testing.T) {
	cases := []struct {
		fireA, fireB bool
		wantAct      uint8
	}{
		{false, false, 0xC0}, // repos
		{true, false, 0x80},  // A seul
		{false, true, 0x80},  // B seul
		{true, true, 0x80},   // A et B (idempotent)
	}
	for _, c := range cases {
		t.Run("", func(t *testing.T) {
			snap := GamepadSnapshot{Connected: true, FireA: c.fireA, FireB: c.fireB}
			got := JoystickFromGamepad(snap, 0.3, PlayerOne)
			want := machine.JoystickInput{Position: 0xFF, Action: c.wantAct}
			if got != want {
				t.Errorf("A=%v B=%v: got %+v, want %+v", c.fireA, c.fireB, got, want)
			}
		})
	}
}

// TestJoystickFromGamepad_InvalidPlayer (J4a.T8) : un player ≠ 1 et ≠ 2
// produit NeutralJoystick (aucun bit posé). Garde-fou : si l'app passe un
// slot vide encodé avec une valeur invalide, le snapshot ne fuit pas.
func TestJoystickFromGamepad_InvalidPlayer(t *testing.T) {
	snap := GamepadSnapshot{Connected: true, DPadUp: true, FireA: true}
	for _, p := range []int{0, 3, -1, 99} {
		got := JoystickFromGamepad(snap, 0.3, p)
		if got != machine.NeutralJoystick {
			t.Errorf("player=%d: got %+v, want NeutralJoystick (player invalide)", p, got)
		}
	}
}

// TestJoystickFromGamepad_DeadzoneAtOne (J4a.T9) : deadzone ≥ 1.0 désactive
// l'axe stick (jamais déclenché). Le DPad reste actif. Cas limite utile pour
// un utilisateur qui veut forcer le DPad-only (config future).
func TestJoystickFromGamepad_DeadzoneAtOne(t *testing.T) {
	// Stick poussé à fond mais deadzone = 1.0 → pas déclenché.
	snap := GamepadSnapshot{Connected: true, LeftStickX: -0.99, LeftStickY: -0.99}
	got := JoystickFromGamepad(snap, 1.0, PlayerOne)
	if got != machine.NeutralJoystick {
		t.Errorf("deadzone=1.0 et stick=-0.99: got %+v, want NeutralJoystick", got)
	}
	// DPad reste actif (le deadzone ne le concerne pas).
	snap.DPadUp = true
	got = JoystickFromGamepad(snap, 1.0, PlayerOne)
	want := machine.JoystickInput{Position: 0xFE, Action: 0xC0}
	if got != want {
		t.Errorf("deadzone=1.0 + DPad up: got %+v, want %+v", got, want)
	}
}
