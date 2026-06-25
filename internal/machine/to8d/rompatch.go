// Fichier : rompatch.go — alignement des ROM TO8D (moniteur + BASIC) sur le modèle
// « trap », en mémoire uniquement.
//
// Pourquoi : la VRAIE ROM TO8D pilote certaines E/S (disque, cassette, crayon
// optique, souris, imprimante, clavier) par accès matériel bas niveau qui ne sont
// pas émulés tels quels — le firmware bouclerait sur des registres non modélisés.
// L'émulateur de référence dcto8d (Daniel Coulom, GPLv3) contourne cela au Hardreset
// via TO8dpatch() (dcto8demulation.c:340-365) : il remplace ces routines par des
// stubs « opcode-trap + RTS », interceptés ici par gatearray.Trap (cf. io.go).
//
// On reproduit fidèlement ce modèle, mais en patchant la copie EN MÉMOIRE des blobs
// ROM chargés — le fichier rom/to8d.rom fourni par l'utilisateur n'est JAMAIS modifié.
//
// Les octets `original`/`patched` ont été extraits octet-à-octet de la référence C :
//   - octets ORIGINAUX : tableaux bruts to8dmoniteur[] / to8dbasic[] ;
//   - octets PATCHÉS   : tables to8dmoniteurpatch[] / to8dbasicpatch[].
//
// Mappage d'offset (cf. TO8dpatch) :
//   - moniteur : off = bank + (addr - 0xE000), bank ∈ {0x0000, 0x2000} ;
//   - BASIC    : off = bank + addr,            bank = 0xC000.
//
// HORS PÉRIMÈTRE #118-minimal : l'injection de la date courante au Hardreset
// (dcto8demulation.c:366-376) est volontairement omise — purement cosmétique (BASIC),
// et time.Now() romprait le déterminisme du boot. À porter dans un lot dédié.
package to8d

import "bytes"

// romPatch décrit le remplacement d'un fragment de n octets dans un blob ROM.
// len(original) == len(patched) == n (vérifié par TestPatchTablesWellFormed).
type romPatch struct {
	off      int    // offset dans le blob ROM (moniteur ou BASIC)
	original []byte // octets attendus dans la ROM brute
	patched  []byte // octets de remplacement (opcode-trap + RTS / opérande)
	desc     string
}

// monitorPatches : ROM moniteur (16 Ko). Réf C to8dmoniteurpatch[].
var monitorPatches = []romPatch{
	{0x00fe, []byte{0x8d}, []byte{0x39}, "contrôleur : reset → RTS"},
	{0x0177, []byte{0x17, 0x02}, []byte{0x15, 0x39}, "disque : écrire secteur (trap 0x15) + RTS"},
	{0x03a7, []byte{0x17, 0x00}, []byte{0x14, 0x39}, "disque : lire secteur (trap 0x14) + RTS"},
	{0x04c8, []byte{0x8d, 0x69, 0x96}, []byte{0x18, 0x35, 0xff}, "disque : formater (trap 0x18)"},
	{0x0c30, []byte{0x34, 0x4e}, []byte{0x4e, 0x39}, "souris : coordonnées (trap 0x4e) + RTS"},
	{0x0c80, []byte{0xb6, 0x60, 0x74}, []byte{0x52, 0x20, 0x0e}, "souris : clic (trap 0x52)"},
	{0x1a60, []byte{0x34, 0x43}, []byte{0x4b, 0x39}, "crayon : coordonnées (trap 0x4b) + RTS"},
	{0x1b3b, []byte{0x34, 0x7f}, []byte{0x51, 0x39}, "interface comm./imprimante (trap 0x51) + RTS"},
	{0x2fd3, []byte{0x34, 0x01, 0x1a}, []byte{0x5f, 0x35, 0x9a}, "clavier : écriture"},
	{0x30a5, []byte{0x86, 0x02}, []byte{0x20, 0x3e}, "clavier : écriture modifiée"},
	{0x30f7, []byte{0x96}, []byte{0x86}, "clavier : LDA #imm (injection scancode)"},
	{0x3124, []byte{0x96}, []byte{0x86}, "clavier : LDA #imm (injection CTRL)"},
}

// basicPatches : ROM interne BASIC (64 Ko). Réf C to8dbasicpatch[].
var basicPatches = []romPatch{
	{0xf273, []byte{0x95, 0x2a}, []byte{0x20, 0x0e}, "open lecture/écriture"},
	{0xf28c, []byte{0x27}, []byte{0x01}, "suppression IO device error"},
	{0xf2f1, []byte{0x34, 0x04}, []byte{0x45, 0x39}, "cassette : écrire octet (trap 0x45) + RTS"},
	{0xf338, []byte{0xa6, 0x53}, []byte{0x42, 0x39}, "cassette : lire octet (trap 0x42) + RTS"},
	{0xfcec, []byte{0xb6, 0xe7, 0xcc}, []byte{0x52, 0x12, 0x12}, "souris : clic écran palette (trap 0x52)"},
	{0xfcfa, []byte{0x85, 0xc0, 0x27}, []byte{0x4e, 0x12, 0x20}, "souris : coordonnées palette (trap 0x4e)"},
	{0xff96,
		[]byte{0x34, 0x4f, 0x86, 0xe7, 0x1f, 0x8b, 0x7c, 0x60, 0x75},
		[]byte{0x8e, 0x05, 0x58, 0x30, 0x1f, 0x26, 0xfc, 0x4b, 0x39},
		"crayon : coordonnées écran palette (trap 0x4b)"},
}

// patchReport rend compte de l'application (diagnostic/tests).
type patchReport struct {
	Applied int  // points patchés à cet appel (étaient à l'octet d'origine)
	Already int  // points déjà patchés (idempotence)
	OK      bool // true si le blob est reconnu (tous les points connus)
}

// applyPatches aligne un blob ROM sur le modèle trap, en mémoire uniquement.
//
// Stratégie TOUT-OU-RIEN et SÛRE (identique au MO5, internal/core/rompatch.go) : on
// vérifie d'abord que CHAQUE point correspond soit à l'octet d'origine, soit à
// l'octet déjà patché. Si un seul point est hors bornes ou inconnu, le blob n'est
// pas la variante reconnue : on n'écrit RIEN (OK=false) pour ne pas corrompre une ROM
// inattendue. Idempotent : réappliquer ne fait rien de plus.
func applyPatches(blob []byte, patches []romPatch) patchReport {
	// Passe 1 : vérification (bornes + octet d'origine OU déjà patché).
	for _, p := range patches {
		if p.off < 0 || p.off+len(p.original) > len(blob) {
			return patchReport{OK: false}
		}
		cur := blob[p.off : p.off+len(p.original)]
		if !bytes.Equal(cur, p.original) && !bytes.Equal(cur, p.patched) {
			return patchReport{OK: false}
		}
	}
	// Passe 2 : application (seuls les points encore à l'octet d'origine).
	rep := patchReport{OK: true}
	for _, p := range patches {
		dst := blob[p.off : p.off+len(p.patched)]
		if bytes.Equal(dst, p.patched) {
			rep.Already++
			continue
		}
		copy(dst, p.patched)
		rep.Applied++
	}
	return rep
}
