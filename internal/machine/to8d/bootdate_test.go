package to8d

import (
	"bytes"
	"os"
	"testing"
	"time"
)

// loadBasicBlob lit la copie du blob ROM BASIC réel (premiers romBasicSize octets de
// rom/to8d.rom), déjà aligné « trap » comme dans newFromROM, pour tester l'injection
// de date sur une variante reconnue.
func loadBasicBlob(t *testing.T) []byte {
	t.Helper()
	full, err := os.ReadFile(romTestPath())
	if err != nil {
		t.Fatalf("lecture ROM TO8D : %v", err)
	}
	basic := append([]byte(nil), full[:romBasicSize]...)
	if rep := applyPatches(basic, basicPatches); !rep.OK {
		t.Fatalf("ROM BASIC de test non reconnue par applyPatches")
	}
	return basic
}

// TestInjectBootDate_WritesDateAndRoutine vérifie les TROIS écritures attendues à
// partir de la date fixe : chaîne jj-mm-aa, octet terminal, routine de reset.
func TestInjectBootDate_WritesDateAndRoutine(t *testing.T) {
	basic := loadBasicBlob(t)
	date := time.Date(2026, time.January, 2, 0, 0, 0, 0, time.UTC) // → "02-01-26"

	if !injectBootDate(basic, date) {
		t.Fatal("injectBootDate a échoué sur une ROM BASIC reconnue")
	}

	if got := string(basic[bootDateStrOff : bootDateStrOff+8]); got != "02-01-26" {
		t.Errorf("chaîne date = %q, want %q", got, "02-01-26")
	}
	if basic[bootDateTermOff] != 0x1f {
		t.Errorf("octet terminal = 0x%02x, want 0x1f", basic[bootDateTermOff])
	}
	if got := basic[bootDateInitOff : bootDateInitOff+len(bootDateInitPatch)]; !bytes.Equal(got, bootDateInitPatch) {
		t.Errorf("routine reset = % x, want % x", got, bootDateInitPatch)
	}
}

// TestInjectBootDate_FormatDDMMYY garde le format français jj-mm-aa (fidèle à la réf
// strftime %d-%m-%y) sur une date à composantes distinctes et zéro-paddées.
func TestInjectBootDate_FormatDDMMYY(t *testing.T) {
	basic := loadBasicBlob(t)
	date := time.Date(2025, time.December, 9, 0, 0, 0, 0, time.UTC) // jour 09, mois 12, an 25

	if !injectBootDate(basic, date) {
		t.Fatal("injectBootDate a échoué")
	}
	if got := string(basic[bootDateStrOff : bootDateStrOff+8]); got != "09-12-25" {
		t.Errorf("chaîne date = %q, want %q (jj-mm-aa)", got, "09-12-25")
	}
}

// TestInjectBootDate_RejectsUnknownBlob : sur un blob qui ne porte pas les octets
// d'origine attendus, l'injection ne doit RIEN écrire et retourner false (tout-ou-rien).
func TestInjectBootDate_RejectsUnknownBlob(t *testing.T) {
	blob := make([]byte, romBasicSize) // tout à zéro : ni placeholder ni routine d'origine
	before := append([]byte(nil), blob...)

	if injectBootDate(blob, testBootDate) {
		t.Fatal("injectBootDate aurait dû refuser un blob non reconnu")
	}
	if !bytes.Equal(blob, before) {
		t.Fatal("injectBootDate a muté un blob non reconnu (doit être tout-ou-rien)")
	}
}

// TestInjectBootDate_RejectsTooShort : blob trop court → false sans panic (bornes).
func TestInjectBootDate_RejectsTooShort(t *testing.T) {
	if injectBootDate(make([]byte, bootDateStrOff), testBootDate) {
		t.Fatal("injectBootDate aurait dû refuser un blob trop court")
	}
}

// TestInjectBootDate_NotIdempotent documente le contrat : appelé une 2e fois, le slot
// ne porte plus « jj-mm-aa » → refus (l'appel est prévu UNE fois sur une copie fraîche).
func TestInjectBootDate_NotIdempotent(t *testing.T) {
	basic := loadBasicBlob(t)
	if !injectBootDate(basic, testBootDate) {
		t.Fatal("première injection : échec inattendu")
	}
	if injectBootDate(basic, testBootDate) {
		t.Error("seconde injection : devait refuser (slot date déjà écrit)")
	}
}
