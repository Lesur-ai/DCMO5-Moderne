package to9p

import (
	"bytes"
	"os"
	"testing"
	"time"
)

func loadBasicBlob(t *testing.T) []byte {
	t.Helper()
	full, err := os.ReadFile(romTestPath())
	if err != nil {
		t.Fatalf("lecture ROM TO9+ : %v", err)
	}
	basic := append([]byte(nil), full[:romBasicSize]...)
	mon := append([]byte(nil), full[romBasicSize:]...)
	if rep := applyROMPatches(mon, basic); !rep.OK {
		t.Fatalf("ROM TO9+ de test non reconnue par applyROMPatches")
	}
	return basic
}

func TestInjectBootDate_WritesDateAndRoutine(t *testing.T) {
	basic := loadBasicBlob(t)
	date := time.Date(2026, time.January, 2, 0, 0, 0, 0, time.UTC)

	if !injectBootDate(basic, date) {
		t.Fatal("injectBootDate a échoué sur une ROM BASIC TO9+ reconnue")
	}
	if got := string(basic[bootDateStrOff : bootDateStrOff+8]); got != "02-01-26" {
		t.Fatalf("chaîne date = %q, want %q", got, "02-01-26")
	}
	if basic[bootDateTermOff] != 0x1f {
		t.Fatalf("octet terminal = 0x%02x, want 0x1f", basic[bootDateTermOff])
	}
	if got := basic[bootDateInitOff : bootDateInitOff+len(bootDateInitPatch)]; !bytes.Equal(got, bootDateInitPatch) {
		t.Fatalf("routine reset = % x, want % x", got, bootDateInitPatch)
	}
}

func TestInjectBootDate_FormatDDMMYY(t *testing.T) {
	basic := loadBasicBlob(t)
	date := time.Date(2025, time.December, 9, 0, 0, 0, 0, time.UTC)

	if !injectBootDate(basic, date) {
		t.Fatal("injectBootDate a échoué")
	}
	if got := string(basic[bootDateStrOff : bootDateStrOff+8]); got != "09-12-25" {
		t.Fatalf("chaîne date = %q, want %q", got, "09-12-25")
	}
}

func TestInjectBootDate_RejectsUnknownBlob(t *testing.T) {
	blob := make([]byte, romBasicSize)
	before := append([]byte(nil), blob...)

	if injectBootDate(blob, testBootDate) {
		t.Fatal("injectBootDate aurait dû refuser un blob non reconnu")
	}
	if !bytes.Equal(blob, before) {
		t.Fatal("injectBootDate a muté un blob non reconnu")
	}
}
