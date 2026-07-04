// Fichier : to9p.go - modele clavier TO9+.
//
// Ref C : dcto9/source/dcto9pkeyb.h (to9pkey[] indices 0x00-0x53) et
// dcto9/source/dcto9pemulation.c (to9key[] scancode TO8-style -> ASCII TO9+).
// Le modele hote garde les indices physiques TO9+ ; le gate-array TO9+ convertit
// ensuite ces indices en ASCII via E7DE/E7DF.
package keyboard

const (
	to9pKeyACC   = 0x14 // ACC ; cote materiel, touche normale dans TO9key()
	to9pKeyENT   = 0x46 // ENT principale
	to9pKeyShift = 0x51 // SHIFT gauche ; SHIFT droit 0x52 gere cote gate-array
	to9pKeyCNT   = 0x53 // CONTROL (CNT)
	to9pKeyCount = 84   // TO9PKEY_MAX
)

// charToTO9P est volontairement distinct de charToTO8D meme quand les indices
// physiques coincident : la publication materielle TO9+ diverge (ASCII E7DF).
var charToTO9P = map[rune]charKey{
	'y': {0x02, false}, 'Y': {0x02, false},
	'h': {0x03, false}, 'H': {0x03, false},
	'n': {0x07, false}, 'N': {0x07, false},
	't': {0x0a, false}, 'T': {0x0a, false},
	'g': {0x0b, false}, 'G': {0x0b, false},
	'b': {0x0f, false}, 'B': {0x0f, false},
	'r': {0x12, false}, 'R': {0x12, false},
	'f': {0x13, false}, 'F': {0x13, false},
	'v': {0x17, false}, 'V': {0x17, false},
	'e': {0x1a, false}, 'E': {0x1a, false},
	'd': {0x1b, false}, 'D': {0x1b, false},
	'c': {0x1f, false}, 'C': {0x1f, false},
	'z': {0x22, false}, 'Z': {0x22, false},
	's': {0x23, false}, 'S': {0x23, false},
	'x': {0x27, false}, 'X': {0x27, false},
	'a': {0x2a, false}, 'A': {0x2a, false},
	'q': {0x2b, false}, 'Q': {0x2b, false},
	'w': {0x2f, false}, 'W': {0x2f, false},
	'u': {0x32, false}, 'U': {0x32, false},
	'j': {0x33, false}, 'J': {0x33, false},
	'i': {0x3a, false}, 'I': {0x3a, false},
	'k': {0x3b, false}, 'K': {0x3b, false},
	'o': {0x42, false}, 'O': {0x42, false},
	'l': {0x43, false}, 'L': {0x43, false},
	'p': {0x4a, false}, 'P': {0x4a, false},
	'm': {0x4b, false}, 'M': {0x4b, false},

	' ':  {0x34, false},
	'\n': {to9pKeyENT, false},
	'\r': {to9pKeyENT, false},

	'_': {0x01, false}, '6': {0x01, true},
	'(': {0x09, false}, '5': {0x09, true},
	'\'': {0x11, false}, '4': {0x11, true},
	'"': {0x19, false}, '3': {0x19, true},
	'é': {0x21, false}, '2': {0x21, true},
	'*': {0x29, false}, '1': {0x29, true},
	'è': {0x31, false}, '7': {0x31, true},
	'!': {0x39, false}, '8': {0x39, true},
	'ç': {0x41, false}, '9': {0x41, true},
	'à': {0x49, false}, '0': {0x49, true},

	'=': {0x0c, false}, '+': {0x0c, true},
	'#': {0x28, false}, '@': {0x28, true},
	'[': {0x2c, false}, '{': {0x2c, true},
	',': {0x37, false}, '?': {0x37, true},
	'$': {0x3c, false}, '&': {0x3c, true},
	']': {0x3e, false}, '}': {0x3e, true},
	';': {0x3f, false}, '.': {0x3f, true},
	'-': {0x44, false}, '\\': {0x44, true},
	'ù': {0x45, false}, '%': {0x45, true},
	':': {0x47, false}, '/': {0x47, true},
	')': {0x4c, false}, '°': {0x4c, true},
	'>': {0x4f, false}, '<': {0x4f, true},
}

var to9pModel = &Model{
	KeyCount: to9pKeyCount,
	ShiftKey: to9pKeyShift,
	CNTKey:   to9pKeyCNT,
	ACCKey:   to9pKeyACC,
	ENTKey:   to9pKeyENT,
	JoystickKeyboardSuppressSpecialKeys: map[int]bool{
		0x04: true, // flèche haut
		0x3d: true, // flèche bas
		0x0d: true, // flèche gauche
		0x05: true, // flèche droite
	},
	chars: charToTO9P,
}

// TO9PModel retourne le modele clavier du Thomson TO9+.
func TO9PModel() *Model { return to9pModel }
