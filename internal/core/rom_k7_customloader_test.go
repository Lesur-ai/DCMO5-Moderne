package core_test

// rom_k7_customloader_test.go — régression #5emeaxe (loader/protection à EXG).
//
// 5emeaxe.k7 : LOADM"" charge un stub auto-exécuté qui déchiffre/charge le reste
// via les traps cassette. Le déchiffreur utilise EXG Y,X (postbyte 0x21) ; tant
// que cet encodage inversé d'EXG n'était pas géré par le CPU, le déchiffrement
// produisait du faux code menant à SWI 00 ($229C) → reset, et la cassette ne
// lisait que ~386 octets. Avec le correctif EXG, le chargement progresse loin
// au-delà.
//
//   DCMO5_LONG_TESTS=1 go test ./internal/core/ -run TestROM_Cassette_CustomLoader -v

import (
	"testing"

	"github.com/Lesur-ai/dcmo5/internal/core"
	"github.com/Lesur-ai/dcmo5/internal/keyboard"
	"github.com/Lesur-ai/dcmo5/internal/media"
	"github.com/Lesur-ai/dcmo5/internal/media/impl"
	"github.com/Lesur-ai/dcmo5/internal/spec"
)

type k7ReadCounter struct {
	inner media.Tape
	reads int
}

func (s *k7ReadCounter) ReadByte() (byte, error) { s.reads++; return s.inner.ReadByte() }
func (s *k7ReadCounter) WriteByte(b byte) error  { return s.inner.WriteByte(b) }
func (s *k7ReadCounter) Rewind() error           { return s.inner.Rewind() }
func (s *k7ReadCounter) Position() int64         { return s.inner.Position() }

func TestROM_Cassette_CustomLoader_5emeAxe(t *testing.T) {
	skipIfNotLong(t)
	rom := loadROM(t)
	inner, err := impl.OpenTape("../../software/5emeaxe.k7", true)
	if err != nil {
		t.Skipf("5emeaxe.k7 absent: %v", err)
	}
	tape := &k7ReadCounter{inner: inner}
	m, _ := core.NewMachine(core.Options{ROMSys: rom, Tape: tape, PatchSystemROM: true})
	m.Reset()
	frame := spec.CPUClockHz / 60
	m.Step(5 * spec.CPUClockHz)

	inj := keyboard.NewInjector(keyboard.MO5Model(), keyboard.DefaultHoldFrames, keyboard.DefaultGapFrames)
	inj.EnqueueString(`LOADM""` + "\n")
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

	atReset := 0
	for i := 0; i < 60*40; i++ { // ~40 s
		m.Step(frame)
		if pc := m.CPUSnapshot().PC; pc == 0xF003 {
			atReset++
		}
	}
	t.Logf("5emeaxe : octets cassette lus = %d / %d ; échantillons PC@reset(F003)=%d", tape.reads, 52998, atReset)

	// Sans le correctif EXG, la lecture restait bloquée à ~386 octets (déchiffrement
	// faux → SWI 00 → reset). Avec le correctif, le loader progresse largement.
	if tape.reads < 5000 {
		t.Errorf("chargement bloqué : seulement %d octets lus (régression EXG Y,X ?)", tape.reads)
	}
}
