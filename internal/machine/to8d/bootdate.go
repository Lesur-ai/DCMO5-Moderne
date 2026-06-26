// Fichier : bootdate.go — injection de la date courante au boot TO8D, en mémoire
// uniquement (copie du blob ROM BASIC, jamais le fichier rom/to8d.rom).
//
// Pourquoi : le moniteur TO8D démarre sur « DATE : 00-00-00 » (matériel nu, la date
// se saisit à la main). L'émulateur de référence dcto8d (Daniel Coulom, GPLv3)
// pré-remplit la date courante au Hardreset (dcto8demulation.c:366-376) :
//   - il remplace la chaîne « jj-mm-aa » du BASIC (addr $2B90 = offset 0xEB90) par
//     la date du jour au format %d-%m-%y ;
//   - il patche la routine de reset (addr $24E2 = offset 0xE4E2) en
//     « LDX #$2B90 ; BSR $29C8 » pour que le moniteur initialise la date système à
//     partir de cette chaîne.
//
// On reproduit fidèlement ce comportement. Le TODO différé de #118 (cf. rompatch.go)
// est ainsi levé.
//
// Déterminisme : la date est passée PAR VALEUR (now), jamais lue via time.Now() ici.
// Le chemin de production injecte time.Now() ; les tests injectent une date fixe, ce
// qui garde le boot reproductible (cf. TestBootDeterministic).
package to8d

import (
	"bytes"
	"time"
)

// Offsets dans le blob ROM BASIC (64 Ko). Mappage BASIC : offset = 0xC000 + addr
// (cf. rompatch.go). Réf C : dcto8demulation.c:366-376.
const (
	bootDateStrOff  = 0xEB90 // chaîne « jj-mm-aa » (addr BASIC $2B90)
	bootDateTermOff = 0xEB98 // octet terminal (0x1F) suivant la chaîne
	bootDateInitOff = 0xE4E2 // routine de reset à patcher (addr BASIC $24E2)
)

// Octets d'origine attendus (garde « tout-ou-rien ») et octets injectés.
var (
	bootDatePlaceholder = []byte("jj-mm-aa")                         // 6a 6a 2d 6d 6d 2d 61 61
	bootDateInitOrig    = []byte{0xb7, 0xe7, 0xfe, 0xb7, 0xe7, 0xfa} // STA $E7FE ; STA $E7FA
	bootDateInitPatch   = []byte{0x8e, 0x2b, 0x90, 0xbd, 0x29, 0xc8} // LDX #$2B90 ; BSR $29C8
)

// bootDateLayout = "jj-mm-aa" exprimé dans le calendrier de référence Go (02 = jour,
// 01 = mois, 06 = année sur 2 chiffres), produisant exactement 8 octets ASCII.
const bootDateLayout = "02-01-06"

// injectBootDate écrit la date now (format jj-mm-aa) dans la copie EN MÉMOIRE du blob
// ROM BASIC et patche la routine de reset associée. À appeler UNE FOIS sur une copie
// fraîche du blob (post-applyPatches), depuis newFromROM.
//
// TOUT-OU-RIEN (même discipline que applyPatches) : si le slot date, son octet
// terminal ou la routine de reset ne portent pas les octets d'origine attendus, le
// blob n'est pas la variante reconnue → on n'écrit RIEN et on retourne false (jamais
// de corruption d'une ROM inattendue). Comme applyPatches valide déjà cette même ROM
// BASIC en amont, un retour false ici signale une incohérence de variante.
func injectBootDate(romBasic []byte, now time.Time) bool {
	// Bornes : le plus grand offset touché est bootDateInitOff + 6 et bootDateTermOff.
	if bootDateInitOff+len(bootDateInitPatch) > len(romBasic) || bootDateTermOff >= len(romBasic) {
		return false
	}
	// Vérification des octets d'origine (slot date + terminal + routine reset).
	if !bytes.Equal(romBasic[bootDateStrOff:bootDateStrOff+len(bootDatePlaceholder)], bootDatePlaceholder) ||
		romBasic[bootDateTermOff] != 0x1f ||
		!bytes.Equal(romBasic[bootDateInitOff:bootDateInitOff+len(bootDateInitOrig)], bootDateInitOrig) {
		return false
	}
	// Écriture : date (8 octets, longueur garantie par bootDateLayout) + terminal + routine.
	copy(romBasic[bootDateStrOff:], []byte(now.Format(bootDateLayout)))
	romBasic[bootDateTermOff] = 0x1f
	copy(romBasic[bootDateInitOff:], bootDateInitPatch)
	return true
}
