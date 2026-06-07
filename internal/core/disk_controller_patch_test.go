package core_test

// Tests du patch de la ROM contrôleur CD90-640 (alignement trap du DOS, #94).

import (
	"os"
	"testing"

	"github.com/Lesur-ai/dcmo5/internal/core"
	"github.com/Lesur-ai/dcmo5/internal/media"
	"github.com/Lesur-ai/dcmo5/internal/media/impl"
	"github.com/Lesur-ai/dcmo5/internal/spec"
)

// ctrlPatchPoints : (offset depuis 0xA000, octet d'origine, octet patché).
var ctrlPatchPoints = []struct {
	off               int
	original, patched byte
}{
	{0x12E, 0x86, 0x39},
	{0x17D, 0x8D, 0x15}, {0x17E, 0x56, 0x39},
	{0x202, 0x8D, 0x14}, {0x203, 0x3B, 0x39},
	{0x30C, 0x17, 0x39},
	{0x32C, 0x34, 0x18}, {0x32D, 0x7F, 0x39},
}

// ctrlROMWithOriginals fabrique une ROM contrôleur (1984 o) portant les octets
// d'origine aux points de patch (reste à zéro).
func ctrlROMWithOriginals() []byte {
	rom := make([]byte, 0x7C0)
	for _, p := range ctrlPatchPoints {
		rom[p.off] = p.original
	}
	return rom
}

func TestDiskControllerPatch_AppliesAll(t *testing.T) {
	m, err := core.NewMachine(core.Options{DiskControllerROM: ctrlROMWithOriginals(), PatchSystemROM: true})
	if err != nil {
		t.Fatalf("NewMachine: %v", err)
	}
	for _, p := range ctrlPatchPoints {
		addr := uint16(0xA000 + p.off)
		if v := m.Read8(addr); v != p.patched {
			t.Errorf("Read8(%04X) = %02X, want %02X (patché)", addr, v, p.patched)
		}
	}
}

func TestDiskControllerPatch_UnknownROM_NoChange(t *testing.T) {
	rom := ctrlROMWithOriginals()
	rom[0x202] = 0x00 // un point ne correspond ni à l'origine ni au patch
	m, _ := core.NewMachine(core.Options{DiskControllerROM: rom, PatchSystemROM: true})
	// Tout-ou-rien : aucun point modifié.
	for _, p := range ctrlPatchPoints {
		addr := uint16(0xA000 + p.off)
		want := p.original
		if p.off == 0x202 {
			want = 0x00
		}
		if v := m.Read8(addr); v != want {
			t.Errorf("ROM inconnue : Read8(%04X)=%02X modifié à tort (tout-ou-rien)", addr, v)
		}
	}
}

func TestDiskControllerPatch_OptOut(t *testing.T) {
	m, _ := core.NewMachine(core.Options{DiskControllerROM: ctrlROMWithOriginals(), PatchSystemROM: false})
	for _, p := range ctrlPatchPoints {
		addr := uint16(0xA000 + p.off)
		if v := m.Read8(addr); v != p.original {
			t.Errorf("PatchSystemROM=false : Read8(%04X)=%02X modifié (devait rester %02X)", addr, v, p.original)
		}
	}
}

// ── Intégration boot DOS (long) ───────────────────────────────────────────────

type countingDiskCP struct {
	inner media.Disk
	reads int
}

func (d *countingDiskCP) ReadSector(u, t, s int) ([256]byte, error) {
	d.reads++
	return d.inner.ReadSector(u, t, s)
}
func (d *countingDiskCP) WriteSector(u, t, s int, b [256]byte) error {
	return d.inner.WriteSector(u, t, s, b)
}
func (d *countingDiskCP) FormatUnit(u int) error { return d.inner.FormatUnit(u) }

func TestROM_DiskBoot_ReadsWithPatch(t *testing.T) {
	skipIfNotLong(t)
	rom := loadROM(t)
	dcrom, err := os.ReadFile("../../rom/cd90-640.rom")
	if err != nil {
		t.Skipf("ROM contrôleur absente: %v", err)
	}
	open := func() media.Disk {
		d, err := impl.OpenDisk("../../software/dos-5p25-mo5.fd", true)
		if err != nil {
			t.Skipf("disquette DOS absente: %v", err)
		}
		return d
	}

	boot := func(patch bool) int {
		disk := &countingDiskCP{inner: open()}
		m, err := core.NewMachine(core.Options{ROMSys: rom, DiskControllerROM: dcrom, Disk: disk, PatchSystemROM: patch})
		if err != nil {
			t.Fatalf("NewMachine: %v", err)
		}
		m.Reset()
		m.Step(15 * spec.CPUClockHz)
		return disk.reads
	}

	withPatch := boot(true)
	t.Logf("lectures disque AVEC patch contrôleur = %d", withPatch)
	if withPatch == 0 {
		t.Errorf("avec patch : le DOS ne lit pas la disquette (reads=0)")
	}

	// Contrôle : sans patch, le DOS bit-bang le FDC non émulé → aucune lecture.
	if noPatch := boot(false); noPatch != 0 {
		t.Errorf("sans patch : attendu 0 lecture, got %d", noPatch)
	}
}
