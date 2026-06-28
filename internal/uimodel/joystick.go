// joystick.go — Inc J2b du support joystick : mapping PUR clavier → machine.
// JoystickInput. Logique commune MO5/TO8D, testable en CI sans Ebitengine.
//
// Le paquet uimodel est volontairement SANS DÉPENDANCE Ebitengine (cf. règle
// projet) : on manipule des KeyCode (int neutre) que la couche app convertit
// depuis ebiten.Key par un simple `int(key)`. Cela permet de tester ce mapping
// rigoureusement en CI, sans avoir à mocker Ebitengine.
//
// La résolution suit la convention bits LOGIQUE INVERSÉE figée par J1b (ref C
// Joysemul) : 0 = appuyé, repos = machine.NeutralJoystick. Une direction
// pressée efface le bit correspondant via `&^= mask`. Les directions opposées
// (J1 nord + J1 sud par exemple) S'ANNULENT par construction : aucune des
// deux n'est posée, ce qui empêche le hardware Thomson de voir un état
// physiquement impossible et évite des comportements indéfinis dans les jeux.
package uimodel

import "github.com/Lesur-ai/dcmo5/internal/machine"

// KeyCode est un identifiant de touche hôte neutre (= int(ebiten.Key) côté app).
// Permet à uimodel de rester pur sans importer Ebitengine. La couche app fait
// le cast à l'appel ; le paquet uimodel ne suppose RIEN sur la signification
// physique de la valeur, juste qu'elle identifie une touche de manière unique.
type KeyCode int

// KeyboardJoystickMapping mappe les touches hôte vers les axes/boutons des
// deux manettes Thomson émulées. Chaque champ est une KeyCode (= int neutre,
// cast d'ebiten.Key côté app). Pour la convention bits machine.JoystickInput,
// voir machine/machine.go (logique inversée, repos = machine.NeutralJoystick).
type KeyboardJoystickMapping struct {
	J1Up, J1Down, J1Left, J1Right, J1Fire KeyCode
	J2Up, J2Down, J2Left, J2Right, J2Fire KeyCode
}

// Masques bits Position pour les directions (logique inversée : 0 = appuyé).
// L'ordre est figé par J1b (TestBus_Joystick_BitConvention_Inverted +
// TestSetJoystick_BitConvention_TO8D, ref C Joysemul cases 0-9).
const (
	joyPosJ1Up    = 0x01
	joyPosJ1Down  = 0x02
	joyPosJ1Left  = 0x04
	joyPosJ1Right = 0x08
	joyPosJ2Up    = 0x10
	joyPosJ2Down  = 0x20
	joyPosJ2Left  = 0x40
	joyPosJ2Right = 0x80

	joyActJ1Fire = 0x40
	joyActJ2Fire = 0x80
)

// JoystickFromKeys traduit l'état clavier (lu via la fonction pressed fournie
// par l'appelant) en machine.JoystickInput selon la mapping fournie. Pure
// (pas d'effet de bord), totalement testable en CI.
//
// Règle des directions opposées : si J1Up ET J1Down sont pressées simultanément,
// AUCUNE n'est posée dans le résultat (le hardware Thomson ne peut pas voir
// les deux : c'est physiquement impossible sur un joystick analogique réel).
// Idem J1Left/J1Right, J2Up/J2Down, J2Left/J2Right. Sans cette règle, certains
// jeux qui ne testent que « est-ce que nord est appuyé ? » prendraient un état
// arbitraire et déclencheraient des bugs cosmétiques (sprite hésitant).
func JoystickFromKeys(m KeyboardJoystickMapping, pressed func(KeyCode) bool) machine.JoystickInput {
	pos := uint8(0xFF)
	act := uint8(0xC0)

	// Directions J1 — directions opposées s'annulent.
	up, down := pressed(m.J1Up), pressed(m.J1Down)
	left, right := pressed(m.J1Left), pressed(m.J1Right)
	if up && !down {
		pos &^= joyPosJ1Up
	}
	if down && !up {
		pos &^= joyPosJ1Down
	}
	if left && !right {
		pos &^= joyPosJ1Left
	}
	if right && !left {
		pos &^= joyPosJ1Right
	}

	// Directions J2 — idem.
	up2, down2 := pressed(m.J2Up), pressed(m.J2Down)
	left2, right2 := pressed(m.J2Left), pressed(m.J2Right)
	if up2 && !down2 {
		pos &^= joyPosJ2Up
	}
	if down2 && !up2 {
		pos &^= joyPosJ2Down
	}
	if left2 && !right2 {
		pos &^= joyPosJ2Left
	}
	if right2 && !left2 {
		pos &^= joyPosJ2Right
	}

	// Boutons fire — pas de notion d'opposé.
	if pressed(m.J1Fire) {
		act &^= joyActJ1Fire
	}
	if pressed(m.J2Fire) {
		act &^= joyActJ2Fire
	}

	return machine.JoystickInput{Position: pos, Action: act}
}

// ReservedKeys retourne la liste des touches utilisées par la mapping joystick.
// Permet à la couche app d'EXCLURE ces touches du clavier émulation (cf. règle
// D5 du plan workflow joystick : éviter que ZQSD côté joystick J2 tapent des
// caractères en BASIC simultanément). Les valeurs duplicates (rare en pratique)
// sont conservées telles quelles — l'appelant peut faire un set s'il préfère.
func ReservedKeys(m KeyboardJoystickMapping) []KeyCode {
	return []KeyCode{
		m.J1Up, m.J1Down, m.J1Left, m.J1Right, m.J1Fire,
		m.J2Up, m.J2Down, m.J2Left, m.J2Right, m.J2Fire,
	}
}
