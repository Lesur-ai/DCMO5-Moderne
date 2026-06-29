package uimodel

// joystick_merge.go — Inc J2c du support joystick : composition de plusieurs
// sources joystick (clavier J1+J2, gamepad J1, gamepad J2) en un seul
// machine.JoystickInput publié à la machine. Pur, testable en CI.
//
// Sémantique : OR LOGIQUE des appuis (en logique inversée bits, c'est un AND
// bitwise). Si une source pose un bit à 0 (= appuyé), le résultat l'a à 0. La
// machine ne sait pas qui appuie (gamepad ou clavier) et ne doit pas le savoir :
// elle voit l'union des entrées.
//
// Cas typique 2 joueurs :
//   - clavier publie : J1 nord (= flèche haut) → JoystickInput{0xFE, 0xC0}
//   - gamepad J2 publie : J2 fire             → JoystickInput{0xFF, 0x40}
//   - merge                                   → JoystickInput{0xFE, 0x40}
//
// La couche app (Inc J3a/J5a) appellera MergeJoysticks pour combiner ces
// sources avant Host.SetInput. Sans cette fonction la composition se ferait
// ad-hoc dans App.Update et serait impossible à tester en CI (cf. D4 plan).

import "github.com/Lesur-ai/dcmo5/internal/machine"

// MergeJoysticks combine deux états joystick (logique inversée : 0 = appuyé).
// Le résultat a un bit à 0 dès qu'AU MOINS UNE des deux sources l'a à 0 — c'est
// donc un AND bitwise des champs Position et Action. Repos = NeutralJoystick :
// MergeJoysticks(NeutralJoystick, NeutralJoystick) == NeutralJoystick.
//
// L'opération est commutative et associative (AND est commutatif/associatif),
// donc l'ordre des arguments est sans importance et on peut composer plus de
// deux sources par appels successifs (cf. tests).
//
// CHOIX DE DESIGN D5 (29/06/2026) — opposés inter-sources : si le clavier
// publie J1↑ et le gamepad publie J1↓, le résultat a les deux bits à 0 (↑+↓
// simultanés). Pas de cancel post-merge. Raisons : (1) cas réel quasi-nul
// (même joueur sur clavier ET gamepad en directions opposées) ; (2) fidélité
// matérielle (le hardware Thomson ne cancellait pas les opposés sur le port
// physique) ; (3) évite de la logique per-joueur dans le hot path.
func MergeJoysticks(a, b machine.JoystickInput) machine.JoystickInput {
	return machine.JoystickInput{
		Position: a.Position & b.Position,
		Action:   a.Action & b.Action,
	}
}
