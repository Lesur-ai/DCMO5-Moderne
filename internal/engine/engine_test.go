package engine

import (
	"testing"

	"github.com/Lesur-ai/dcmo5/internal/machine"
	"github.com/Lesur-ai/dcmo5/internal/spec"
)

// fakeDevice : machine synthétique pour tester la boucle du moteur en isolation
// (sans core ni machine concrète). RAM 64K, code = NOP (0x12), son réglable.
type fakeDevice struct {
	ram         [0x10000]byte
	sound       uint8
	traps       int
	cyclesSum   int  // somme des c reçus via OnInstructionCycles (invariant)
	assertTimer bool // si vrai, asserte IRQTimer à chaque instruction (test livraison IRQ)
	lines       []int
}

type renderedSegment struct {
	line  int
	cycle int
}

type fakeSegmentDevice struct {
	*fakeDevice
	segments []renderedSegment
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
func (d *fakeDevice) OnInstructionCycles(c int, irq *machine.IRQLines) {
	d.cyclesSum += c
	if d.assertTimer {
		irq.Assert(machine.IRQTimer)
	}
}
func (d *fakeDevice) RenderVideoLine(n int) { d.lines = append(d.lines, n) }
func (d *fakeSegmentDevice) RenderVideoSegments(line, cycle int) {
	d.segments = append(d.segments, renderedSegment{line: line, cycle: cycle})
}

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
	d.sound = 0x3F // niveau audio max (6 bits) ; valeur MO5 reproduite localement
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
		if buf[i] != 0x3F {
			t.Fatalf("échantillon %d = 0x%02X, want 0x%02X", i, buf[i], 0x3F)
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

func TestEngineRendersVideoLineOnBoundary(t *testing.T) {
	d := newFake()
	e := New(d, spec.AudioSampleRate)
	e.Reset()
	e.Step(spec.VideoCyclesPerLine)
	if len(d.lines) != 1 || d.lines[0] != 0 {
		t.Fatalf("lignes rendues = %v, want [0]", d.lines)
	}
}

func TestEngineRendersVideoSegmentsDuringLine(t *testing.T) {
	d := &fakeSegmentDevice{fakeDevice: newFake()}
	e := New(d, spec.AudioSampleRate)
	e.Reset()
	e.Step(spec.VideoCyclesPerLine)
	if len(d.segments) == 0 {
		t.Fatal("aucun segment rendu")
	}
	last := d.segments[len(d.segments)-1]
	if last.line != 0 || last.cycle != spec.VideoCyclesPerLine {
		t.Fatalf("dernier segment = %+v, want line=0 cycle=%d", last, spec.VideoCyclesPerLine)
	}
	if len(d.lines) != 0 {
		t.Fatalf("fallback ligne appelé malgré renderer segmentaire : %v", d.lines)
	}
}

// TestEngineDeliversAssertedIRQ vérifie qu'une ligne d'IRQ assertée par le Device est
// livrée au CPU une fois le masque I levé (cas niveau masqué-puis-démasqué). Le
// programme démasque I (ANDCC #$EF) puis boucle ; l'IRQTimer assertée doit faire
// vectoriser le CPU vers son handler (BRA sur place à 0x2000).
func TestEngineDeliversAssertedIRQ(t *testing.T) {
	d := newFake()
	d.assertTimer = true
	// LDS #$1FFF : place la pile en milieu de RAM (sinon S=0 et l'empilement de
	// l'IRQ, qui descend depuis 0xFFFF, écraserait le vecteur 0xFFF8).
	d.ram[0x0000] = 0x10
	d.ram[0x0001] = 0xCE
	d.ram[0x0002] = 0x1F
	d.ram[0x0003] = 0xFF
	d.ram[0x0004] = 0x1C // ANDCC #imm
	d.ram[0x0005] = 0xEF // efface le bit I (0x10) → IRQ démasquée
	// 0x0006.. : NOP (déjà rempli) jusqu'à la prise d'IRQ.
	d.ram[0xFFF8] = 0x20 // vecteur IRQ → 0x2000
	d.ram[0xFFF9] = 0x00
	d.ram[0x2000] = 0x20 // BRA
	d.ram[0x2001] = 0xFE // offset -2 → boucle sur 0x2000 (handler stable)

	e := New(d, spec.AudioSampleRate)
	e.Reset()
	e.Step(1000)

	if pc := e.CPU().Snapshot().PC; pc != 0x2000 {
		t.Fatalf("PC = 0x%04X, attendu 0x2000 (IRQ assertée non livrée au CPU)", pc)
	}
}
