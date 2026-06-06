package core_test

// hotswap_test.go — montage/éjection des médias à chaud (P9.2).
// Tests observables : chaque cas vérifie un effet concret sur la mémoire ou les
// registres après l'opération, pas seulement l'absence de panique.

import (
	"testing"

	"github.com/Lesur-ai/dcmo5/internal/core"
	"github.com/Lesur-ai/dcmo5/internal/media/impl"
)

// romWithReset construit une ROM système 16 Ko dont le vecteur reset pointe
// vers addr. Permet d'observer un reset CPU via PC.
func romWithReset(addr uint16) []byte {
	rom := make([]byte, 0x4000)
	rom[0x3FFE] = byte(addr >> 8)
	rom[0x3FFF] = byte(addr)
	return rom
}

// ── Cassette ──────────────────────────────────────────────────────────────────

func TestMountTape_ReadsNewMedia(t *testing.T) {
	path := t.TempDir() + "/new.k7"
	tape, _ := impl.NewTape(path)
	tape.WriteByte(0x55)
	tape.Rewind()
	tape.Close()

	// Machine démarrée SANS cassette.
	m, _ := core.NewMachine(core.Options{})
	m.Reset()

	// Avant montage : lire la cassette ne doit rien produire.
	m.Write8(0x2045, 0x00)
	m.Entreesortie(0x42)
	if v := m.Read8(0x2045); v != 0x00 {
		t.Fatalf("sans cassette: 0x2045 = 0x%02X, want inchangé 0x00", v)
	}

	// Monter une cassette à chaud puis lire un octet.
	tape2, _ := impl.OpenTape(path, true)
	m.MountTape(tape2)
	m.Entreesortie(0x42)
	if v := m.Read8(0x2045); v != 0x55 {
		t.Errorf("après MountTape: 0x2045 = 0x%02X, want 0x55", v)
	}
	if a := m.CPUSnapshot().A; a != 0x55 {
		t.Errorf("après MountTape: A = 0x%02X, want 0x55", a)
	}
}

func TestEjectTape_StopsReading(t *testing.T) {
	path := t.TempDir() + "/eject.k7"
	tape, _ := impl.NewTape(path)
	tape.WriteByte(0x55)
	tape.Rewind()
	tape.Close()

	tape2, _ := impl.OpenTape(path, true)
	m, _ := core.NewMachine(core.Options{Tape: tape2})
	m.Reset()
	m.Entreesortie(0x42)
	if v := m.Read8(0x2045); v != 0x55 {
		t.Fatalf("préparation: 0x2045 = 0x%02X, want 0x55", v)
	}

	// Éjecter : une nouvelle lecture ne doit plus rien produire.
	m.EjectTape()
	m.Write8(0x2045, 0xEE)
	m.Entreesortie(0x42)
	if v := m.Read8(0x2045); v != 0xEE {
		t.Errorf("après EjectTape: 0x2045 = 0x%02X, want inchangé 0xEE", v)
	}
}

// ── Cartouche ─────────────────────────────────────────────────────────────────

func TestMountCartridge_MapsBank(t *testing.T) {
	// Machine sans cartouche : 0xB100 ne renvoie pas la valeur cartouche.
	m, _ := core.NewMachine(core.Options{})
	m.Reset()
	if v := m.Read8(0xB100); v == 0xAB {
		t.Fatalf("sans cartouche: 0xB100 = 0xAB inattendu")
	}

	// Monter une cartouche 16 Ko avec une valeur connue à l'offset 0x100.
	cart := &stubCartridge{data: make([]byte, 0x4000)}
	cart.data[0x0100] = 0xAB
	m.MountCartridge(cart)

	if v := m.Read8(0xB100); v != 0xAB {
		t.Errorf("après MountCartridge: 0xB100 = 0x%02X, want 0xAB", v)
	}
}

func TestEjectCartridge_DisablesBank(t *testing.T) {
	cart := &stubCartridge{data: make([]byte, 0x4000)}
	cart.data[0x0100] = 0xAB
	m, _ := core.NewMachine(core.Options{Cartridge: cart})
	m.Reset()
	if v := m.Read8(0xB100); v != 0xAB {
		t.Fatalf("préparation: 0xB100 = 0x%02X, want 0xAB", v)
	}

	// Éjecter : le banc cartouche est désactivé, 0xB100 ne renvoie plus 0xAB.
	m.EjectCartridge()
	if v := m.Read8(0xB100); v == 0xAB {
		t.Errorf("après EjectCartridge: 0xB100 = 0xAB, banc cartouche non désactivé")
	}
}

func TestMountCartridge_NoResidueFromPrevious(t *testing.T) {
	// Première cartouche : 0xAB en 0xB100.
	first := &stubCartridge{data: make([]byte, 0x4000)}
	first.data[0x0100] = 0xAB
	m, _ := core.NewMachine(core.Options{})
	m.Reset()
	m.MountCartridge(first)
	if v := m.Read8(0xB100); v != 0xAB {
		t.Fatalf("première cartouche: 0xB100 = 0x%02X, want 0xAB", v)
	}

	// Seconde cartouche, vierge à cet offset : aucun résidu de la première.
	second := &stubCartridge{data: make([]byte, 0x4000)}
	m.MountCartridge(second)
	if v := m.Read8(0xB100); v != 0x00 {
		t.Errorf("seconde cartouche: 0xB100 = 0x%02X (résidu), want 0x00", v)
	}
}

func TestMountCartridge_ResetsCPU(t *testing.T) {
	// ROM système avec vecteur reset → 0xE000.
	m, _ := core.NewMachine(core.Options{ROMSys: romWithReset(0xE000)})
	m.Reset()
	if pc := m.CPUSnapshot().PC; pc != 0xE000 {
		t.Fatalf("reset initial: PC = 0x%04X, want 0xE000", pc)
	}
	// Faire avancer le CPU pour déplacer PC (NOP=0x12 partout via RAM user).
	m.Step(50)
	moved := m.CPUSnapshot().PC

	// Monter une cartouche doit relancer la machine (reset → PC = vecteur).
	cart := &stubCartridge{data: make([]byte, 0x4000)}
	m.MountCartridge(cart)
	if pc := m.CPUSnapshot().PC; pc != 0xE000 {
		t.Errorf("après MountCartridge: PC = 0x%04X (départ 0xE000, avait bougé à 0x%04X), want reset 0xE000", pc, moved)
	}
}

func TestMountCartridge_ResetsRAM(t *testing.T) {
	// Valeur de la RAM user après un reset propre (référence).
	ref, _ := core.NewMachine(core.Options{})
	ref.Reset()
	const addr = 0x5000
	resetVal := ref.Read8(addr)

	// Machine « sale » : on écrit une valeur distincte de la valeur de reset.
	m, _ := core.NewMachine(core.Options{})
	m.Reset()
	m.Write8(addr, resetVal^0xFF)
	if m.Read8(addr) == resetVal {
		t.Fatalf("préparation: la valeur écrite doit différer de la valeur reset")
	}

	// Monter une cartouche doit réinitialiser la RAM (ref Loadmemo).
	cart := &stubCartridge{data: make([]byte, 0x4000)}
	m.MountCartridge(cart)
	if v := m.Read8(addr); v != resetVal {
		t.Errorf("après MountCartridge: RAM[0x%04X] = 0x%02X (RAM périmée), want 0x%02X (reset)", addr, v, resetVal)
	}
}

// TestInitprog_KeepsRAM vérifie que Initprog (reset doux) PRÉSERVE la RAM et
// recharge le vecteur reset, contrairement à Reset (qui efface la RAM).
func TestInitprog_KeepsRAM(t *testing.T) {
	m, _ := core.NewMachine(core.Options{ROMSys: romWithReset(0xE000)})
	m.Reset()
	m.Write8(0x5000, 0x42) // valeur en RAM utilisateur

	m.Initprog()
	if v := m.Read8(0x5000); v != 0x42 {
		t.Errorf("Initprog: RAM[0x5000] = 0x%02X, want 0x42 (RAM préservée)", v)
	}
	if pc := m.CPUSnapshot().PC; pc != 0xE000 {
		t.Errorf("Initprog: PC = 0x%04X, want 0xE000 (vecteur reset rechargé)", pc)
	}

	// Contraste : Reset efface bien la RAM.
	m.Write8(0x5000, 0x42)
	m.Reset()
	if v := m.Read8(0x5000); v == 0x42 {
		t.Error("Reset devrait effacer la RAM (RAM[0x5000] inchangée)")
	}
}

// ── Disquette ─────────────────────────────────────────────────────────────────

func TestMountDisk_ReadsNewMedia(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/hot.fd"
	disk, err := impl.NewDisk(path)
	if err != nil {
		t.Fatalf("NewDisk: %v", err)
	}
	var sector [256]byte
	sector[0] = 0xDE
	disk.WriteSector(0, 1, 1, sector)
	disk.Close()

	// Machine sans disquette.
	m, _ := core.NewMachine(core.Options{})
	m.Reset()

	// Monter la disquette à chaud puis lire le secteur connu.
	disk2, _ := impl.OpenDisk(path, false)
	m.MountDisk(disk2)
	m.Write8(0x2049, 0)    // face 0
	m.Write8(0x204B, 1)    // piste 1
	m.Write8(0x204C, 1)    // secteur 1
	m.Write8(0x204F, 0x40) // dest hi
	m.Write8(0x2050, 0x00) // dest lo → 0x4000
	m.Entreesortie(0x14)   // ReadSector
	if v := m.Read8(0x4000); v != 0xDE {
		t.Errorf("après MountDisk: RAM[0x4000] = 0x%02X, want 0xDE", v)
	}
}

func TestEjectDisk_StopsReading(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/eject.fd"
	disk, _ := impl.NewDisk(path)
	var sector [256]byte
	sector[0] = 0xDE
	disk.WriteSector(0, 1, 1, sector)
	disk.Close()

	disk2, _ := impl.OpenDisk(path, false)
	m, _ := core.NewMachine(core.Options{Disk: disk2})
	m.Reset()

	// Éjecter : la lecture de secteur ne doit plus écrire en RAM.
	m.EjectDisk()
	m.Write8(0x4000, 0x11)
	m.Write8(0x2049, 0)
	m.Write8(0x204B, 1)
	m.Write8(0x204C, 1)
	m.Write8(0x204F, 0x40)
	m.Write8(0x2050, 0x00)
	m.Entreesortie(0x14)
	if v := m.Read8(0x4000); v != 0x11 {
		t.Errorf("après EjectDisk: RAM[0x4000] = 0x%02X, want inchangé 0x11", v)
	}
}
