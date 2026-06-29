// Fichier : keyboard.go — clavier gate-array (TO8D aujourd'hui, TO9+ plus tard).
//
// Référence : dcto8demulation.c TO8key() (l.134-164) et dcto8dkeyb.c (table des
// scancodes, KEYBOARDKEY_MAX = 84). Sur le FRONT d'appui d'une touche
// alphanumérique (scancode ≤ 0x4F), le scancode (+ bit SHIFT 0x80) est écrit à
// l'offset FIXE 0x30F8 du moniteur système (banque 1), l'indicateur CTRL en
// 0x3125, le bit 0 de E7C8 est posé et l'IRQ clavier (CP1) levée via
// TriggerKeyboardIRQ (lot #114). CAPSLOCK (touche 0x50) force le bit 0x80 sur les
// 26 lettres. L'acquittement (E7C3 bit 0x20 effacé) est déjà géré (lot #114).
//
// CONTRAT D'ORDRE (modèle idempotent) : SetKey ne déclenche le traitement clavier
// qu'au FRONT (changement d'état), fidèlement à l'événement discret de la réf C.
// Le caller (host / adaptateur machine, #118) DOIT appliquer les transitions des
// touches modificatrices (SHIFT 0x51/0x52, CNT 0x53) AVANT les touches-caractère
// d'une même frame, sinon le bit SHIFT/CTRL du scancode reflète un état partiel.

package gatearray

// keyboardKeyMax est le nombre de touches des claviers gate-array TO8D/TO9+
// (réf C KEYBOARDKEY_MAX / TO9PKEY_MAX).
const keyboardKeyMax = 84

type keyboardDef struct {
	characterMax int
	capsLockKey  int
	shiftKeys    []int
	ctrlKey      int
	handlePress  func(g *GateArray, key int, shiftPressed bool, ctrlPressed bool)
}

var to8dKeyboardDef = keyboardDef{
	characterMax: 0x4f,
	capsLockKey:  0x50,
	shiftKeys:    []int{0x51, 0x52},
	ctrlKey:      0x53,
	handlePress:  (*GateArray).handleTO8DKeyPress,
}

// SetKey applique l'état idempotent d'une touche gate-array (k dans
// [0, keyboardKeyMax)). Elle ne déclenche le traitement matériel qu'au
// FRONT, c.-à-d. quand l'état change réellement (le modèle hôte réapplique l'état
// à chaque frame ; la réf C, elle, reçoit des événements discrets).
func (g *GateArray) SetKey(k int, pressed bool) {
	if k < 0 || k >= len(g.touche) {
		return
	}
	var state byte = 0x80 // relâchée
	if pressed {
		state = 0x00 // enfoncée
	}
	if g.touche[k] == state {
		return // pas de transition : ne pas rejouer le front
	}
	g.touche[k] = state
	g.handleKeyTransition(k)
}

// handleKeyTransition applique la partie commune des claviers gate-array : front,
// relâchement global, capslock et calcul des modificateurs. La publication
// matérielle de la touche pressée est déléguée à la définition de clavier.
func (g *GateArray) handleKeyTransition(n int) {
	def := g.keyboard
	if g.touche[n] != 0 { // touche relâchée (0x80)
		for i := 0; i <= def.characterMax && i < len(g.touche); i++ {
			if g.touche[i] == 0 { // une touche alphanumérique reste enfoncée
				return
			}
		}
		g.port[0x08] = 0x00 // E7C8 bit0 = 0 : toutes les touches relâchées
		g.keybIRQCount = 0
		return
	}
	// touche enfoncée (touche[n] == 0x00)
	if n == def.capsLockKey { // CAPSLOCK : bascule
		g.capslock = !g.capslock
	}
	if n > def.characterMax { // SHIFT / CNT / joysticks / capslock : pas de code caractère
		return
	}
	if def.handlePress == nil {
		return
	}
	def.handlePress(g, n, g.anyKeyPressed(def.shiftKeys), g.keyPressed(def.ctrlKey))
}

func (g *GateArray) keyPressed(k int) bool {
	return k >= 0 && k < len(g.touche) && g.touche[k] == 0
}

func (g *GateArray) anyKeyPressed(keys []int) bool {
	for _, k := range keys {
		if g.keyPressed(k) {
			return true
		}
	}
	return false
}

// handleTO8DKeyPress reproduit la publication TO8key() (dcto8demulation.c:134-164).
// Sur appui d'une touche ≤ 0x4F, écrit scancode + bit SHIFT à l'offset FIXE
// 0x30F8 du moniteur, l'indicateur CTRL en 0x3125, pose E7C8 bit0 et lève l'IRQ
// clavier.
func (g *GateArray) handleTO8DKeyPress(n int, shiftPressed bool, ctrlPressed bool) {
	var shift byte
	if shiftPressed {
		shift = 0x80
	}
	if g.capslock && isTO8DLetter(n) { // capslock force la majuscule sur les 26 lettres
		shift = 0x80
	}
	g.romMon[0x30f8] = byte(n) | shift // scancode + indicateur SHIFT (offset FIXE banque 1)
	if ctrlPressed {
		g.romMon[0x3125] = 1
	} else {
		g.romMon[0x3125] = 0
	}
	g.port[0x08] |= 0x01   // E7C8 bit0 = 1 : touche enfoncée
	g.TriggerKeyboardIRQ() // port[0x00] |= 0x82 (CP1) + keybIRQCount (réf C : 500000)
}

// isTO8DLetter indique si le scancode est l'une des 26 lettres affectées par le
// CAPSLOCK (réf C dcto8demulation.c:153-156).
func isTO8DLetter(n int) bool {
	switch n {
	case 0x02, 0x03, 0x07, 0x0a, 0x0b, 0x0f, 0x12, 0x13, 0x17, 0x1a, 0x1b, 0x1f,
		0x22, 0x23, 0x27, 0x2a, 0x2b, 0x2f, 0x32, 0x33, 0x3a, 0x3b, 0x42, 0x43,
		0x4a, 0x4b:
		return true
	}
	return false
}
