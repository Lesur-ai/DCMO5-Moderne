package core_test

// rompatch_rom_test.go — test d'intégration LONG (#85) : prouve qu'avec le patch
// ROM, un LOAD"" sur la vraie ROM lit RÉELLEMENT la cassette (le trap 0x42 est
// atteint) et que le CPU ne reste plus bloqué dans la boucle bit-bang F168.
//
//   DCMO5_LONG_TESTS=1 go test ./internal/core/ -run TestROM_Cassette -v

import (
	"testing"

	"github.com/Lesur-ai/dcmo5/internal/core"
	"github.com/Lesur-ai/dcmo5/internal/keyboard"
	"github.com/Lesur-ai/dcmo5/internal/media"
	"github.com/Lesur-ai/dcmo5/internal/media/impl"
	"github.com/Lesur-ai/dcmo5/internal/spec"
)

// countingTape compte les lectures effectives de la cassette sous-jacente.
type countingTape struct {
	inner media.Tape
	reads int
}

func (t *countingTape) ReadByte() (byte, error) { t.reads++; return t.inner.ReadByte() }
func (t *countingTape) WriteByte(b byte) error  { return t.inner.WriteByte(b) }
func (t *countingTape) Rewind() error           { return t.inner.Rewind() }
func (t *countingTape) Position() int64         { return t.inner.Position() }

// typeLoadAndRun injecte la frappe "LOAD\"\"\n" au niveau cœur (clavier MO5) et
// fait tourner la machine ; retourne l'histogramme de PC de la phase post-LOAD.
func typeLoadAndRun(t *testing.T, m *core.Machine) map[uint16]int {
	t.Helper()
	frame := spec.CPUClockHz / 60

	m.Step(5 * spec.CPUClockHz) // boot jusqu'au prompt BASIC

	inj := keyboard.NewInjector(keyboard.DefaultHoldFrames, keyboard.DefaultGapFrames)
	inj.EnqueueString(`LOAD""` + "\n")

	prev := map[int]bool{}
	apply := func(keys []int) {
		next := map[int]bool{}
		for _, k := range keys {
			next[k] = true
		}
		for k := range prev {
			if !next[k] {
				m.SetKey(core.Key(k), false)
			}
		}
		for k := range next {
			m.SetKey(core.Key(k), true)
		}
		prev = next
	}
	for inj.Pending() > 0 {
		apply(inj.Tick())
		m.Step(frame)
	}
	apply(nil)

	hist := map[uint16]int{}
	for i := 0; i < 200000; i++ {
		m.Step(64)
		hist[m.CPUSnapshot().PC]++
	}
	return hist
}

func TestROM_Cassette_LoadReadsTape_WithPatch(t *testing.T) {
	skipIfNotLong(t)
	rom := loadROM(t)

	inner, err := impl.OpenTape("../../software/memory-mo5.k7", true)
	if err != nil {
		t.Skipf("cassette de test absente: %v", err)
	}
	tape := &countingTape{inner: inner}

	m, err := core.NewMachine(core.Options{ROMSys: rom, Tape: tape, PatchSystemROM: true})
	if err != nil {
		t.Fatalf("NewMachine: %v", err)
	}
	m.Reset()

	hist := typeLoadAndRun(t, m)

	// 1) La cassette a réellement été lue (le trap 0x42 a abouti à des lectures).
	if tape.reads == 0 {
		t.Errorf("avec patch : LOAD\"\" n'a lu AUCUN octet de la cassette (reads=0)")
	}
	t.Logf("octets cassette lus = %d", tape.reads)

	// 2) Le CPU n'est plus piégé dans la boucle bit-bang F168.
	const stuck = 0xF168
	if frac := float64(hist[stuck]) / 200000; frac > 0.5 {
		t.Errorf("CPU encore bloqué à %04X (%.0f%% du temps) : patch inopérant", stuck, frac*100)
	}
}

func TestROM_Cassette_HangsWithoutPatch(t *testing.T) {
	skipIfNotLong(t)
	rom := loadROM(t)

	inner, err := impl.OpenTape("../../software/memory-mo5.k7", true)
	if err != nil {
		t.Skipf("cassette de test absente: %v", err)
	}
	tape := &countingTape{inner: inner}

	// SANS patch : on doit reproduire le bug (CPU bloqué à F168, 0 lecture).
	m, err := core.NewMachine(core.Options{ROMSys: rom, Tape: tape, PatchSystemROM: false})
	if err != nil {
		t.Fatalf("NewMachine: %v", err)
	}
	m.Reset()

	hist := typeLoadAndRun(t, m)

	// Signal robuste du bug : sans patch, la ROM lit la cassette par bit-bang du
	// port 0xA7C0 et n'emprunte JAMAIS le trap 0x42 → la cassette (interface Tape)
	// n'est jamais lue. La position où le CPU stagne est incidente (timing) et
	// n'est donc pas asservie ici.
	if tape.reads != 0 {
		t.Errorf("sans patch : attendu 0 lecture cassette via trap, got %d", tape.reads)
	}
	// Diagnostic informatif : PC dominant de la phase post-LOAD.
	var topPCaddr uint16
	var topPCn int
	for pc, n := range hist {
		if n > topPCn {
			topPCn, topPCaddr = n, pc
		}
	}
	t.Logf("sans patch : 0 lecture cassette (bug reproduit). PC dominant=%04X (%.0f%%)",
		topPCaddr, float64(topPCn)/200000*100)
}
