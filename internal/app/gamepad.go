// gamepad.go — Inc J4b du support joystick : glue Ebitengine ↔ uimodel pour
// le gamepad MATÉRIEL. Lit l'état standard layout de jusqu'à 2 gamepads
// connectés, normalise en uimodel.GamepadSnapshot, et publie via le pipeline
// commun (MergeJoysticks(clavier, gamepad) → in.Joystick → Host.tick).
//
// Slot management (D8 plan workflow joystick) : 2 slots fixes (J1, J2). Une
// nouvelle connexion prend le premier slot libre (ordre de connexion). Une
// déconnexion vide le slot — le joueur revient au repos immédiatement, sans
// rester figé sur la dernière direction. Persistance par GUID reportée v2.
//
// Convention boutons (D7 + B6/B7 plan workflow) : standard layout Ebitengine
// (Xbox A/B = PS ✕/○ = Switch Pro A/B inversés). Le mapping FireA OR FireB
// rend la convention indifférente — l'utilisateur appuie sur l'un des deux
// boutons frontaux et le bit fire du joystick Thomson est posé. DPad et
// stick analogique gauche sont OR'és avec deadzone (cf. uimodel.JoystickFromGamepad).
//
// Permission macOS « Input Monitoring » : sur certains Mac récents, l'accès
// aux gamepads via SDL2 / Ebitengine peut nécessiter d'accepter une demande
// système. La première connexion d'un gamepad déclenche le prompt ; si
// l'utilisateur refuse, AppendGamepadIDs retournera une liste vide. Limitation
// documentée mais non détectable côté code.
package app

import (
	"github.com/hajimehoshi/ebiten/v2"

	"github.com/Lesur-ai/dcmo5/internal/machine"
	"github.com/Lesur-ai/dcmo5/internal/uimodel"
)

// gamepadDeadzone : seuil sous lequel le stick analogique est considéré au
// repos. 0.3 est le défaut conventionnel (B7 plan workflow) — couvre le
// drift hardware sans masquer les mouvements intentionnels. Configurable v2.
const gamepadDeadzone = 0.3

// gamepadSlot représente un slot J1 ou J2. id pointe sur l'ebiten.GamepadID
// occupant le slot, ou nil si le slot est vide. On utilise un pointeur pour
// distinguer « pas de gamepad » (nil) de « gamepad d'ID 0 » (ebiten.GamepadID
// est un int — la valeur 0 est valide).
type gamepadSlot struct {
	id *ebiten.GamepadID
}

// gamepadSlots : 2 slots fixes pour J1 (index 0) et J2 (index 1). Stocké
// dans App pour persister entre frames.
type gamepadSlots [2]gamepadSlot

// updateGamepadSlots réconcilie les 2 slots avec la liste courante des gamepads
// connectés (lue via ebiten.AppendGamepadIDs). À appeler une fois par tick
// AVANT gamepadSnapshot (cf. App.Update).
//
// Pourquoi réconciliation (et pas inpututil.AppendJustConnectedGamepadIDs) :
// JustConnected ne signale QUE les gamepads branchés APRÈS le démarrage du
// programme. Une manette déjà connectée au lancement n'apparaît jamais comme
// « just connected » → ne serait JAMAIS attribuée à un slot.
// La réconciliation comparée frame par frame résout ce cas tout en gérant
// hot-plug et déconnexion.
//
// Politique de placement (D8) :
//   - Pour chaque ID nouvellement présent : attribué au premier slot libre.
//   - Pour chaque ID d'un slot qui disparaît de la liste : slot libéré
//     (Connected=false → uimodel.JoystickFromGamepad retourne NeutralJoystick).
//   - Au-delà de 2 gamepads, les supplémentaires sont IGNORÉS jusqu'à
//     libération d'un slot.
func (a *App) updateGamepadSlots(idBuf []ebiten.GamepadID) []ebiten.GamepadID {
	idBuf = ebiten.AppendGamepadIDs(idBuf[:0])
	// Set des IDs connectés cette frame (pour O(1) lookup ci-dessous).
	connected := make(map[ebiten.GamepadID]bool, len(idBuf))
	for _, id := range idBuf {
		connected[id] = true
	}
	// 1. Libérer les slots dont l'ID n'est plus connecté ; mémoriser les slots
	//    qui restent occupés pour ne pas attribuer deux fois le même ID.
	occupied := make(map[ebiten.GamepadID]bool, len(a.gamepadSlots))
	for i := range a.gamepadSlots {
		s := &a.gamepadSlots[i]
		if s.id == nil {
			continue
		}
		if !connected[*s.id] {
			s.id = nil
		} else {
			occupied[*s.id] = true
		}
	}
	// 2. Attribuer chaque ID nouvellement présent au premier slot libre.
	for _, id := range idBuf {
		if occupied[id] {
			continue
		}
		idCopy := id // évite que tous les pointeurs partagent la dernière itération
		for i := range a.gamepadSlots {
			s := &a.gamepadSlots[i]
			if s.id == nil {
				s.id = &idCopy
				occupied[id] = true
				break
			}
		}
	}
	return idBuf
}

// gamepadSnapshot lit l'état du gamepad occupant le slot indiqué, normalisé
// en uimodel.GamepadSnapshot (pas de référence Ebitengine côté uimodel).
// Slot vide ou gamepad sans standard layout → Connected=false (le pipeline
// pur uimodel.JoystickFromGamepad retourne alors NeutralJoystick).
func (a *App) gamepadSnapshot(slotIdx int) uimodel.GamepadSnapshot {
	if slotIdx < 0 || slotIdx >= len(a.gamepadSlots) || a.gamepadSlots[slotIdx].id == nil {
		return uimodel.GamepadSnapshot{Connected: false}
	}
	id := *a.gamepadSlots[slotIdx].id
	// Hors standard layout (rare : gamepad très exotique sans mapping SDL2),
	// on signale Connected=true mais sans input. Évite de retomber sur le bas
	// niveau (axes/boutons indexés bruts) — fallback différé v2 (D7).
	if !ebiten.IsStandardGamepadLayoutAvailable(id) {
		return uimodel.GamepadSnapshot{Connected: true}
	}
	return uimodel.GamepadSnapshot{
		Connected:  true,
		DPadUp:     ebiten.IsStandardGamepadButtonPressed(id, ebiten.StandardGamepadButtonLeftTop),
		DPadDown:   ebiten.IsStandardGamepadButtonPressed(id, ebiten.StandardGamepadButtonLeftBottom),
		DPadLeft:   ebiten.IsStandardGamepadButtonPressed(id, ebiten.StandardGamepadButtonLeftLeft),
		DPadRight:  ebiten.IsStandardGamepadButtonPressed(id, ebiten.StandardGamepadButtonLeftRight),
		LeftStickX: ebiten.StandardGamepadAxisValue(id, ebiten.StandardGamepadAxisLeftStickHorizontal),
		LeftStickY: ebiten.StandardGamepadAxisValue(id, ebiten.StandardGamepadAxisLeftStickVertical),
		// Fire = bouton face « bas » (Xbox A, PS ✕) OR bouton face « droite »
		// (Xbox B, PS ○). Couvre Switch Pro A/B inversés sans avoir à le savoir.
		FireA: ebiten.IsStandardGamepadButtonPressed(id, ebiten.StandardGamepadButtonRightBottom),
		FireB: ebiten.IsStandardGamepadButtonPressed(id, ebiten.StandardGamepadButtonRightRight),
	}
}

// joystickFromGamepads compose l'état joystick venant des deux slots gamepad
// (J1 et J2 indépendants) en un seul machine.JoystickInput. Appelée par
// App.Update juste avant la composition avec le joystick clavier via
// uimodel.MergeJoysticks. Le gamepad publie EN PERMANENCE — il n'y a pas de
// toggle « ON/OFF » côté gamepad (contrairement au clavier J3a) : un gamepad
// connecté qui ne touche rien retourne NeutralJoystick, transparent à la
// composition.
func (a *App) joystickFromGamepads() machine.JoystickInput {
	j1 := uimodel.JoystickFromGamepad(a.gamepadSnapshot(0), gamepadDeadzone, uimodel.PlayerOne)
	j2 := uimodel.JoystickFromGamepad(a.gamepadSnapshot(1), gamepadDeadzone, uimodel.PlayerTwo)
	return uimodel.MergeJoysticks(j1, j2)
}
