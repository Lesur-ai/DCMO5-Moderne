// keyboard_init.go — Inc Ka du fix clavier TO8D : peuple
// keyboard.Model.SpecialKeys pour chaque machine enregistrée, en convertissant
// les ebiten.Key en int neutres consommables par le paquet `keyboard` (qui
// reste sans dépendance ebiten). Remplace la var globale `keyMapping` figée
// MO5 par une table PAR MACHINE, lue par learnLiveKeys et resolveKeys via
// `a.kbModel.SpecialKeys`. L'init() s'exécute au load du paquet `app`, avant
// New()/Run() — la table est donc disponible dès la première frame.
//
// Touches CARACTÈRE (lettres, chiffres, ponctuation) absentes ici : elles
// passent par CharToKey + l'injecteur, qui respecte le layout OS. Mapper une
// lettre en positionnel casserait l'indépendance de layout (AZERTY→QWERTY).
package app

import (
	"github.com/hajimehoshi/ebiten/v2"

	"github.com/Lesur-ai/dcmo5/internal/keyboard"
)

// mo5SpecialKeys : touches positionnelles du clavier MO5. 18 entrées,
// transférées VERBATIM depuis l'ancienne var keyMapping (app.go pré-Ka).
// Ref : dcmo5keyb.h mo5key[] (indices 0x00–0x39).
var mo5SpecialKeys = map[int]int{
	int(ebiten.KeySpace):        0x20, // ESPACE
	int(ebiten.KeyEnter):        0x34, // ENT
	int(ebiten.KeyBackspace):    0x01, // EFF (effacement)
	int(ebiten.KeyInsert):       0x09, // INS
	int(ebiten.KeyDelete):       0x33, // RAZ
	int(ebiten.KeyHome):         0x11, // [retour]
	int(ebiten.KeyArrowRight):   0x19,
	int(ebiten.KeyArrowLeft):    0x29,
	int(ebiten.KeyArrowDown):    0x21,
	int(ebiten.KeyArrowUp):      0x31,
	int(ebiten.KeyShiftLeft):    0x38, // SHIFT
	int(ebiten.KeyShiftRight):   0x38,
	int(ebiten.KeyControlLeft):  0x35, // CNT
	int(ebiten.KeyControlRight): 0x35,
	int(ebiten.KeyAltLeft):      0x36, // ACC (accent)
	int(ebiten.KeyAltRight):     0x36,
	int(ebiten.KeyTab):          0x39, // BASIC
	int(ebiten.KeyEnd):          0x37, // STP (stop)
}

// to8dSpecialKeys : touches positionnelles du clavier TO8D. Indices issus de
// dcto8d200905/dcto8dkeyb.h (table pckeycode[], commentaires de label en
// regard de chaque indice TO8D). Décisions owner 28/06 :
//   - chiffres + symboles AZERTY : non listés ici (passent par charToTO8D, Inc Kb)
//   - LCK (0x50) non câblé v1 : KeyCapsLock n'est pas un event toggle Ebitengine
//   - ACC positionnel sur AltLeft (cohérence MO5), pas de ^¨ rune
//   - numpad PC mappé sur numpad TO8D (KeyKPEnter=0x36 ≠ KeyEnter=0x46)
//   - layout AZERTY France-Windows v1, autres layouts hors scope
var to8dSpecialKeys = map[int]int{
	// Modificateurs (cf. internal/keyboard/to8d.go to8dKey* — invariant que
	// to8dModel.ShiftKey/CNTKey/ACCKey pointent ici). Les DEUX shifts hôte (gauche
	// et droit) pointent volontairement sur le MÊME scancode 0x51 = Model.ShiftKey :
	// (1) c'est le seul indice retourné par ModifierKeys() côté shift, donc Host.tick
	// l'applique en 1ère passe (latching gate-array correct) ; (2) le clavier TO8D
	// physique a deux touches SHIFT (0x51, 0x52) au libellé identique « SHIFT »,
	// traitées de manière équivalente par le gate-array, donc la perte de distinction
	// gauche/droite est sans impact fonctionnel. Mapper KeyShiftRight sur 0x52
	// réintroduirait le bug d'ordre Kc (0x52 posé en 2e passe).
	int(ebiten.KeyShiftLeft):    0x51, // SHIFT (les deux KeyShift hôte → même scancode)
	int(ebiten.KeyShiftRight):   0x51,
	int(ebiten.KeyControlLeft):  0x53, // CNT
	int(ebiten.KeyControlRight): 0x53,
	int(ebiten.KeyAltLeft):      0x14, // ACC (touche morte d'accent)
	int(ebiten.KeyAltRight):     0x14,

	// Touches d'édition. ENT principale (0x46) ≠ Ent pad (0x36).
	int(ebiten.KeyEnter):     0x46, // ENTRÉE principale
	int(ebiten.KeySpace):     0x34, // ESPACE
	int(ebiten.KeyBackspace): 0x16, // EFF (effacement, cohérent MO5 KeyBackspace→EFF)
	int(ebiten.KeyDelete):    0x06, // RAZ (remise à zéro, cohérent MO5 KeyDelete→RAZ)
	int(ebiten.KeyInsert):    0x0e, // INS
	int(ebiten.KeyEnd):       0x30, // STOP (cohérent MO5 KeyEnd→STP)

	// Flèches.
	int(ebiten.KeyArrowUp):    0x04,
	int(ebiten.KeyArrowDown):  0x3d,
	int(ebiten.KeyArrowLeft):  0x0d,
	int(ebiten.KeyArrowRight): 0x05,

	// Touches de fonction TO8D (F1, F2, F4). F3/F5 SONT INTERCEPTÉES par App.Update
	// comme raccourcis globaux (pause/reset) AVANT resolveKeys : les mapper ici ne
	// les ferait pas atteindre la machine — au pire elles déclencheraient le shortcut
	// global. F6..F10 inexistantes sur TO8D physique.
	int(ebiten.KeyF1): 0x20,
	int(ebiten.KeyF2): 0x00,
	int(ebiten.KeyF4): 0x10,

	// Pavé numérique TO8D (13 touches : 0-9, ., Ent pad).
	int(ebiten.KeyKP0):       0x1e,
	int(ebiten.KeyKP1):       0x15,
	int(ebiten.KeyKP2):       0x25,
	int(ebiten.KeyKP3):       0x4e,
	int(ebiten.KeyKP4):       0x1d,
	int(ebiten.KeyKP5):       0x2d,
	int(ebiten.KeyKP6):       0x2e,
	int(ebiten.KeyKP7):       0x1c,
	int(ebiten.KeyKP8):       0x24,
	int(ebiten.KeyKP9):       0x35,
	int(ebiten.KeyKPDecimal): 0x26,
	int(ebiten.KeyKPEnter):   0x36, // Ent pad (≠ ENT principale 0x46)
}

// to9pSpecialKeys est separee de to8dSpecialKeys meme si les indices physiques
// coincident aujourd'hui : le gate-array TO9+ publie des codes ASCII sur E7DF,
// et ce modele doit pouvoir diverger sans toucher au TO8D.
var to9pSpecialKeys = map[int]int{
	int(ebiten.KeyShiftLeft):    0x51,
	int(ebiten.KeyShiftRight):   0x51,
	int(ebiten.KeyControlLeft):  0x53,
	int(ebiten.KeyControlRight): 0x53,
	int(ebiten.KeyAltLeft):      0x14,
	int(ebiten.KeyAltRight):     0x14,

	int(ebiten.KeyEnter):     0x46,
	int(ebiten.KeySpace):     0x34,
	int(ebiten.KeyBackspace): 0x16,
	int(ebiten.KeyDelete):    0x06,
	int(ebiten.KeyInsert):    0x0e,
	int(ebiten.KeyEnd):       0x30,

	int(ebiten.KeyArrowUp):    0x04,
	int(ebiten.KeyArrowDown):  0x3d,
	int(ebiten.KeyArrowLeft):  0x0d,
	int(ebiten.KeyArrowRight): 0x05,

	int(ebiten.KeyF1): 0x20,
	int(ebiten.KeyF2): 0x00,
	int(ebiten.KeyF4): 0x10,

	int(ebiten.KeyKP0):       0x1e,
	int(ebiten.KeyKP1):       0x15,
	int(ebiten.KeyKP2):       0x25,
	int(ebiten.KeyKP3):       0x4e,
	int(ebiten.KeyKP4):       0x1d,
	int(ebiten.KeyKP5):       0x2d,
	int(ebiten.KeyKP6):       0x2e,
	int(ebiten.KeyKP7):       0x1c,
	int(ebiten.KeyKP8):       0x24,
	int(ebiten.KeyKP9):       0x35,
	int(ebiten.KeyKPDecimal): 0x26,
	int(ebiten.KeyKPEnter):   0x36,
}

func init() {
	keyboard.MO5Model().SpecialKeys = mo5SpecialKeys
	keyboard.TO8DModel().SpecialKeys = to8dSpecialKeys
	keyboard.TO9PModel().SpecialKeys = to9pSpecialKeys
}
