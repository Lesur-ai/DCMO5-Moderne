package to8d

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// romTestPath retourne le chemin de la ROM TO8D réelle, versionnée dans le dépôt
// (rom/to8d.rom), relatif au paquet (internal/machine/to8d). Disponible en CI.
func romTestPath() string { return filepath.Join("..", "..", "..", "rom", "to8d.rom") }

// TestPatchTablesWellFormed garantit l'invariant exploité par applyPatches :
// len(original) == len(patched) > 0 pour chaque entrée (sinon la passe 1 et la passe 2
// n'opèrent pas sur la même fenêtre d'octets).
func TestPatchTablesWellFormed(t *testing.T) {
	tables := []struct {
		name string
		ps   []romPatch
	}{{"monitor", monitorPatches}, {"basic", basicPatches}}
	for _, tbl := range tables {
		for _, p := range tbl.ps {
			if len(p.original) == 0 || len(p.original) != len(p.patched) {
				t.Errorf("%s @0x%04x (%s): original=%d patched=%d (attendu égaux et >0)",
					tbl.name, p.off, p.desc, len(p.original), len(p.patched))
			}
		}
	}
}

// TestApplyPatches_Idempotent : sur un blob brut synthétique, la 1re passe patche tous
// les points (Applied=N, Already=0) ; la 2e passe ne fait rien (Applied=0, Already=N).
func TestApplyPatches_Idempotent(t *testing.T) {
	blob := make([]byte, romMonSize)
	for _, p := range monitorPatches {
		copy(blob[p.off:], p.original)
	}
	r1 := applyPatches(blob, monitorPatches)
	if !r1.OK || r1.Applied != len(monitorPatches) || r1.Already != 0 {
		t.Fatalf("1re passe = %+v, attendu OK, Applied=%d, Already=0", r1, len(monitorPatches))
	}
	for _, p := range monitorPatches {
		if got := blob[p.off : p.off+len(p.patched)]; !bytes.Equal(got, p.patched) {
			t.Errorf("@0x%04x (%s) = % x, attendu % x", p.off, p.desc, got, p.patched)
		}
	}
	r2 := applyPatches(blob, monitorPatches)
	if !r2.OK || r2.Applied != 0 || r2.Already != len(monitorPatches) {
		t.Fatalf("2e passe (idempotence) = %+v, attendu OK, Applied=0, Already=%d", r2, len(monitorPatches))
	}
}

// TestApplyPatches_RejectsUnknown : un seul point ni original ni patché ⇒ OK=false et
// AUCUNE écriture (stratégie tout-ou-rien, passe 1 protège la ROM inattendue).
func TestApplyPatches_RejectsUnknown(t *testing.T) {
	blob := make([]byte, romMonSize)
	for _, p := range monitorPatches {
		copy(blob[p.off:], p.original)
	}
	blob[monitorPatches[0].off] = 0xAA // octet inconnu
	if r := applyPatches(blob, monitorPatches); r.OK {
		t.Fatalf("blob inconnu accepté : %+v", r)
	}
	// Les autres points doivent rester à l'octet d'origine (rien écrit).
	p1 := monitorPatches[1]
	if got := blob[p1.off : p1.off+len(p1.original)]; !bytes.Equal(got, p1.original) {
		t.Errorf("mutation malgré rejet @0x%04x : % x", p1.off, got)
	}
}

// TestApplyPatches_OutOfBounds : un blob trop court est rejeté sans panique.
func TestApplyPatches_OutOfBounds(t *testing.T) {
	if r := applyPatches(make([]byte, 0x10), monitorPatches); r.OK {
		t.Fatalf("blob hors bornes accepté : %+v", r)
	}
}

// TestSplitROMMatchesReference est le test de FIDÉLITÉ du découpage contre la ROM
// RÉELLE (rom/to8d.rom, versionnée). Il verrouille trois propriétés à la fois :
//   - l'ORDRE du blob : BASIC (64 Ko) d'abord, moniteur (16 Ko) ensuite. Si l'ordre
//     était inversé, les octets aux offsets ne seraient ni original ni patché ⇒ OK=false ;
//   - la ROM est BRUTE : tous les points sont à l'octet d'origine (Applied=N, Already=0) ;
//   - le vecteur reset (banque système 0) pointe bien dans la plage moniteur.
//
// Toute substitution de ROM ou inversion du découpage fait échouer ce test.
func TestSplitROMMatchesReference(t *testing.T) {
	blob, err := os.ReadFile(romTestPath())
	if err != nil {
		t.Fatalf("lecture ROM réelle : %v", err)
	}
	if len(blob) != romTotalSize {
		t.Fatalf("taille ROM = %d, attendu %d", len(blob), romTotalSize)
	}
	romBasic := append([]byte(nil), blob[:romBasicSize]...)
	romMon := append([]byte(nil), blob[romBasicSize:]...)

	if r := applyPatches(romMon, monitorPatches); !r.OK || r.Applied != len(monitorPatches) || r.Already != 0 {
		t.Fatalf("moniteur réel = %+v, attendu OK, Applied=%d, Already=0 (ordre du blob ou ROM inattendue)",
			r, len(monitorPatches))
	}
	if r := applyPatches(romBasic, basicPatches); !r.OK || r.Applied != len(basicPatches) || r.Already != 0 {
		t.Fatalf("basic réel = %+v, attendu OK, Applied=%d, Already=0", r, len(basicPatches))
	}

	// Vecteur reset banque système 0 : romsys mappe 0xE000 → romMon[0], donc 0xFFFE
	// se lit en romMon[0x1ffe]. Doit pointer dans le moniteur (≥ 0xE000).
	reset := uint16(romMon[0x1ffe])<<8 | uint16(romMon[0x1fff])
	if reset < 0xE000 {
		t.Fatalf("vecteur reset = 0x%04x, hors plage moniteur 0xE000-0xFFFF", reset)
	}
}
