// Fichier : to8d.go — modèle clavier TO8D (lot #116).
//
// Réf C : dcto8dkeyb.c (table keyboardbutton, scancodes 0x00-0x53) et
// dcto8demulation.c TO8key. Le clavier TO8D compte 84 touches (KEYBOARDKEY_MAX).
// Indices modificateurs : SHIFT 0x51 (un 2e SHIFT 0x52 existe ; les deux sont
// traités au niveau matériel par le gate-array), CNT 0x53, ACC 0x14, ENTRÉE 0x46.
//
// Périmètre #116 : la table caractère → touche ne contient ici que les
// correspondances NON AMBIGUËS — les 26 lettres (insensibles à la casse, comme le
// modèle MO5 ; le firmware gère la casse via le capslock), l'espace et ENTRÉE. La
// rangée des chiffres/symboles (libellés AZERTY à deux caractères, p. ex. « _ 6 »,
// « é 2 ») dépend du layout et est finalisée lors du câblage TO8D (#118).
//
// ACCKey = 0x14 sert au filtrage d'injection de la couche app (ne pas taper ACC
// comme un caractère) ; côté matériel, le gate-array traite 0x14 comme une touche
// ordinaire (aucun cas spécial dans TO8key).
package keyboard

// Indices de touches TO8D significatifs (réf C dcto8dkeyb.c).
const (
	to8dKeyACC   = 0x14 // ACC (accent / dead-key)
	to8dKeyENT   = 0x46 // ENTRÉE principale (≠ 0x36 « ENT pad »)
	to8dKeyShift = 0x51 // SHIFT gauche (SHIFT droit 0x52 géré côté gate-array)
	to8dKeyCNT   = 0x53 // CONTROL (CNT)
	to8dKeyCount = 84   // nombre de touches (KEYBOARDKEY_MAX)
)

// charToTO8D traduit un caractère en touche TO8D. Lettres insensibles à la casse
// (scancodes des touches-lettre de keyboardbutton[]), espace (0x34) et ENTRÉE
// (0x46). Chiffres/symboles : déférés au #118 (cf. en-tête de fichier).
var charToTO8D = map[rune]charKey{
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

	' ':  {0x34, false},       // ESPACE
	'\n': {to8dKeyENT, false}, // ENT
	'\r': {to8dKeyENT, false}, // ENT (CRLF → un seul ENT)
}

// to8dModel est le modèle clavier TO8D (singleton, table en lecture seule).
var to8dModel = &Model{
	KeyCount: to8dKeyCount,
	ShiftKey: to8dKeyShift,
	CNTKey:   to8dKeyCNT,
	ACCKey:   to8dKeyACC,
	ENTKey:   to8dKeyENT,
	chars:    charToTO8D,
}

// TO8DModel retourne le modèle clavier du Thomson TO8D.
func TO8DModel() *Model { return to8dModel }
