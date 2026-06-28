// joystick.go — Inc J3a du support joystick : glue Ebitengine ↔ uimodel pour
// l'émulation joystick au clavier. Touches mappées (validées owner 28/06,
// ajustement post-J3a v1) :
//
//	J1 = flèches ↑↓←→ + RightShift pour fire
//	J2 = WASD physique (KeyW/A/S/D — scancodes Ebitengine indépendants du
//	     layout, donc identiques aux ZQSD AZERTY visuellement) + LeftShift
//	     pour fire
//
// AltGr (KeyAltRight) avait été initialement choisi mais ne marche pas sur Mac
// AZERTY (touche absente ou non capturée par Ebitengine). RightShift est
// universel et symétrique de LeftShift J2 → cohérent et présent sur tous les
// claviers (PC + Mac).
//
// Convention bits machine.JoystickInput figée par J1b/J2b (logique inversée,
// repos = machine.NeutralJoystick).
//
// Mécanisme d'exclusion (D5 plan workflow joystick) :
//
//   - Flèches  : DOUBLE-INPUT accepté. ArrowUp etc. continuent d'activer les
//     touches MO5/TO8D via SpecialKeys (cf. keyboard_init.go) ET
//     publient simultanément les bits joystick J1. Conforme à la
//     ref C dcto8d en OR (vs keybpriority exclusif, reporté v2).
//   - LeftShift : DOUBLE-INPUT accepté (exception). Conserve sa fonction Shift
//     MO5/TO8D ET active le bit J2 fire — retirer Shift du clavier
//     serait trop intrusif (utilisateur ne peut plus shifter en
//     BASIC).
//   - WASD : EXCLUS du clavier émulation (= touches J2 directions). WASD ne
//     pollueront plus le BASIC (sinon Z/Q/S/D tapés en permanence à chaque
//     mouvement joystick J2).
//   - RightShift : DOUBLE-INPUT accepté (symétrique de LeftShift J2). Conserve
//     sa fonction Shift MO5/TO8D ET active le bit J1 fire en parallèle.
package app

import (
	"github.com/hajimehoshi/ebiten/v2"

	"github.com/Lesur-ai/dcmo5/internal/machine"
	"github.com/Lesur-ai/dcmo5/internal/uimodel"
)

// appJoystickMapping fixe la correspondance touche → axe/bouton joystick.
// Constante package-level : pas de configurabilité runtime en v1 (cf. D3 plan
// workflow). Un panneau visualisation est prévu en Inc J3a.5.
var appJoystickMapping = uimodel.KeyboardJoystickMapping{
	J1Up:    uimodel.KeyCode(ebiten.KeyArrowUp),
	J1Down:  uimodel.KeyCode(ebiten.KeyArrowDown),
	J1Left:  uimodel.KeyCode(ebiten.KeyArrowLeft),
	J1Right: uimodel.KeyCode(ebiten.KeyArrowRight),
	J1Fire:  uimodel.KeyCode(ebiten.KeyShiftRight),
	J2Up:    uimodel.KeyCode(ebiten.KeyW),
	J2Down:  uimodel.KeyCode(ebiten.KeyS),
	J2Left:  uimodel.KeyCode(ebiten.KeyA),
	J2Right: uimodel.KeyCode(ebiten.KeyD),
	J2Fire:  uimodel.KeyCode(ebiten.KeyShiftLeft),
}

// joystickExclusiveKeys : touches qui sont mappées au joystick ET DOIVENT
// être exclues du clavier émulation MO5/TO8D. Voir doc de fichier pour la
// règle complète (D5 plan workflow joystick).
//
// Ne contient PAS les flèches ni KeyShiftLeft/KeyShiftRight (double-input
// D5/exception). Seuls les WASD sont à exclure du clavier émulation.
var joystickExclusiveKeys = map[ebiten.Key]struct{}{
	ebiten.KeyW: {},
	ebiten.KeyA: {},
	ebiten.KeyS: {},
	ebiten.KeyD: {},
}

// isJoystickExclusiveKey indique si une touche est réservée au joystick et
// doit être ignorée par les chemins clavier émulation (learnLiveKeys +
// resolveKeys boucle SpecialKeys). N'a d'effet QUE si le mode joystick clavier
// est activé (toggle F12) — sinon retourne false pour toutes les touches afin
// de laisser WASD/AltGr taper normalement en BASIC.
func isJoystickExclusiveKey(k ebiten.Key, joystickKBEnabled bool) bool {
	if !joystickKBEnabled {
		return false
	}
	_, ok := joystickExclusiveKeys[k]
	return ok
}

// joystickFromKeys appelle uimodel.JoystickFromKeys avec le mapping fixe de
// l'app et la fonction de presse Ebitengine. Retourne machine.NeutralJoystick
// si le mode joystick clavier est désactivé (toggle F12) — auquel cas l'état
// reste neutre, indépendamment de l'état des touches du clavier physique.
func joystickFromKeys(pressed func(ebiten.Key) bool, joystickKBEnabled bool) machine.JoystickInput {
	if !joystickKBEnabled {
		return machine.NeutralJoystick
	}
	return uimodel.JoystickFromKeys(appJoystickMapping, func(kc uimodel.KeyCode) bool {
		return pressed(ebiten.Key(kc))
	})
}
