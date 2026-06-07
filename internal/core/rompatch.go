// Fichier : rompatch.go — alignement de la ROM système MO5 sur le modèle « trap ».
//
// Pourquoi : la VRAIE ROM MO5 (mo5-v1.1) pilote certaines E/S (cassette, crayon
// optique, imprimante) par accès matériel bas niveau — par ex. la lecture
// cassette attend en boucle un front sur le bit 7 du port 0xA7C0, signal fourni
// par le matériel réel mais NON émulé. Résultat : un LOAD" boucle indéfiniment
// sans jamais atteindre la routine de lecture.
//
// L'émulateur de référence dcmo5 v11 contourne cela en embarquant une ROM
// PATCHÉE où ces routines sont remplacées par des stubs « opcode illégal + RTS » :
// l'opcode illégal est intercepté par entreesortie() (cf. io.go) qui dispatche
// vers le périphérique émulé. On reproduit fidèlement ce modèle, mais en
// patchant la copie EN MÉMOIRE de la ROM (m.rom[]) — le fichier ROM fourni par
// l'utilisateur n'est JAMAIS modifié.
//
// Les 5 points ci-dessous ont été obtenus par diff exhaustif octet-à-octet entre
// rom/mo5-v1.1.rom (réelle) et dcmo5v11.0/include/dcmo5rom.h (ROM patchée du C).
package core

// romBase est l'adresse de mappage de la ROM système (0xC000–0xFFFF).
const romBase = 0xC000

// romPatch décrit le remplacement d'un fragment de 2 octets dans la ROM système.
// original : octets attendus dans la ROM réelle non patchée.
// patched  : octets de remplacement (opcode-trap I/O + RTS 0x39).
type romPatch struct {
	addr     uint16
	original [2]byte
	patched  [2]byte
	desc     string
}

// dcmo5SystemRomPatches : table de patch alignée sur la ROM patchée de dcmo5 v11.
// Chaque entrée transforme une routine d'E/S matérielle en stub-trap.
var dcmo5SystemRomPatches = []romPatch{
	{0xF168, [2]byte{0xA6, 0xC4}, [2]byte{0x41, 0x39}, "cassette : lire bit (trap 0x41) + RTS"},
	{0xF181, [2]byte{0xC6, 0x08}, [2]byte{0x42, 0x39}, "cassette : lire octet (trap 0x42) + RTS"},
	{0xF1AF, [2]byte{0x97, 0x45}, [2]byte{0x45, 0x39}, "cassette : écrire octet (trap 0x45) + RTS"},
	{0xF548, [2]byte{0x1A, 0x50}, [2]byte{0x4B, 0x39}, "crayon optique : lire X/Y (trap 0x4B) + RTS"},
	{0xF713, [2]byte{0xCE, 0xA7}, [2]byte{0x51, 0x39}, "imprimante : émettre (trap 0x51) + RTS"},
}

// RomPatchReport rend compte de l'application des patchs (diagnostic/tests).
type RomPatchReport struct {
	Applied int  // points patchés à cet appel (étaient à l'octet d'origine)
	Already int  // points déjà patchés (idempotence)
	OK      bool // true si la ROM est reconnue (tous les points connus)
}

// applySystemRomPatches aligne m.rom[] sur le modèle trap, en mémoire uniquement.
//
// Stratégie TOUT-OU-RIEN et SÛRE : on vérifie d'abord que CHAQUE point correspond
// soit à l'octet d'origine, soit à l'octet déjà patché. Si un seul point est
// inconnu (ni l'un ni l'autre), la ROM n'est pas la variante reconnue : on
// n'écrit RIEN (OK=false) pour ne pas corrompre une ROM inattendue. Idempotent :
// réappliquer ne fait rien de plus.
func (m *Machine) applySystemRomPatches() RomPatchReport {
	at := func(addr uint16) [2]byte {
		i := int(addr) - romBase
		return [2]byte{m.rom[i], m.rom[i+1]}
	}

	// Passe 1 : vérification. Aucune écriture si un point est hors ROM ou inconnu.
	for _, p := range dcmo5SystemRomPatches {
		i := int(p.addr) - romBase
		if i < 0 || i+1 >= len(m.rom) { // garde de bornes : table mal formée → no-op sûr
			return RomPatchReport{OK: false}
		}
		cur := at(p.addr)
		if cur != p.original && cur != p.patched {
			return RomPatchReport{OK: false}
		}
	}

	// Passe 2 : application (seuls les points encore à l'octet d'origine).
	rep := RomPatchReport{OK: true}
	for _, p := range dcmo5SystemRomPatches {
		i := int(p.addr) - romBase
		if at(p.addr) == p.patched {
			rep.Already++
			continue
		}
		m.rom[i] = p.patched[0]
		m.rom[i+1] = p.patched[1]
		rep.Applied++
	}
	return rep
}

// ── ROM du contrôleur de disquette CD90-640 ───────────────────────────────────
//
// Même problème que la ROM système : la VRAIE ROM contrôleur pilote le FDC par
// des accès matériel bas niveau (poll de registres non émulés → boucles de délai,
// le DOS abandonne sans lire). dcmo5 v11 embarque une ROM contrôleur PATCHÉE où
// les routines lire/écrire/formater secteur sont remplacées par des stubs-trap.
// Les 8 octets ci-dessous proviennent du diff entre la vraie cd90-640.rom et la
// ROM contrôleur de dcmo5rom.h. Patch appliqué EN MÉMOIRE (diskRom[]) uniquement.

// diskRomBase est l'adresse de mappage de la ROM contrôleur (0xA000..0xA7BF).
const diskRomBase = 0xA000

// bytePatch décrit le remplacement d'UN octet (les patchs contrôleur mêlent
// stubs-trap 2 octets et neutralisations 1 octet, d'où une granularité d'octet).
type bytePatch struct {
	addr     uint16
	original uint8
	patched  uint8
	desc     string
}

// dcmo5DiskControllerPatches : alignée sur la ROM contrôleur patchée de dcmo5 v11.
var dcmo5DiskControllerPatches = []bytePatch{
	{0xA12E, 0x86, 0x39, "init/restore FDC → RTS"},
	{0xA17D, 0x8D, 0x15, "écrire secteur (trap 0x15)"},
	{0xA17E, 0x56, 0x39, "  + RTS"},
	{0xA202, 0x8D, 0x14, "lire secteur (trap 0x14)"},
	{0xA203, 0x3B, 0x39, "  + RTS"},
	{0xA30C, 0x17, 0x39, "seek/moteur → RTS"},
	{0xA32C, 0x34, 0x18, "formater (trap 0x18)"},
	{0xA32D, 0x7F, 0x39, "  + RTS"},
}

// applyDiskControllerPatches aligne diskRom[] sur le modèle trap, en mémoire.
// Même stratégie tout-ou-rien / idempotente / sûre que applySystemRomPatches.
// No-op si aucun contrôleur n'est monté (diskRomLen == 0).
func (m *Machine) applyDiskControllerPatches() RomPatchReport {
	if m.diskRomLen == 0 {
		return RomPatchReport{OK: false}
	}
	// Passe 1 : vérification (bornes + octet d'origine ou déjà patché).
	for _, p := range dcmo5DiskControllerPatches {
		i := int(p.addr) - diskRomBase
		if i < 0 || i >= m.diskRomLen {
			return RomPatchReport{OK: false}
		}
		if cur := m.diskRom[i]; cur != p.original && cur != p.patched {
			return RomPatchReport{OK: false}
		}
	}
	// Passe 2 : application.
	rep := RomPatchReport{OK: true}
	for _, p := range dcmo5DiskControllerPatches {
		i := int(p.addr) - diskRomBase
		if m.diskRom[i] == p.patched {
			rep.Already++
			continue
		}
		m.diskRom[i] = p.patched
		rep.Applied++
	}
	return rep
}
