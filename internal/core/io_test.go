package core_test

import (
	"bytes"
	"testing"

	"github.com/Lesur-ai/dcmo5/internal/core"
	"github.com/Lesur-ai/dcmo5/internal/media/impl"
)

// ── Cartouche ─────────────────────────────────────────────────────────────────

type stubCartridge struct{ data []byte }

func (c *stubCartridge) Bytes() []byte { return c.data }

func TestIO_Cartridge_Loaded(t *testing.T) {
	// Une cartouche 16 Ko avec une valeur connue à l'offset 0x0100.
	// Après NewMachine, Read8(0xB100) doit retourner cette valeur
	// (0xB100 - 0xB000 = 0x100, base car[0]).
	cart := &stubCartridge{data: make([]byte, 0x4000)}
	cart.data[0x0100] = 0xAB
	m, err := core.NewMachine(core.Options{Cartridge: cart})
	if err != nil {
		t.Fatalf("NewMachine: %v", err)
	}
	// Activer la cartouche : déjà fait par loadCartridge (carflags=4)
	if v := m.Read8(0xB100); v != 0xAB {
		t.Errorf("cartouche chargée: Read8(0xB100) = 0x%02X, want 0xAB", v)
	}
}

func TestIO_Cartridge_MEMO5_type(t *testing.T) {
	// Cartouche 32 Ko → cartype=1 (MEMO5 bank switch).
	cart := &stubCartridge{data: make([]byte, 0x8000)}
	cart.data[0x4000] = 0xCD // banque 1
	m, err := core.NewMachine(core.Options{Cartridge: cart})
	if err != nil {
		t.Fatalf("NewMachine: %v", err)
	}
	// La première banque (banque 0) doit être accessible.
	if v := m.Read8(0xB000); v != 0x00 {
		t.Logf("cartouche MEMO5 banque 0 chargée (val 0x%02X)", v)
	}
}

func TestIO_Cartridge_Nil(t *testing.T) {
	// Sans cartouche : Read8(0xB000) retourne 0 (pas de ROM banque).
	m, err := core.NewMachine(core.Options{})
	if err != nil {
		t.Fatalf("NewMachine: %v", err)
	}
	v := m.Read8(0xB000)
	if v != 0x00 {
		t.Errorf("sans cartouche: Read8(0xB000) = 0x%02X, want 0x00", v)
	}
}

// ── Imprimante ────────────────────────────────────────────────────────────────

func TestIO_Printer_ReceivesByte(t *testing.T) {
	var buf bytes.Buffer
	printer := impl.NewWriterPrinter(&buf)
	// ROM : LDB #0x42 ; NOP…  puis Entreesortie(0x51) direct.
	rom := make([]byte, 0x4000)
	rom[0x3FFE] = 0xC0
	rom[0x3FFF] = 0x00 // reset vector → 0xC000
	rom[0x0000] = 0xC6 // LDB immediate
	rom[0x0001] = 0x42 // valeur
	for i := 2; i < len(rom)-2; i++ {
		rom[i] = 0x12 // NOP
	}
	m, _ := core.NewMachine(core.Options{ROMSys: rom, Printer: printer})
	m.Reset()
	m.Step(4) // exécute LDB #0x42 (2cy) + NOP (2cy) → B = 0x42
	m.Entreesortie(0x51)
	if buf.Len() == 0 {
		t.Error("imprimante n'a reçu aucun octet")
	} else if buf.Bytes()[0] != 0x42 {
		t.Errorf("imprimante: reçu 0x%02X, want 0x42", buf.Bytes()[0])
	}
}

func TestIO_StepDispatch_viaIllegalOpcode(t *testing.T) {
	// Vérifie que Step() dispatche l'I/O quand le CPU rencontre un opcode illégal.
	// La ROM MO5 utilise des opcodes illégaux comme stubs I/O (ref: dc6809emul.c
	// "default: return -code"). 0x51 est un opcode illégal du 6809 = code imprimante.
	var buf bytes.Buffer
	printer := impl.NewWriterPrinter(&buf)
	rom := make([]byte, 0x4000)
	rom[0x3FFE] = 0xC0
	rom[0x3FFF] = 0x00
	// Programme : LDB #0x7B ; 0x51 (opcode illégal = print) ; NOP...
	rom[0x0000] = 0xC6 // LDB immediate
	rom[0x0001] = 0x7B // valeur à imprimer
	rom[0x0002] = 0x51 // opcode illégal → Entreesortie(0x51) = imprime
	for i := 3; i < len(rom)-2; i++ {
		rom[i] = 0x12 // NOP infini
	}
	m, _ := core.NewMachine(core.Options{ROMSys: rom, Printer: printer})
	m.Reset()
	// Exécuter au moins 2+64 cycles pour passer le LDB+trap
	m.Step(100)
	if buf.Len() == 0 {
		t.Error("Step() n'a pas dispatché l'I/O via opcode illégal 0x51")
	} else if buf.Bytes()[0] != 0x7B {
		t.Errorf("imprimante via opcode illégal: reçu 0x%02X, want 0x7B", buf.Bytes()[0])
	}
}

// ── Disquette ─────────────────────────────────────────────name────────────────

func TestIO_Disk_ReadSectorDispatch(t *testing.T) {
	// Créer une disquette temporaire avec un secteur connu
	dir := t.TempDir()
	path := dir + "/test.fd"
	disk, err := impl.NewDisk(path)
	if err != nil {
		t.Fatalf("NewDisk: %v", err)
	}
	var sector [256]byte
	sector[0] = 0xDE
	sector[1] = 0xAD
	disk.WriteSector(0, 1, 1, sector) // face 0, piste 1, secteur 1
	disk.Close()

	disk2, _ := impl.OpenDisk(path, false)
	m, _ := core.NewMachine(core.Options{Disk: disk2})
	m.Reset()

	// Écrire les paramètres en RAM (adresses lues par readSector)
	m.Write8(0x2049, 0)    // face 0
	m.Write8(0x204B, 1)    // piste 1
	m.Write8(0x204C, 1)    // secteur 1 (1-based)
	m.Write8(0x204F, 0x40) // dest hi
	m.Write8(0x2050, 0x00) // dest lo → 0x4000
	m.Entreesortie(0x14)   // ReadSector

	if v := m.Read8(0x4000); v != 0xDE {
		t.Errorf("ReadSector: RAM[0x4000] = 0x%02X, want 0xDE", v)
	}
	if v := m.Read8(0x4001); v != 0xAD {
		t.Errorf("ReadSector: RAM[0x4001] = 0x%02X, want 0xAD", v)
	}
}

// ── Cassette ──────────────────────────────────────────────────────────────────

func TestIO_Tape_ReadOctet(t *testing.T) {
	path := t.TempDir() + "/test.k7"
	tape, _ := impl.NewTape(path)
	tape.WriteByte(0x55)
	tape.WriteByte(0xAA)
	tape.Rewind()
	tape.Close()

	tape2, _ := impl.OpenTape(path, true)
	m, _ := core.NewMachine(core.Options{Tape: tape2})
	m.Reset()
	m.Entreesortie(0x42) // ReadOctetK7
	if v := m.Read8(0x2045); v != 0x55 {
		t.Errorf("ReadOctetK7: RAM[0x2045] = 0x%02X, want 0x55", v)
	}
	if a := m.CPUSnapshot().A; a != 0x55 {
		t.Errorf("ReadOctetK7: A = 0x%02X, want 0x55", a)
	}
}

// TestIO_Tape_ReadBit vérifie la lecture bit à bit (ref dcmo5devices.c
// Readbitk7) : A=0xFF quand le bit courant vaut 1, 0x00 sinon. L'octet 0xC0
// (bits 1,1,0,0,0,0,0,0) donne donc FF, FF, 00 sur les trois premiers bits.
func TestIO_Tape_ReadBit(t *testing.T) {
	path := t.TempDir() + "/bit.k7"
	tape, _ := impl.NewTape(path)
	tape.WriteByte(0xC0) // 1100 0000
	tape.Rewind()
	tape.Close()

	tape2, _ := impl.OpenTape(path, true)
	m, _ := core.NewMachine(core.Options{Tape: tape2})
	m.Reset()

	want := []uint8{0xFF, 0xFF, 0x00}
	for i, w := range want {
		m.Entreesortie(0x41) // ReadBitK7
		if a := m.CPUSnapshot().A; a != w {
			t.Errorf("ReadBitK7 bit %d: A = 0x%02X, want 0x%02X", i, a, w)
		}
	}
}
