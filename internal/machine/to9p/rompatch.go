package to9p

import "bytes"

type romPatch struct {
	off      int
	original []byte
	patched  []byte
	desc     string
}

var basicPatches = []romPatch{
	{0xee51, []byte{0xb7}, []byte{0xf6}, "BASIC: adaptation DCTO9P"},
	{0xee5f, []byte{0xb7}, []byte{0xb6}, "BASIC: adaptation DCTO9P"},
	{0xeea1, []byte{0xf7}, []byte{0xf6}, "BASIC: adaptation DCTO9P"},
	{0xf273, []byte{0x95, 0x2a}, []byte{0x20, 0x0e}, "open lecture/écriture"},
	{0xf28c, []byte{0x27}, []byte{0x01}, "suppression IO device error"},
	{0xf2f1, []byte{0x34, 0x04}, []byte{0x45, 0x39}, "cassette : écrire octet (trap 0x45) + RTS"},
	{0xf338, []byte{0xa6, 0x53}, []byte{0x42, 0x39}, "cassette : lire octet (trap 0x42) + RTS"},
	{0xf576,
		[]byte{0x34, 0x4f, 0x86, 0xe7, 0x1f, 0x8b, 0x7c, 0x60, 0x75},
		[]byte{0x8e, 0x05, 0x58, 0x30, 0x1f, 0x26, 0xfc, 0x4b, 0x39},
		"crayon : coordonnées écran palette (trap 0x4b)"},
	{0xfd10, []byte{0xb6, 0xe7, 0xcc}, []byte{0x52, 0x12, 0x12}, "souris : clic écran palette (trap 0x52)"},
	{0xfd1e, []byte{0x85, 0xc0, 0x27}, []byte{0x4e, 0x12, 0x20}, "souris : coordonnées palette (trap 0x4e)"},
}

var monitorPatches = []romPatch{
	{0x00fe, []byte{0x8d}, []byte{0x39}, "contrôleur : reset → RTS"},
	{0x0177, []byte{0x17, 0x02}, []byte{0x15, 0x39}, "disque : écrire secteur (trap 0x15) + RTS"},
	{0x03a7, []byte{0x17, 0x00}, []byte{0x14, 0x39}, "disque : lire secteur (trap 0x14) + RTS"},
	{0x04c8, []byte{0x8d, 0x69, 0x96}, []byte{0x18, 0x35, 0xff}, "disque : formater (trap 0x18)"},
	{0x1a72, []byte{0xb6, 0x60, 0x74}, []byte{0x52, 0x20, 0x0e}, "souris : clic (trap 0x52)"},
	{0x1aa1, []byte{0x34, 0x43}, []byte{0x4b, 0x39}, "crayon : coordonnées (trap 0x4b) + RTS"},
	{0x1b83, []byte{0x34, 0x7f}, []byte{0x51, 0x39}, "interface comm./imprimante (trap 0x51) + RTS"},
	{0x2fda, []byte{0xc1, 0xf6, 0x27}, []byte{0x5f, 0x35, 0x9a}, "clavier : écriture"},
	{0x3193, []byte{0x34, 0x4e}, []byte{0x4e, 0x39}, "souris : coordonnées (trap 0x4e) + RTS"},
}

// patchReport reprend la forme des patchers MO5/TO8D. Pour le TO9+ Lot #186,
// OK=false ne reconnaît aucune variante : il signale seulement une violation
// d'invariant de taille avant mutation.
type patchReport struct {
	Applied int
	Already int
	OK      bool
}

func validatePatches(blob []byte, patches []romPatch) bool {
	for _, p := range patches {
		if p.off < 0 || p.off+len(p.original) > len(blob) {
			return false
		}
		cur := blob[p.off : p.off+len(p.original)]
		if !bytes.Equal(cur, p.original) && !bytes.Equal(cur, p.patched) {
			return false
		}
	}
	return true
}

func applyValidatedPatches(blob []byte, patches []romPatch) patchReport {
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

func applyPatches(blob []byte, patches []romPatch) patchReport {
	if !validatePatches(blob, patches) {
		return patchReport{OK: false}
	}
	return applyValidatedPatches(blob, patches)
}

// applyROMPatches aligne les copies mémoire des ROM TO9+ sur le modèle trap de
// DCTO9P. Les fichiers ROM fournis par l'utilisateur ne sont jamais modifiés.
func applyROMPatches(romMon, romBasic []byte) patchReport {
	if len(romMon) != romMonSize || len(romBasic) != romBasicSize {
		return patchReport{OK: false}
	}
	if !validatePatches(romBasic, basicPatches) || !validatePatches(romMon, monitorPatches) {
		return patchReport{OK: false}
	}
	basic := applyValidatedPatches(romBasic, basicPatches)
	monitor := applyValidatedPatches(romMon, monitorPatches)
	return patchReport{
		Applied: basic.Applied + monitor.Applied,
		Already: basic.Already + monitor.Already,
		OK:      true,
	}
}
