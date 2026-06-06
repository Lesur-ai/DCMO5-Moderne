package core_test

// Tests du contrôleur de disquette CD90-640 : mapping ROM (0xA000..0xA7BF) et
// sémantique d'erreur Diskerror (code en 0x204E = n-1, carry positionné).

import (
	"os"
	"testing"

	"github.com/Lesur-ai/dcmo5/internal/core"
	"github.com/Lesur-ai/dcmo5/internal/media/impl"
)

// makeSingleDensityFD crée une disquette simple densité (40 pistes = 163840 o).
func makeSingleDensityFD(t *testing.T) string {
	t.Helper()
	p := t.TempDir() + "/sd.fd"
	f, err := os.Create(p)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := f.Truncate(163840); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	f.Close()
	return p
}

func TestDiskController_ROMMapping(t *testing.T) {
	rom := make([]byte, 0x7C0) // 1984 o, taille de la vraie CD90-640
	rom[0x000] = 0xAB
	rom[0x7BF] = 0xCD
	m, err := core.NewMachine(core.Options{DiskControllerROM: rom})
	if err != nil {
		t.Fatalf("NewMachine: %v", err)
	}
	if v := m.Read8(0xA000); v != 0xAB {
		t.Errorf("Read8(0xA000) = 0x%02X, want 0xAB (début ROM contrôleur)", v)
	}
	if v := m.Read8(0xA7BF); v != 0xCD {
		t.Errorf("Read8(0xA7BF) = 0x%02X, want 0xCD (fin ROM contrôleur)", v)
	}
}

func TestDiskController_NoROM_ReadsZero(t *testing.T) {
	m, _ := core.NewMachine(core.Options{})
	if v := m.Read8(0xA000); v != 0x00 {
		t.Errorf("sans contrôleur : Read8(0xA000) = 0x%02X, want 0x00", v)
	}
}

// readDiskErr lit le code d'erreur disque exposé en 0x204E (= n-1) et l'état du
// carry après un trap disque.
func diskErrCode(m *core.Machine) (code uint8, carry bool) {
	return m.Read8(0x204E), m.CPUSnapshot().CC&0x01 != 0
}

func TestDiskError_NoDisk_Code71(t *testing.T) {
	m, _ := core.NewMachine(core.Options{})
	m.Reset()
	m.Entreesortie(0x14) // READSECTOR sans disquette → Diskerror(71)
	code, carry := diskErrCode(m)
	if code != 70 || !carry {
		t.Errorf("pas de disquette : 0x204E=%d carry=%v, want 70 (71-1) + carry", code, carry)
	}
}

func TestDiskError_BadParams_Code53(t *testing.T) {
	path := t.TempDir() + "/e.fd"
	disk, _ := impl.NewDisk(path)
	m, _ := core.NewMachine(core.Options{Disk: disk})
	m.Reset()

	// Piste 80 (> 79) → Diskerror(53).
	m.Write8(0x2049, 0)
	m.Write8(0x204A, 0)
	m.Write8(0x204B, 80)
	m.Write8(0x204C, 1)
	m.Entreesortie(0x14)
	if code, carry := diskErrCode(m); code != 52 || !carry {
		t.Errorf("piste 80 : 0x204E=%d carry=%v, want 52 (53-1) + carry", code, carry)
	}

	// 0x204A ≠ 0 → Diskerror(53) (octet de poids fort piste, doit être nul).
	m.Reset()
	m.Write8(0x2049, 0)
	m.Write8(0x204A, 1)
	m.Write8(0x204B, 0)
	m.Write8(0x204C, 1)
	m.Entreesortie(0x14)
	if code, _ := diskErrCode(m); code != 52 {
		t.Errorf("0x204A≠0 : 0x204E=%d, want 52", code)
	}
}

func TestDiskError_OutOfCapacity_Code53(t *testing.T) {
	// Disquette simple densité (40 pistes) : lire piste 50 → hors capacité → 53.
	path := makeSingleDensityFD(t)
	disk, err := impl.OpenDisk(path, false)
	if err != nil {
		t.Fatalf("OpenDisk: %v", err)
	}
	m, _ := core.NewMachine(core.Options{Disk: disk})
	m.Reset()
	m.Write8(0x2049, 0)
	m.Write8(0x204A, 0)
	m.Write8(0x204B, 50) // ≤ 79 (param valide) mais hors capacité du fichier 40 pistes
	m.Write8(0x204C, 1)
	m.Write8(0x204F, 0x60)
	m.Write8(0x2050, 0x00)
	m.Entreesortie(0x14)
	if code, carry := diskErrCode(m); code != 52 || !carry {
		t.Errorf("piste 50 hors capacité : 0x204E=%d carry=%v, want 52 + carry", code, carry)
	}
}

func TestDiskError_WriteProtected_Code72(t *testing.T) {
	// Disquette ouverte en lecture seule → écriture (trap 0x15) → Diskerror(72).
	path := makeSingleDensityFD(t)
	disk, err := impl.OpenDisk(path, true) // readOnly
	if err != nil {
		t.Fatalf("OpenDisk: %v", err)
	}
	m, _ := core.NewMachine(core.Options{Disk: disk})
	m.Reset()
	m.Write8(0x2049, 0)
	m.Write8(0x204A, 0)
	m.Write8(0x204B, 0)
	m.Write8(0x204C, 1)
	m.Write8(0x204F, 0x60)
	m.Write8(0x2050, 0x00)
	m.Entreesortie(0x15) // WRITESECTOR
	if code, carry := diskErrCode(m); code != 71 || !carry {
		t.Errorf("écriture protégée : 0x204E=%d carry=%v, want 71 (72-1) + carry", code, carry)
	}
}

// TestDiskRead_RealFD_SingleDensity lit le secteur 1 d'une vraie disquette
// simple densité (163840 o) via le trap 0x14 et compare aux octets bruts du
// fichier. Preuve end-to-end du chemin de lecture sur un .fd réel. Skip si absent.
func TestDiskRead_RealFD_SingleDensity(t *testing.T) {
	const fd = "../../software/dos-5p25-mo5.fd"
	raw, err := os.ReadFile(fd)
	if err != nil {
		t.Skipf("disquette de test absente: %v", err)
	}
	if len(raw) != 163840 {
		t.Skipf("taille inattendue (%d), test prévu pour 163840 o", len(raw))
	}
	disk, err := impl.OpenDisk(fd, true)
	if err != nil {
		t.Fatalf("OpenDisk (densité variable doit accepter 163840 o): %v", err)
	}
	m, _ := core.NewMachine(core.Options{Disk: disk})
	m.Reset()
	// Secteur (unité 0, piste 0, secteur 1) → offset fichier 0.
	m.Write8(0x2049, 0)
	m.Write8(0x204A, 0)
	m.Write8(0x204B, 0)
	m.Write8(0x204C, 1)
	m.Write8(0x204F, 0x60)
	m.Write8(0x2050, 0x00) // destination 0x6000
	m.Entreesortie(0x14)

	if _, carry := diskErrCode(m); carry {
		t.Fatalf("lecture secteur 1 d'un .fd réel : carry positionné (erreur disque)")
	}
	for i := 0; i < 256; i++ {
		if got, want := m.Read8(0x6000+uint16(i)), raw[i]; got != want {
			t.Fatalf("octet %d du secteur 1 : RAM=0x%02X, fichier=0x%02X", i, got, want)
		}
	}
}
