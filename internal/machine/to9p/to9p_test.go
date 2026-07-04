package to9p

import (
	"hash/fnv"
	"os"
	"path/filepath"
	"testing"

	"github.com/Lesur-ai/dcmo5/internal/keyboard"
	"github.com/Lesur-ai/dcmo5/internal/machine"
)

func romTestPath() string { return filepath.Join("..", "..", "..", "rom", "to9p.rom") }

const (
	bootCycles    = 1_200_000
	bootSignature = 0xdfa2f5c5
)

func mustBoot(t *testing.T) machine.Machine {
	t.Helper()
	blob, err := os.ReadFile(romTestPath())
	if err != nil {
		t.Fatalf("lecture ROM TO9+ : %v", err)
	}
	m, err := newFromROM(blob)
	if err != nil {
		t.Fatalf("boot TO9+ : %v", err)
	}
	return m
}

func frameAfter(m machine.Machine, cycles int) []uint32 {
	for done := 0; done < cycles; {
		done += m.Step(cycles - done)
	}
	w, h := m.FrameSize()
	fb := make([]uint32, w*h)
	m.FramebufferInto(fb)
	return fb
}

func fnv1a(fb []uint32) uint32 {
	h := fnv.New32a()
	var b [4]byte
	for _, px := range fb {
		b[0] = byte(px)
		b[1] = byte(px >> 8)
		b[2] = byte(px >> 16)
		b[3] = byte(px >> 24)
		h.Write(b[:])
	}
	return h.Sum32()
}

func equalFB(a, b []uint32) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func uniform(fb []uint32) bool {
	for _, p := range fb {
		if p != fb[0] {
			return false
		}
	}
	return true
}

func TestProfileRegistered(t *testing.T) {
	p, ok := machine.ByID("to9p")
	if !ok {
		t.Fatal("profil to9p non enregistré")
	}
	if p.Name != "Thomson TO9+" || p.Family != machine.FamilyTOGateArray {
		t.Fatalf("profil to9p = {Name:%q Family:%d}", p.Name, p.Family)
	}
	var rom *machine.Param
	for i := range p.Params {
		if p.Params[i].Key == machine.KeyROM {
			rom = &p.Params[i]
		}
	}
	if rom == nil || !rom.Required || rom.Kind != machine.ParamFile {
		t.Fatalf("paramètre ROM to9p invalide : %+v", rom)
	}
}

func TestNewFromConfigErrors(t *testing.T) {
	if _, err := newFromConfig(machine.Config{}); err == nil {
		t.Error("ROM absente : erreur attendue")
	}
	if _, err := newFromConfig(machine.Config{machine.KeyROM: "/inexistant.rom"}); err == nil {
		t.Error("ROM introuvable : erreur attendue")
	}
	if _, err := newFromROM(make([]byte, 1024)); err == nil {
		t.Error("taille ROM invalide : erreur attendue")
	}
}

func TestNewFromConfigWithTrackedReference(t *testing.T) {
	m, err := newFromConfig(machine.Config{machine.KeyROM: romTestPath()})
	if err != nil {
		t.Fatalf("newFromConfig avec rom/to9p.rom: %v", err)
	}
	if _, ok := m.(*adapter); !ok {
		t.Fatalf("machine concrète = %T, attendu *adapter", m)
	}
}

func TestSplitROMCopiesAndLayout(t *testing.T) {
	blob := make([]byte, romTotalSize)
	blob[0] = 0x42
	blob[romBasicSize-1] = 0x43
	blob[romBasicSize] = 0x44
	blob[romTotalSize-1] = 0x45

	romBasic, romMon, err := splitROM(blob)
	if err != nil {
		t.Fatalf("splitROM: %v", err)
	}
	if len(romBasic) != romBasicSize || len(romMon) != romMonSize {
		t.Fatalf("tailles split = basic %d mon %d", len(romBasic), len(romMon))
	}
	if romBasic[0] != 0x42 || romBasic[romBasicSize-1] != 0x43 ||
		romMon[0] != 0x44 || romMon[romMonSize-1] != 0x45 {
		t.Fatalf("découpage ROM incorrect : basic[0]=0x%02x basic[end]=0x%02x mon[0]=0x%02x mon[end]=0x%02x",
			romBasic[0], romBasic[romBasicSize-1], romMon[0], romMon[romMonSize-1])
	}
	blob[0], blob[romBasicSize] = 0xaa, 0xbb
	if romBasic[0] == 0xaa || romMon[0] == 0xbb {
		t.Fatal("splitROM aliase le blob appelant au lieu de copier les segments")
	}
}

func TestApplyROMPatchesNoOpContract(t *testing.T) {
	romBasic := make([]byte, romBasicSize)
	romMon := make([]byte, romMonSize)
	romBasic[0], romMon[0] = 0x12, 0x34

	r1 := applyROMPatches(romMon, romBasic)
	r2 := applyROMPatches(romMon, romBasic)
	if !r1.OK || r1.Applied != 0 || r1.Already != 0 {
		t.Fatalf("1re passe patch = %+v, attendu OK no-op", r1)
	}
	if !r2.OK || r2.Applied != 0 || r2.Already != 0 {
		t.Fatalf("2e passe patch = %+v, attendu idempotent no-op", r2)
	}
	if romBasic[0] != 0x12 || romMon[0] != 0x34 {
		t.Fatalf("patch no-op a muté les ROMs : basic[0]=0x%02x mon[0]=0x%02x", romBasic[0], romMon[0])
	}
	if r := applyROMPatches(make([]byte, romMonSize-1), romBasic); r.OK {
		t.Fatalf("moniteur hors taille accepté : %+v", r)
	}
}

func TestSplitROMMatchesTrackedReference(t *testing.T) {
	blob, err := os.ReadFile(romTestPath())
	if err != nil {
		t.Fatalf("lecture ROM TO9+ : %v", err)
	}
	romBasic, romMon, err := splitROM(blob)
	if err != nil {
		t.Fatalf("split ROM réelle : %v", err)
	}
	if string(romBasic[:16]) != " BASIC 512 MICRO" {
		t.Fatalf("signature BASIC TO9+ inattendue : %q", string(romBasic[:16]))
	}
	// Le gate-array mappe la banque système 0 sur 0xE000-0xFFFF :
	// 0xFFFE correspond donc à l'offset 0x1FFE dans le moniteur 16 Ko.
	reset := uint16(romMon[0x1ffe])<<8 | uint16(romMon[0x1fff])
	if reset != 0xfda0 {
		t.Fatalf("vecteur reset = 0x%04x, attendu 0xFDA0 pour rom/to9p.rom", reset)
	}
}

func TestNewFromROMWiresROMIntoGateArray(t *testing.T) {
	blob, err := os.ReadFile(romTestPath())
	if err != nil {
		t.Fatalf("lecture ROM TO9+ : %v", err)
	}
	romBasic, romMon, err := splitROM(blob)
	if err != nil {
		t.Fatalf("split ROM réelle : %v", err)
	}
	m, err := newFromROM(blob)
	if err != nil {
		t.Fatalf("newFromROM: %v", err)
	}
	if w, h := m.FrameSize(); w != 672 || h != 216 {
		t.Fatalf("FrameSize = %dx%d, attendu 672x216", w, h)
	}
	if km := m.KeyboardModel(); km != keyboard.TO9PModel() {
		t.Fatalf("KeyboardModel = %+v, attendu TO9PModel %+v", km, keyboard.TO9PModel())
	}
	a, ok := m.(*adapter)
	if !ok {
		t.Fatalf("machine concrète = %T, attendu *adapter", m)
	}
	if got, want := a.ga.Read8(0xfffe), romMon[0x1ffe]; got != want {
		t.Fatalf("moniteur câblé à 0xFFFE = 0x%02x, attendu romMon[0x1ffe]=0x%02x", got, want)
	}
	a.ga.Write8(0xe7c3, 0x04) // active la ROM interne BASIC sur l'espace 0x0000-0x3FFF.
	if got, want := a.ga.Read8(0x0000), romBasic[0]; got != want {
		t.Fatalf("BASIC câblé à 0x0000 = 0x%02x, attendu romBasic[0]=0x%02x", got, want)
	}

	a.ga.Write8(0xe7c3, 0x10) // sélection banque moniteur 1 pour observer le chemin TO8D.
	before := a.ga.Read8(0xf0f8)
	a.ga.SetKey(0x02, true) // Y : TO9+ publie ASCII, pas scancode moniteur TO8D.
	if got := a.ga.Read8(0xe7de); got != 0x01 {
		t.Fatalf("TO9+ E7DE après frappe = 0x%02x, attendu 0x01", got)
	}
	if got := a.ga.Read8(0xe7df); got != 0x59 {
		t.Fatalf("TO9+ E7DF après frappe Y = 0x%02x, attendu 0x59", got)
	}
	if after := a.ga.Read8(0xf0f8); after != before {
		t.Fatalf("TO9+ a muté le chemin moniteur TO8D : F0F8 avant=0x%02x après=0x%02x", before, after)
	}
}

func TestBootDeterministic(t *testing.T) {
	m1 := mustBoot(t)
	w, h := m1.FrameSize()
	fbReset := make([]uint32, w*h)
	m1.FramebufferInto(fbReset)
	if pc := m1.CPUSnapshot().PC; pc != 0xFDA0 {
		t.Fatalf("PC au reset = 0x%04x, attendu le vecteur reset TO9+ 0xFDA0", pc)
	}

	fbBoot := frameAfter(m1, bootCycles)

	if uniform(fbBoot) {
		t.Fatal("framebuffer uniforme après boot : le firmware TO9+ n'a rien rendu")
	}
	if equalFB(fbReset, fbBoot) {
		t.Fatal("framebuffer inchangé depuis le reset : le boot TO9+ n'a rien dessiné")
	}
	if pc := m1.CPUSnapshot().PC; pc == 0xFDA0 {
		t.Fatal("PC toujours au vecteur reset TO9+ : le CPU n'a pas exécuté")
	}

	fbBoot2 := frameAfter(mustBoot(t), bootCycles)
	if !equalFB(fbBoot, fbBoot2) {
		t.Fatal("boot TO9+ non déterministe : deux instances fraîches divergent")
	}

	got := fnv1a(fbBoot)
	if bootSignature == 0 {
		t.Fatalf("signature de boot TO9+ à figer : bootSignature = 0x%08x", got)
	}
	if got != bootSignature {
		t.Fatalf("signature framebuffer boot TO9+ = 0x%08x, attendu 0x%08x (régression du boot ?)", got, bootSignature)
	}
}
