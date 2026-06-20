package engine

import (
	"testing"

	"github.com/Lesur-ai/dcmo5/internal/machine"
	"github.com/Lesur-ai/dcmo5/internal/spec"
)

// fakeDevice : machine synthétique pour tester la boucle du moteur en isolation
// (sans core ni machine concrète). RAM 64K, code = NOP (0x12), son réglable.
type fakeDevice struct {
	ram       [0x10000]byte
	sound     uint8
	traps     int
	cyclesSum int // somme des c reçus via OnInstructionCycles (invariant)
}

func newFake() *fakeDevice {
	d := &fakeDevice{}
	for i := range d.ram {
		d.ram[i] = 0x12 // NOP (2 cycles)
	}
	// Vecteur reset → 0x0000.
	d.ram[0xFFFE] = 0x00
	d.ram[0xFFFF] = 0x00
	return d
}

func (d *fakeDevice) Read8(a uint16) uint8     { return d.ram[a] }
func (d *fakeDevice) Write8(a uint16, v uint8) { d.ram[a] = v }
func (d *fakeDevice) Trap(code int)            { d.traps++ }
func (d *fakeDevice) SoundLevel() uint8        { return d.sound }
func (d *fakeDevice) FrameSize() (int, int)    { return 4, 2 }
func (d *fakeDevice) DecodeFrame(dst []uint32) {
	for i := range dst {
		dst[i] = 0xFF00FF00
	}
}
func (d *fakeDevice) OnInstructionCycles(c int, irq *machine.IRQLines) { d.cyclesSum += c }

// Vérification à la compilation que fakeDevice satisfait le contrat.
var _ Device = (*fakeDevice)(nil)

func TestEngineConsumesCyclesAndDeviceTiming(t *testing.T) {
	d := newFake()
	e := New(d, spec.AudioSampleRate)
	e.Reset()

	const want = 50000
	got := e.Step(want)
	if got < want {
		t.Errorf("Step a consommé %d cycles, attendu >= %d", got, want)
	}
	// Invariant : OnInstructionCycles est appelé pour chaque instruction avec son
	// coût → la somme doit égaler les cycles consommés.
	if d.cyclesSum != got {
		t.Errorf("OnInstructionCycles somme = %d, consommé = %d", d.cyclesSum, got)
	}
}

func TestEngineAudioSampling(t *testing.T) {
	d := newFake()
	d.sound = spec.AudioLevelMax // 0x3F
	e := New(d, spec.AudioSampleRate)
	e.Reset()

	e.Step(spec.CPUClockHz / 100)              // ~10 ms → ~SampleRate/100 échantillons
	buf := make([]uint8, spec.AudioSampleRate) // assez large
	n := e.DrainAudio(buf)
	if n == 0 {
		t.Fatal("aucun échantillon audio produit")
	}
	exp := spec.AudioSampleRate / 100
	if n < exp-2 || n > exp+2 {
		t.Errorf("nb échantillons = %d, attendu ~%d", n, exp)
	}
	for i := 0; i < n; i++ {
		if buf[i] != spec.AudioLevelMax {
			t.Fatalf("échantillon %d = 0x%02X, want 0x%02X", i, buf[i], spec.AudioLevelMax)
		}
	}
	if e.DrainAudio(buf) != 0 {
		t.Error("DrainAudio devrait avoir vidé le tampon")
	}
}

func TestEngineTrap(t *testing.T) {
	d := newFake()
	d.ram[0x0000] = 0x14 // opcode illégal MO5 → CPU.Step retourne négatif → Trap
	e := New(d, spec.AudioSampleRate)
	e.Reset()
	e.Step(200)
	if d.traps == 0 {
		t.Fatal("Trap non appelé sur opcode illégal")
	}
}

func TestEngineFramebuffer(t *testing.T) {
	d := newFake()
	e := New(d, spec.AudioSampleRate)
	w, h := e.FrameSize()
	if w != 4 || h != 2 {
		t.Fatalf("FrameSize = %dx%d, want 4x2", w, h)
	}
	dst := make([]uint32, w*h)
	e.FramebufferInto(dst)
	for i, px := range dst {
		if px != 0xFF00FF00 {
			t.Fatalf("pixel %d = 0x%08X (DecodeFrame non délégué)", i, px)
		}
	}
}
