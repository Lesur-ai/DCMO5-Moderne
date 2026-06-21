// Package keyboard traduit la saisie clavier de l'hôte en frappes MO5.
//
// Le clavier MO5 est une matrice scannée par la ROM. Plutôt qu'un mapping
// positionnel (scancode physique → touche MO5), qui ignore le layout de l'OS et
// ne permet pas les caractères obtenus avec Shift (« " », « ? », « : »…), on
// traduit les caractères Unicode réellement produits par l'OS en combinaisons
// MO5 (touche [+ SHIFT]). Un injecteur rejoue ces frappes une par une, en
// maintenant chaque touche assez longtemps pour que le scan ROM la voie.
//
// Logique pure (sans dépendance graphique) : testable headless.
// Réf. table : dcmo5keyb.h mo5key[] (libellés « touche / shift »).
package keyboard

// Index de touches MO5 utiles à l'injecteur.
const (
	Mo5KeyShift = 0x38 // SHIFT
	Mo5KeyCNT   = 0x35 // CONTROL (CNT)
	Mo5KeyENT   = 0x34 // ENTRÉE (validation de ligne)
)

// Cadences par défaut de l'injecteur (en frames à 60 Hz) et borne de file.
const (
	DefaultHoldFrames = 4   // maintien d'une frappe (scan ROM ~50 Hz)
	DefaultGapFrames  = 3   // relâchement entre deux frappes successives
	DefaultQueueMax   = 256 // borne la file (collage massif / répétition OS)
	// enterGapFrames : relâchement APRÈS un ENTRÉE. Plus long, car le BASIC
	// traite la ligne saisie (tokenisation) et ne lit pas le clavier pendant ce
	// temps : sans cette pause, le premier caractère de la ligne suivante est
	// avalé (« 10 CLS » → « 0 CLS »).
	enterGapFrames = 16
)

// charKey décrit la combinaison MO5 produisant un caractère donné.
type charKey struct {
	key   int  // index de touche (borné par Model.KeyCount)
	shift bool // true si SHIFT doit être maintenu
}

// charToMO5 traduit un caractère en combinaison de touches MO5. Construit depuis
// les libellés de mo5key[] : pour « 2 \" », '2' donne la touche sans shift et
// '"' la même touche avec shift.
var charToMO5 = map[rune]charKey{
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

	',': {0x08, false}, '<': {0x08, true},
	'.': {0x10, false}, '>': {0x10, true},
	'@': {0x18, false}, '^': {0x18, true},
	'/': {0x24, false}, '?': {0x24, true},
	'-': {0x26, false}, '=': {0x26, true},
	'*': {0x2C, false}, ':': {0x2C, true},
	'+': {0x2E, false}, ';': {0x2E, true},

	// Espace et retour-chariot : utiles pour les séquences (--exec, coller).
	' ':  {0x20, false}, // ESPACE
	'\n': {0x34, false}, // ENT (retour-chariot)
	'\r': {0x34, false}, // ENT (CRLF → un seul ENT, voir EnqueueString)
}

// CharToMO5Key traduit un caractère en (touche MO5, shift). ok=false si le
// caractère n'a pas d'équivalent direct sur le clavier MO5.
func CharToMO5Key(r rune) (key int, shift bool, ok bool) {
	c, found := charToMO5[r]
	return c.key, c.shift, found
}

type phase int

const (
	phaseIdle phase = iota // aucune frappe en cours
	phaseHold              // touche maintenue pressée
	phaseGap               // relâchement avant la frappe suivante
)

// Injector rejoue une file de frappes caractère par caractère : chaque frappe
// est maintenue holdFrames puis suivie d'un trou de gapFrames, pour que le scan
// clavier de la ROM distingue deux frappes identiques consécutives.
type Injector struct {
	model      *Model
	queue      []charKey
	holdFrames int
	gapFrames  int
	queueMax   int

	phase   phase
	current charKey
	timer   int
}

// NewInjector crée un injecteur pour le modèle clavier donné, avec les durées
// fournies (frames). Le modèle porte la table caractère → touche et les indices
// des modificateurs (data-driven).
func NewInjector(model *Model, holdFrames, gapFrames int) *Injector {
	return &Injector{
		model:      model,
		holdFrames: holdFrames,
		gapFrames:  gapFrames,
		queueMax:   DefaultQueueMax,
	}
}

// Enqueue ajoute le caractère à la file s'il a un équivalent MO5. La file est
// bornée : au-delà, la frappe la plus ancienne est abandonnée.
func (i *Injector) Enqueue(r rune) {
	key, shift, ok := i.model.CharToKey(r)
	if !ok {
		return
	}
	if i.queueMax > 0 && len(i.queue) >= i.queueMax {
		i.queue = i.queue[1:]
	}
	i.queue = append(i.queue, charKey{key: key, shift: shift})
}

// EnqueueString enfile une séquence (--exec, coller presse-papier). Les fins de
// ligne \r\n et \r sont normalisées en un seul ENT ; les caractères sans
// équivalent MO5 sont ignorés.
func (i *Injector) EnqueueString(s string) {
	runes := []rune(s)
	for j := 0; j < len(runes); j++ {
		if runes[j] == '\r' {
			if j+1 < len(runes) && runes[j+1] == '\n' {
				j++ // consommer le \n de la paire \r\n
			}
			i.Enqueue('\n')
			continue
		}
		i.Enqueue(runes[j])
	}
}

// Pending retourne le nombre de frappes en attente (frappe courante incluse).
func (i *Injector) Pending() int {
	n := len(i.queue)
	if i.phase != phaseIdle {
		n++
	}
	return n
}

// Tick avance d'une frame et retourne les touches MO5 à presser pendant cette
// frame (touche courante + SHIFT si nécessaire). Retourne nil pendant les trous
// et au repos.
func (i *Injector) Tick() []int {
	if i.phase == phaseIdle {
		if len(i.queue) == 0 {
			return nil
		}
		i.current = i.queue[0]
		i.queue = i.queue[1:]
		i.phase = phaseHold
		i.timer = i.holdFrames
	}

	switch i.phase {
	case phaseHold:
		keys := []int{i.current.key}
		if i.current.shift {
			keys = append(keys, i.model.ShiftKey)
		}
		i.timer--
		if i.timer <= 0 {
			i.phase = phaseGap
			i.timer = i.gapFrames
			if i.current.key == i.model.ENTKey {
				i.timer = enterGapFrames // laisser le BASIC traiter la ligne
			}
		}
		return keys
	case phaseGap:
		i.timer--
		if i.timer <= 0 {
			i.phase = phaseIdle
		}
		return nil
	default:
		return nil
	}
}
