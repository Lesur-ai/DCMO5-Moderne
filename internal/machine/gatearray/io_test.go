package gatearray_test

// Tests d'intégration des traps d'E/S TO8D (#115) : disque secteur (params
// 0x6049–0x6050), cassette octet (0x2045), crayon/souris/clic, imprimante, son.
// Un vrai CPU 6809 est attaché au gate-array (les handlers lisent/écrivent ses
// registres) ; les médias sont des mocks.

import (
	"io"
	"testing"

	"github.com/Lesur-ai/dcmo5/internal/cpu6809"
	"github.com/Lesur-ai/dcmo5/internal/machine/gatearray"
)

func newGAWithCPU() (*gatearray.GateArray, *cpu6809.CPU) {
	g := newGA()
	cpu := cpu6809.New(g)
	g.AttachCPU(cpu)
	return g, cpu
}

// ── Médias mock ───────────────────────────────────────────────────────────────

type fakeDisk struct {
	sector    [256]byte
	formatted bool
}

func (d *fakeDisk) ReadSector(_, _, _ int) ([256]byte, error)  { return d.sector, nil }
func (d *fakeDisk) WriteSector(_, _, _ int, v [256]byte) error { d.sector = v; return nil }
func (d *fakeDisk) FormatUnit(_ int) error                     { d.formatted = true; return nil }

type fakeTape struct {
	data    []byte
	pos     int
	written []byte
}

func (t *fakeTape) ReadByte() (byte, error) {
	if t.pos >= len(t.data) {
		return 0, io.EOF
	}
	b := t.data[t.pos]
	t.pos++
	return b, nil
}
func (t *fakeTape) WriteByte(b byte) error { t.written = append(t.written, b); return nil }
func (t *fakeTape) Rewind() error          { t.pos = 0; return nil }
func (t *fakeTape) Position() int64        { return int64(t.pos) }

type fakePrinter struct{ out []byte }

func (p *fakePrinter) WriteByte(b byte) error { p.out = append(p.out, b); return nil }

// ── Son ───────────────────────────────────────────────────────────────────────

func TestSoundLevel(t *testing.T) {
	g := newGA()
	g.Write8(0xE7CF, 0x04) // sélectionne le registre musique
	g.Write8(0xE7CD, 0x2A) // niveau son
	if v := g.SoundLevel(); v != 0x2A {
		t.Errorf("SoundLevel = 0x%02X, want 0x2A", v)
	}
	g.Write8(0xE7CF, 0x00) // bit musique absent → e7cd ne change plus le son
	g.Write8(0xE7CD, 0x3F)
	if v := g.SoundLevel(); v != 0x2A {
		t.Errorf("SoundLevel = 0x%02X, want inchangé 0x2A", v)
	}
}

// ── Disque ────────────────────────────────────────────────────────────────────

func TestTrapDiskReadSector(t *testing.T) {
	g, _ := newGAWithCPU()
	disk := &fakeDisk{}
	for i := range disk.sector {
		disk.sector[i] = byte(i)
	}
	g.MountDisk(disk)
	g.Write8(0x6049, 0) // unité
	g.Write8(0x604A, 0)
	g.Write8(0x604B, 1)    // piste
	g.Write8(0x604C, 2)    // secteur
	g.Write8(0x604F, 0x80) // adresse de destination = 0x8000
	g.Write8(0x6050, 0x00)
	g.Trap(0x14)
	for i := 0; i < 256; i++ {
		if v := g.Read8(0x8000 + uint16(i)); v != byte(i) {
			t.Fatalf("secteur[%d] = 0x%02X, want 0x%02X", i, v, byte(i))
		}
	}
}

func TestTrapDiskNoMedia(t *testing.T) {
	g, cpu := newGAWithCPU()
	g.Trap(0x14)
	if v := g.Read8(0x604E); v != 70 {
		t.Errorf("0x604e = %d, want 70 (erreur 71-1, lecteur non prêt)", v)
	}
	if cpu.RegCC()&0x01 == 0 {
		t.Error("carry devrait être positionné (erreur disque)")
	}
}

func TestTrapDiskWriteThenFormat(t *testing.T) {
	g, _ := newGAWithCPU()
	disk := &fakeDisk{}
	g.MountDisk(disk)
	// Remplir la source 0x8000 puis écrire le secteur.
	for i := 0; i < 256; i++ {
		g.Write8(0x8000+uint16(i), byte(255-i))
	}
	g.Write8(0x6049, 0)
	g.Write8(0x604A, 0)
	g.Write8(0x604B, 0)
	g.Write8(0x604C, 1)
	g.Write8(0x604F, 0x80)
	g.Write8(0x6050, 0x00)
	g.Trap(0x15) // writeSector
	if disk.sector[0] != 255 || disk.sector[255] != 0 {
		t.Errorf("secteur écrit incohérent: [0]=%d [255]=%d", disk.sector[0], disk.sector[255])
	}
	g.Trap(0x18) // formatDisk
	if !disk.formatted {
		t.Error("FormatUnit non appelé sur trap 0x18")
	}
}

// ── Cassette ──────────────────────────────────────────────────────────────────

func TestTrapCassetteRead(t *testing.T) {
	g, cpu := newGAWithCPU()
	// 0x2045 est dans l'espace ROM en TO8D : on active le recouvrement RAM
	// write-enabled (e7e6) pour que le firmware (et ce test) puisse y écrire.
	g.Write8(0xE7E6, 0x60)
	tape := &fakeTape{data: []byte{0xAB, 0xCD}}
	g.MountTape(tape)
	g.Trap(0x42)
	if cpu.RegA() != 0xAB {
		t.Errorf("A = 0x%02X, want 0xAB (1er octet)", cpu.RegA())
	}
	if v := g.Read8(0x2045); v != 0xAB {
		t.Errorf("0x2045 = 0x%02X, want 0xAB", v)
	}
	g.Trap(0x42)
	if cpu.RegA() != 0xCD {
		t.Errorf("A = 0x%02X, want 0xCD (2e octet)", cpu.RegA())
	}
}

func TestTrapCassetteWrite(t *testing.T) {
	g, cpu := newGAWithCPU()
	tape := &fakeTape{}
	g.MountTape(tape)
	cpu.SetRegA(0x5A)
	g.Trap(0x45) // writeOctetK7
	if len(tape.written) != 1 || tape.written[0] != 0x5A {
		t.Errorf("cassette écrit %v, want [0x5A]", tape.written)
	}
}

// ── Crayon / souris / imprimante ──────────────────────────────────────────────

func TestTrapPen(t *testing.T) {
	g, cpu := newGAWithCPU()
	g.SetPointer(100, 50, false)
	g.Trap(0x4b)
	if cpu.RegX() != 50 { // mode par défaut (320) → x divisé par 2
		t.Errorf("X = %d, want 50 (100>>1)", cpu.RegX())
	}
	if cpu.RegY() != 50 {
		t.Errorf("Y = %d, want 50", cpu.RegY())
	}
	if cpu.RegCC()&0x01 != 0 {
		t.Error("carry devrait être clear (détection OK)")
	}
}

func TestTrapPenOutOfBounds(t *testing.T) {
	g, cpu := newGAWithCPU()
	g.SetPointer(700, 50, false) // x >= 640
	g.Trap(0x4b)
	if cpu.RegCC()&0x01 == 0 {
		t.Error("carry devrait être positionné (hors limites)")
	}
}

func TestTrapPen80Columns(t *testing.T) {
	g, cpu := newGAWithCPU()
	g.Write8(0xE7DC, 0x2a) // mode 80 colonnes → pleine résolution X
	g.SetPointer(600, 50, false)
	g.Trap(0x4b)
	if cpu.RegX() != 600 {
		t.Errorf("X = %d, want 600 (80 colonnes, pas de division)", cpu.RegX())
	}
}

func TestTrapMousePosition(t *testing.T) {
	g, cpu := newGAWithCPU()
	g.Write8(0xE7DC, 0x2a) // 80 colonnes
	g.SetPointer(300, 80, false)
	g.Trap(0x4e) // souris : registres X/Y + RAM 0x60d8 (x) / 0x60d6 (y)
	if cpu.RegX() != 300 {
		t.Errorf("X = %d, want 300", cpu.RegX())
	}
	if v := uint16(g.Read8(0x60D8))<<8 | uint16(g.Read8(0x60D9)); v != 300 {
		t.Errorf("RAM 0x60d8 = %d, want 300", v)
	}
	if v := uint16(g.Read8(0x60D6))<<8 | uint16(g.Read8(0x60D7)); v != 80 {
		t.Errorf("RAM 0x60d6 = %d, want 80", v)
	}
}

func TestTrapMouseButton(t *testing.T) {
	g, cpu := newGAWithCPU()
	g.SetPointer(0, 0, false)
	g.Trap(0x52)
	if cpu.RegA() != 3 {
		t.Errorf("A = %d, want 3 (bouton relâché)", cpu.RegA())
	}
	g.SetPointer(0, 0, true)
	g.Trap(0x52)
	if cpu.RegA() != 0 {
		t.Errorf("A = %d, want 0 (bouton pressé)", cpu.RegA())
	}
	if cpu.RegCC()&0x05 != 0x05 {
		t.Errorf("CC = 0x%02X, want bits 0 et 2 armés (clic)", cpu.RegCC())
	}
}

func TestTrapPrinter(t *testing.T) {
	g, cpu := newGAWithCPU()
	pr := &fakePrinter{}
	g.MountPrinter(pr)
	cpu.SetRegB(0x41)
	g.Trap(0x51)
	if len(pr.out) != 1 || pr.out[0] != 0x41 {
		t.Errorf("imprimante reçu %v, want [0x41]", pr.out)
	}
	if cpu.RegCC()&0x01 != 0 {
		t.Error("carry devrait être clear après impression")
	}
}
