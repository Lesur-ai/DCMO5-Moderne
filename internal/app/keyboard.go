// Fichier : keyboard.go — saisie clavier MO5 par caractère.
//
// Le clavier MO5 est une matrice scannée par la ROM. Le mapping positionnel
// (scancode physique → touche MO5) ignore le layout de l'OS et ne permet pas
// de saisir les caractères obtenus avec Shift (« " », « ? », « : »…).
//
// On distingue donc deux familles de touches :
//
//   - Touches « caractère » (lettres, chiffres, ponctuation) : saisies via les
//     caractères Unicode réellement produits par l'OS (ebiten.AppendInputChars),
//     ce qui respecte n'importe quel layout physique (AZERTY, QWERTY…). Chaque
//     caractère est traduit en (touche MO5 [+ SHIFT]) puis joué par un injecteur
//     qui maintient la pression assez longtemps pour que le scan ROM la voie.
//   - Touches « spéciales » (flèches, ENT, EFF, ESPACE, SHIFT, CNT…) : conservées
//     en mode positionnel (état physique continu), adapté au pilotage et aux jeux.
//
// Réf. table des touches : dcmo5keyb.h mo5key[] (libellés « touche / shift »).
package app

// charKey décrit la combinaison MO5 produisant un caractère donné.
type charKey struct {
	key   int  // index touche MO5 [0, spec.KeyMax)
	shift bool // true si la touche SHIFT (0x38) doit être maintenue
}

// SHIFT MO5
const mo5KeyShift = 0x38

// charToMO5 traduit un caractère imprimable en combinaison de touches MO5.
// Construit depuis les libellés de mo5key[] : pour « 2 \" », '2' donne la touche
// sans shift et '"' la même touche avec shift.
var charToMO5 = map[rune]charKey{
	// Lettres — le MO5 BASIC fonctionne en majuscules ; minuscules et majuscules
	// pointent vers la même touche (pas de SHIFT requis pour obtenir la lettre).
	'a': {0x2D, false}, 'A': {0x2D, false},
	'b': {0x22, false}, 'B': {0x22, false},
	'c': {0x32, false}, 'C': {0x32, false},
	'd': {0x1B, false}, 'D': {0x1B, false},
	'e': {0x1D, false}, 'E': {0x1D, false},
	'f': {0x13, false}, 'F': {0x13, false},
	'g': {0x0B, false}, 'G': {0x0B, false},
	'h': {0x03, false}, 'H': {0x03, false},
	'i': {0x0C, false}, 'I': {0x0C, false},
	'j': {0x02, false}, 'J': {0x02, false},
	'k': {0x0A, false}, 'K': {0x0A, false},
	'l': {0x12, false}, 'L': {0x12, false},
	'm': {0x1A, false}, 'M': {0x1A, false},
	'n': {0x00, false}, 'N': {0x00, false},
	'o': {0x14, false}, 'O': {0x14, false},
	'p': {0x1C, false}, 'P': {0x1C, false},
	'q': {0x2B, false}, 'Q': {0x2B, false},
	'r': {0x15, false}, 'R': {0x15, false},
	's': {0x23, false}, 'S': {0x23, false},
	't': {0x0D, false}, 'T': {0x0D, false},
	'u': {0x04, false}, 'U': {0x04, false},
	'v': {0x2A, false}, 'V': {0x2A, false},
	'w': {0x30, false}, 'W': {0x30, false},
	'x': {0x28, false}, 'X': {0x28, false},
	'y': {0x05, false}, 'Y': {0x05, false},
	'z': {0x25, false}, 'Z': {0x25, false},

	// Rangée des chiffres : caractère normal sans shift, symbole avec shift.
	'1': {0x2F, false}, '!': {0x2F, true},
	'2': {0x27, false}, '"': {0x27, true},
	'3': {0x1F, false}, '#': {0x1F, true},
	'4': {0x17, false}, '$': {0x17, true},
	'5': {0x0F, false}, '%': {0x0F, true},
	'6': {0x07, false}, '&': {0x07, true},
	'7': {0x06, false}, '\'': {0x06, true},
	'8': {0x0E, false}, '(': {0x0E, true},
	'9': {0x16, false}, ')': {0x16, true},
	'0': {0x1E, false}, '`': {0x1E, true},

	// Touches de ponctuation dédiées (libellé « base shift »).
	',': {0x08, false}, '<': {0x08, true},
	'.': {0x10, false}, '>': {0x10, true},
	'@': {0x18, false}, '^': {0x18, true},
	'/': {0x24, false}, '?': {0x24, true},
	'-': {0x26, false}, '=': {0x26, true},
	'*': {0x2C, false}, ':': {0x2C, true},
	'+': {0x2E, false}, ';': {0x2E, true},
}

// CharToMO5Key traduit un caractère en (touche MO5, shift). ok=false si le
// caractère n'a pas d'équivalent direct sur le clavier MO5. Exporté pour les tests.
func CharToMO5Key(r rune) (key int, shift bool, ok bool) {
	c, found := charToMO5[r]
	return c.key, c.shift, found
}

// Durées par défaut de l'injecteur, exprimées en frames (60 Hz).
// Ce ne sont pas des constantes « magiques » : elles règlent la cadence de
// rejeu des caractères et sont injectables via newKeyInjector pour les tests.
const (
	defaultKeyHoldFrames = 3 // maintien d'une frappe (scan ROM ~50 Hz)
	defaultKeyGapFrames  = 2 // relâchement entre deux frappes successives
)

// injectorPhase est l'état courant de l'injecteur de frappes.
type injectorPhase int

const (
	phaseIdle injectorPhase = iota // aucune frappe en cours
	phaseHold                      // touche maintenue pressée
	phaseGap                       // relâchement avant la frappe suivante
)

// keyInjector rejoue une file de frappes caractère par caractère : chaque frappe
// est maintenue holdFrames puis suivie d'un trou de gapFrames, pour que le scan
// clavier de la ROM distingue deux frappes identiques consécutives.
//
// Logique pure, sans dépendance Ebitengine : testable headless.
type keyInjector struct {
	queue      []charKey
	holdFrames int
	gapFrames  int

	phase   injectorPhase
	current charKey
	timer   int
}

// newKeyInjector crée un injecteur avec les durées fournies (frames).
func newKeyInjector(holdFrames, gapFrames int) *keyInjector {
	return &keyInjector{holdFrames: holdFrames, gapFrames: gapFrames}
}

// Enqueue ajoute le caractère à la file s'il a un équivalent MO5.
func (ki *keyInjector) Enqueue(r rune) {
	if key, shift, ok := CharToMO5Key(r); ok {
		ki.queue = append(ki.queue, charKey{key: key, shift: shift})
	}
}

// Pending retourne le nombre de frappes en attente (frappe courante incluse).
func (ki *keyInjector) Pending() int {
	n := len(ki.queue)
	if ki.phase != phaseIdle {
		n++
	}
	return n
}

// Tick avance d'une frame et retourne les touches MO5 à presser pendant cette
// frame (touche courante + SHIFT si nécessaire). Retourne nil pendant les trous
// et au repos.
func (ki *keyInjector) Tick() []int {
	if ki.phase == phaseIdle {
		if len(ki.queue) == 0 {
			return nil
		}
		ki.current = ki.queue[0]
		ki.queue = ki.queue[1:]
		ki.phase = phaseHold
		ki.timer = ki.holdFrames
	}

	switch ki.phase {
	case phaseHold:
		keys := []int{ki.current.key}
		if ki.current.shift {
			keys = append(keys, mo5KeyShift)
		}
		ki.timer--
		if ki.timer <= 0 {
			ki.phase = phaseGap
			ki.timer = ki.gapFrames
		}
		return keys
	case phaseGap:
		ki.timer--
		if ki.timer <= 0 {
			ki.phase = phaseIdle
		}
		return nil
	default:
		return nil
	}
}
