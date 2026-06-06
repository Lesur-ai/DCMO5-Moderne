package core_test

// iotrace_rom_test.go — diagnostic d'instrumentation E/S sur la vraie ROM MO5.
//
// SKIPPÉ par défaut : nécessite la ROM ET DCMO5_LONG_TESTS=1.
//
//   DCMO5_LONG_TESTS=1 go test ./internal/core/... -run TestROM_IOTrace -v
//
// Objectif (Phase P10.1, refs #84/#85/#86) : valider l'instrumentation
// bout-en-bout sur la vraie ROM et OBSERVER quels traps d'E/S sont atteints
// pendant un boot, avec une cassette montée. Le vrai scénario « LOAD" » (qui
// requiert la saisie clavier) se conduit via le binaire instrumenté :
//   DCMO5_IO_TRACE=1 dcmo5 -rom rom/mo5-v1.1.rom -tape software/<x>.k7 \
//       -exec 'LOAD\n'

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Lesur-ai/dcmo5/internal/core"
	"github.com/Lesur-ai/dcmo5/internal/media/impl"
	"github.com/Lesur-ai/dcmo5/internal/spec"
)

// TestROM_IOTrace_BootActivity boote la vraie ROM avec une cassette montée,
// trace les traps d'E/S, et vérifie l'invariant d'instrumentation :
// somme des compteurs == nombre de lignes de journal. Rapporte l'activité
// observée (notamment cassette 0x41/0x42) via t.Logf.
func TestROM_IOTrace_BootActivity(t *testing.T) {
	skipIfNotLong(t)
	rom := loadROM(t)

	// Cassette montée (contenu factice) pour observer toute lecture spontanée.
	tapePath := t.TempDir() + "/diag.k7"
	tape, _ := impl.NewTape(tapePath)
	for i := 0; i < 512; i++ {
		tape.WriteByte(byte(i))
	}
	tape.Rewind()
	tape.Close()
	tape2, _ := impl.OpenTape(tapePath, true)

	var buf bytes.Buffer
	m, err := core.NewMachine(core.Options{ROMSys: rom, Tape: tape2})
	if err != nil {
		t.Fatalf("NewMachine: %v", err)
	}
	m.Reset()
	m.EnableIOTrace(&buf)

	m.Step(5 * spec.CPUClockHz) // 5 secondes simulées

	counts := m.IOTraceCounts()
	total := 0
	for _, v := range counts {
		total += v
	}
	lines := strings.Count(buf.String(), "IOTRACE ")

	// Invariant d'instrumentation : 1 trap observé = 1 ligne journalisée.
	if total != lines {
		t.Errorf("incohérence trace : somme compteurs = %d, lignes journal = %d", total, lines)
	}

	t.Logf("Traps E/S observés sur 5s de boot ROM (cassette montée) :")
	for _, io := range []int{0x14, 0x15, 0x18, 0x41, 0x42, 0x45, 0x4B, 0x51} {
		t.Logf("  %02X %-12s : %d", io, ioTrapNameForTest(io), counts[io])
	}
	t.Logf("→ Diagnostic #85 : cassette (0x41+0x42) atteints au boot seul = %d "+
		"(un LOAD\" explicite se teste via le binaire instrumenté).",
		counts[0x41]+counts[0x42])
}

// ioTrapNameForTest duplique le libellé pour le rapport de test (le helper du
// paquet core n'est pas exporté).
func ioTrapNameForTest(io int) string {
	switch io {
	case 0x14:
		return "READSECTOR"
	case 0x15:
		return "WRITESECTOR"
	case 0x18:
		return "FORMATDISK"
	case 0x41:
		return "READBITK7"
	case 0x42:
		return "READOCTETK7"
	case 0x45:
		return "WRITEOCTETK7"
	case 0x4B:
		return "READPENXY"
	case 0x51:
		return "IMPRIME"
	default:
		return "UNKNOWN"
	}
}
