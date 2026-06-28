// Fichier : to8d.go — modèle clavier TO8D (lot #116 + Inc Kb du fix clavier).
//
// Réf C : dcto8dkeyb.c (table keyboardbutton, scancodes 0x00-0x53), dcto8dkeyb.h
// (pckeycode[] PC→TO8D), dcto8dinterface.c (labels physiques décodés Latin-1).
// Le clavier TO8D compte 84 touches (KEYBOARDKEY_MAX). Indices modificateurs :
// SHIFT 0x51 (un 2e SHIFT 0x52 existe au niveau matériel, géré par le gate-array
// — côté hôte les deux KeyShift pointent sur 0x51, cf. keyboard_init.go), CNT
// 0x53, ACC 0x14, ENTRÉE 0x46.
//
// La table caractères contient : 26 lettres (insensibles à la casse, le firmware
// gère la casse via capslock), espace, ENT, plus (Inc Kb) la rangée chiffres/
// symboles AZERTY-FR au format « libellé gauche / libellé droit » :
//   - libellé GAUCHE = frappe SANS shift (typiquement accent ou symbole)
//   - libellé DROIT  = frappe AVEC shift (typiquement chiffre)
//
// Convention « accent direct, chiffre en shift » alignée sur pckeycode[].
// Décisions owner 28/06 :
//   - layout AZERTY France-Windows v1, autres layouts hors scope
//   - touches mortes ^¨ (label 0x4d) NON mappées en rune : taper '^' isolé sur
//     TO8D requerrait ACC + SHIFT + touche (séquence morte) — PR future
//   - majuscules accentuées (É, À, Ç, Ù) NON produisibles (limitation matérielle :
//     pas de touche dédiée ; le firmware compose via ACC sur le hardware réel)
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

	// Rangée chiffres + accents (Inc Kb). Convention « accent direct, chiffre en
	// shift » (pckeycode[]). Indices et labels vérifiés dans dcto8dinterface.c
	// (décodage Latin-1 → UTF-8).
	'_': {0x01, false}, '6': {0x01, true}, // label « _ 6 »
	'(': {0x09, false}, '5': {0x09, true}, // label « ( 5 »
	'\'': {0x11, false}, '4': {0x11, true}, // label « ' 4 »
	'"': {0x19, false}, '3': {0x19, true}, // label « " 3 »
	'é': {0x21, false}, '2': {0x21, true}, // label « é 2 »
	'*': {0x29, false}, '1': {0x29, true}, // label « * 1 »
	'è': {0x31, false}, '7': {0x31, true}, // label « è 7 »
	'!': {0x39, false}, '8': {0x39, true}, // label « ! 8 »
	'ç': {0x41, false}, '9': {0x41, true}, // label « ç 9 »
	'à': {0x49, false}, '0': {0x49, true}, // label « à 0 »

	// Symboles « = + » et autres paires de la 4e rangée.
	'=': {0x0c, false}, '+': {0x0c, true}, // label « = + »
	'#': {0x28, false}, '@': {0x28, true}, // label « # @ »
	'[': {0x2c, false}, '{': {0x2c, true}, // label « [ { »
	',': {0x37, false}, '?': {0x37, true}, // label « , ? »
	'$': {0x3c, false}, '&': {0x3c, true}, // label « $ & »
	']': {0x3e, false}, '}': {0x3e, true}, // label « ] } »
	';': {0x3f, false}, '.': {0x3f, true}, // label « ; . »
	'-': {0x44, false}, '\\': {0x44, true}, // label « - \ »
	'ù': {0x45, false}, '%': {0x45, true}, // label « ù % »
	':': {0x47, false}, '/': {0x47, true}, // label « : / »
	')': {0x4c, false}, '°': {0x4c, true}, // label « ) ° »
	'>': {0x4f, false}, '<': {0x4f, true}, // label « > < »
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
