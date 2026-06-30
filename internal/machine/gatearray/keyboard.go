// Fichier : keyboard.go — claviers gate-array TO8D et TO9+.
//
// Référence : dcto8demulation.c TO8key() (l.134-164) et dcto8dkeyb.c (table des
// scancodes, KEYBOARDKEY_MAX = 84). Sur le FRONT d'appui d'une touche
// alphanumérique (scancode ≤ 0x4F), le scancode (+ bit SHIFT 0x80) est écrit à
// l'offset FIXE 0x30F8 du moniteur système (banque 1), l'indicateur CTRL en
// 0x3125, le bit 0 de E7C8 est posé et l'IRQ clavier (CP1) levée via
// TriggerKeyboardIRQ (lot #114). CAPSLOCK (touche 0x50) force le bit 0x80 sur les
// 26 lettres. L'acquittement (E7C3 bit 0x20 effacé) est déjà géré (lot #114).
// La variante TO9+ reprend les mêmes indices physiques mais publie directement
// un code ASCII via E7DE/E7DF, d'après dcto9pemulation.c TO9key().
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
	characterMax            int
	capsLockKey             int
	shiftKeys               []int
	ctrlKey                 int
	clearPendingOnASCIIRead bool
	handlePress             func(g *GateArray, key int, shiftPressed bool, ctrlPressed bool)
}

var to8dKeyboardDef = keyboardDef{
	characterMax: 0x4f,
	capsLockKey:  0x50,
	shiftKeys:    []int{0x51, 0x52},
	ctrlKey:      0x53,
	handlePress:  (*GateArray).handleTO8DKeyPress,
}

var to9pKeyboardDef = keyboardDef{
	characterMax:            0x4f,
	capsLockKey:             0x50,
	shiftKeys:               []int{0x51, 0x52},
	ctrlKey:                 0x53,
	clearPendingOnASCIIRead: true,
	handlePress:             (*GateArray).handleTO9PKeyPress,
}

// to9pASCII reproduit dcto9/source/dcto9pemulation.c to9key[0xa0] :
// conversion d'un indice de touche TO8-style vers le code ASCII TO9/TO9+.
// Les 0x50 premières entrées sont non shiftées, les 0x50 suivantes shiftées.
var to9pASCII = [0xa0]byte{
	0x91, 0x5f, 0x79, 0x68, 0x0b, 0x09, 0x1e, 0x6e,
	0x92, 0x28, 0x74, 0x67, 0x3d, 0x08, 0x1c, 0x62,
	0x93, 0x27, 0x72, 0x66, 0x16, 0x31, 0x1d, 0x76,
	0x94, 0x22, 0x65, 0x64, 0x37, 0x34, 0x30, 0x63,
	0x90, 0x80, 0x7a, 0x73, 0x38, 0x32, 0x2e, 0x78,
	0x23, 0x2a, 0x61, 0x71, 0x5b, 0x35, 0x36, 0x77,
	0x02, 0x81, 0x75, 0x6a, 0x20, 0x39, 0x0d, 0x2c,
	0xb0, 0x21, 0x69, 0x6b, 0x24, 0x0a, 0x5d, 0x3b,
	0xb7, 0x82, 0x6f, 0x6c, 0x2d, 0x84, 0x0d, 0x3a,
	0xb3, 0x83, 0x70, 0x6d, 0x29, 0x5e, 0x33, 0x3e,

	0x96, 0x36, 0x59, 0x48, 0x0b, 0x09, 0x0c, 0x4e,
	0x97, 0x35, 0x54, 0x47, 0x2b, 0x08, 0x1c, 0x42,
	0x98, 0x34, 0x52, 0x46, 0x16, 0x31, 0x7f, 0x56,
	0x99, 0x33, 0x45, 0x44, 0x37, 0x34, 0x30, 0x43,
	0x95, 0x32, 0x5a, 0x53, 0x38, 0x32, 0x2e, 0x58,
	0x40, 0x31, 0x41, 0x51, 0x7b, 0x35, 0x36, 0x57,
	0x03, 0x37, 0x55, 0x4a, 0x20, 0x39, 0x0d, 0x3f,
	0xb0, 0x38, 0x49, 0x4b, 0x26, 0x0a, 0x7d, 0x2e,
	0xb8, 0x39, 0x4f, 0x4c, 0x5c, 0x25, 0x0d, 0x2f,
	0xb3, 0x30, 0x50, 0x4d, 0x86, 0x85, 0x33, 0x3c,
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

// handleTO9PKeyPress reproduit DCTO9P TO9key() : l'appui publie directement un
// code ASCII TO9/TO9+ dans E7DF et arme E7DE, sans IRQ clavier 6846.
func (g *GateArray) handleTO9PKeyPress(n int, shiftPressed bool, ctrlPressed bool) {
	idx := n
	if shiftPressed || (g.capslock && isTO8DLetter(n)) {
		idx += 0x50
	}
	ascii := to9pASCII[idx]
	if ctrlPressed {
		if ascii == 23 {
			ascii = 0
		}
		if ascii > 0x3f && ascii < 0x60 {
			ascii -= 0x40
		}
		if ascii > 0x60 && ascii < 0x80 {
			ascii -= 0x60
		}
	}
	g.port[0x08] = 0x01  // E7C8 bit0 = 1 : touche enfoncée
	g.port[0x1e] = 0x01  // E7DE : touche en attente
	g.port[0x1f] = ascii // E7DF : code ASCII TO9/TO9+
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
