// Fichier : model.go — modèle clavier data-driven d'une machine Thomson.
//
// Le modèle (nombre de touches, table caractère → touche, indices des
// modificateurs) est porté par la machine (machine.Machine.KeyboardModel) au lieu
// d'être codé en dur dans l'injecteur et l'UI. La famille TO (clavier 84 touches,
// table ASCII propre) fournira son modèle (#116) sans toucher à l'injecteur.
package keyboard

// Indices des touches modificatrices MO5 (réf : dcmo5keyb.h mo5key[]).
const (
	mo5KeyACC   = 0x36 // ACC (accent / AltGr)
	mo5KeyCount = 58   // nombre de touches du clavier MO5 (MO5KEY_MAX)
)

// Model décrit le clavier d'une machine : nombre de touches (borne des indices),
// table caractère → combinaison de touches, et indices des touches modificatrices
// (SHIFT/CNT/ACC/ENTRÉE). ACCKey vaut -1 si la machine n'a pas de touche ACC.
type Model struct {
	KeyCount int
	ShiftKey int
	CNTKey   int
	ACCKey   int
	ENTKey   int
	chars    map[rune]charKey
}

// CharToKey traduit un caractère en (touche, shift). ok=false si le caractère n'a
// pas d'équivalent direct sur ce clavier.
func (m *Model) CharToKey(r rune) (key int, shift bool, ok bool) {
	c, found := m.chars[r]
	return c.key, c.shift, found
}

// IsModifier indique si l'index de touche est un modificateur (SHIFT/CNT/ACC).
func (m *Model) IsModifier(key int) bool {
	return key == m.ShiftKey || key == m.CNTKey || (m.ACCKey >= 0 && key == m.ACCKey)
}

// mo5Model est le modèle clavier MO5 (singleton : la table chars est en lecture
// seule). Partagé par MO5Model() et le wrapper de compatibilité CharToMO5Key.
var mo5Model = &Model{
	KeyCount: mo5KeyCount,
	ShiftKey: Mo5KeyShift,
	CNTKey:   Mo5KeyCNT,
	ACCKey:   mo5KeyACC,
	ENTKey:   Mo5KeyENT,
	chars:    charToMO5,
}

// MO5Model retourne le modèle clavier du Thomson MO5.
func MO5Model() *Model { return mo5Model }
