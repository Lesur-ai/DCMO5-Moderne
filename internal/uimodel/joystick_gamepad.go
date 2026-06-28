// joystick_gamepad.go — Inc J4a du support joystick : mapping PUR gamepad
// matériel → machine.JoystickInput. Logique commune MO5/TO8D et indépendante
// de la machine, testable en CI sans Ebitengine.
//
// Le paquet uimodel reste SANS DÉPENDANCE Ebitengine (cf. règle projet) : la
// glue Ebitengine (collectGamepadSnapshots, mapping API ebiten.* → snapshot)
// vit dans internal/app/gamepad.go (Inc J4b). Ici on ne manipule que des
// types neutres : GamepadSnapshot publié par l'app, deadzone numérique,
// numéro de joueur (1 ou 2). Cela permet de tester rigoureusement le calcul
// en CI sans avoir besoin d'un gamepad physique.
//
// Composition (D5, D7 plan workflow joystick) :
//   - DPad et stick analogique font un OR (la direction est appuyée si l'une
//     des deux sources l'indique). Permet à un utilisateur d'utiliser au choix
//     la croix directionnelle ou le stick gauche du gamepad.
//   - Deadzone configurable (0.3 par défaut côté app — cf. plan B7).
//   - Directions opposées s'annulent (cohérent J2b : pas d'état physiquement
//     impossible sur un joystick rétro).
//   - Bouton fire = A OR B (couvre Xbox A, PlayStation ✕ et tolère les
//     conventions Switch Pro inversées).
package uimodel

import "github.com/Lesur-ai/dcmo5/internal/machine"

// GamepadSnapshot représente l'état instantané d'un gamepad publié par la
// couche app (qui lit Ebitengine et NORMALISE) vers uimodel. Champs neutres :
// pas de référence à ebiten.StandardGamepad*, pas de constantes Ebitengine.
// L'app remplit ce snapshot en lisant les API standard layout.
//
// Connected = false signifie « slot vide » : JoystickFromGamepad retourne
// alors NeutralJoystick (aucun bit posé). Garde-fou contre un gamepad
// déconnecté en cours de partie : le joueur revient au repos plutôt que de
// rester sur la dernière direction.
type GamepadSnapshot struct {
	Connected bool // false = slot vide → JoystickFromGamepad ignore

	// DPad — quatre directions discrètes (croix directionnelle gamepad).
	DPadUp, DPadDown, DPadLeft, DPadRight bool

	// LeftStickX, LeftStickY — stick analogique gauche, normalisé [-1.0, +1.0].
	// Convention Ebitengine standard layout : Y négatif = haut, Y positif = bas.
	LeftStickX, LeftStickY float64

	// FireA, FireB — boutons A et B standard. OR'és pour produire le bit fire
	// du joystick Thomson (1 seul bouton). Couvre Xbox A/B, PlayStation ✕/○,
	// Switch Pro (A/B sont swap mais l'OR rend ça indifférent).
	FireA, FireB bool
}

// Joueurs (J1 / J2) côté Thomson. Détermine quel ensemble de bits est posé
// dans machine.JoystickInput (cf. J1b convention figée).
const (
	PlayerOne = 1
	PlayerTwo = 2
)

// playerMask retourne les masques (position, action) à éteindre pour chaque
// direction et le bouton fire du joueur p. Si p est invalide (≠ 1, ≠ 2),
// retourne des masques nuls — JoystickFromGamepad ne posera alors aucun bit
// (sécurité).
func playerMask(p int) (up, down, left, right, fire uint8) {
	switch p {
	case PlayerOne:
		return 0x01, 0x02, 0x04, 0x08, 0x40
	case PlayerTwo:
		return 0x10, 0x20, 0x40, 0x80, 0x80
	default:
		return 0, 0, 0, 0, 0
	}
}

// JoystickFromGamepad traduit un snapshot gamepad (publié par la couche app
// depuis Ebitengine) en machine.JoystickInput pour le joueur indiqué (1 ou 2).
//
// Sémantique :
//   - Si snap.Connected == false → NeutralJoystick (slot vide).
//   - Direction = DPad OR (stick au-delà du deadzone). OR car l'utilisateur
//     choisit librement entre DPad et stick.
//   - Directions opposées (DPadUp + DPadDown par ex.) s'annulent → repos.
//   - Bouton fire = FireA OR FireB.
//
// deadzone est typiquement 0.3 (cf. B7 plan workflow) : sous ce seuil, le
// stick est considéré au repos, évitant le drift hardware. Si deadzone ≥ 1.0,
// le stick analogique ne déclenchera jamais — comportement valide (l'user
// devra utiliser le DPad).
func JoystickFromGamepad(snap GamepadSnapshot, deadzone float64, player int) machine.JoystickInput {
	if !snap.Connected {
		return machine.NeutralJoystick
	}

	upMask, downMask, leftMask, rightMask, fireMask := playerMask(player)
	pos := uint8(0xFF)
	act := uint8(0xC0)

	// Combine DPad + stick avec deadzone. Convention Ebitengine standard :
	// Y négatif = haut, Y positif = bas (l'origine est en haut-gauche du repère).
	up := snap.DPadUp || snap.LeftStickY < -deadzone
	down := snap.DPadDown || snap.LeftStickY > deadzone
	left := snap.DPadLeft || snap.LeftStickX < -deadzone
	right := snap.DPadRight || snap.LeftStickX > deadzone

	// Opposés s'annulent (cohérent avec uimodel.JoystickFromKeys, J2b).
	if up && !down {
		pos &^= upMask
	}
	if down && !up {
		pos &^= downMask
	}
	if left && !right {
		pos &^= leftMask
	}
	if right && !left {
		pos &^= rightMask
	}

	if snap.FireA || snap.FireB {
		act &^= fireMask
	}

	return machine.JoystickInput{Position: pos, Action: act}
}
