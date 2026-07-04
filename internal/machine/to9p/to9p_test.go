package to9p

import (
	"bytes"
	"hash/fnv"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Lesur-ai/dcmo5/internal/keyboard"
	"github.com/Lesur-ai/dcmo5/internal/machine"
)

func romTestPath() string { return filepath.Join("..", "..", "..", "rom", "to9p.rom") }

var testBootDate = time.Date(2026, time.January, 2, 0, 0, 0, 0, time.UTC)

const (
	bootCycles    = 1_200_000
	bootSignature = 0xbe3a0985
)

func mustBoot(t *testing.T) machine.Machine {
	t.Helper()
	blob, err := os.ReadFile(romTestPath())
	if err != nil {
		t.Fatalf("lecture ROM TO9+ : %v", err)
	}
	m, err := newFromROM(blob, testBootDate)
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
	if _, err := newFromROM(make([]byte, 1024), testBootDate); err == nil {
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

func TestPatchTablesWellFormed(t *testing.T) {
	for _, tc := range []struct {
		name    string
		patches []romPatch
	}{
		{"monitor", monitorPatches},
		{"basic", basicPatches},
	} {
		for i, p := range tc.patches {
			if len(p.original) == 0 || len(p.original) != len(p.patched) {
				t.Fatalf("%s patch %d (%s) longueurs invalides: original=%d patched=%d",
					tc.name, i, p.desc, len(p.original), len(p.patched))
			}
		}
	}
}

func TestApplyROMPatchesContract(t *testing.T) {
	romBasic := make([]byte, romBasicSize)
	romMon := make([]byte, romMonSize)
	for _, p := range basicPatches {
		copy(romBasic[p.off:], p.original)
	}
	for _, p := range monitorPatches {
		copy(romMon[p.off:], p.original)
	}

	r1 := applyROMPatches(romMon, romBasic)
	r2 := applyROMPatches(romMon, romBasic)
	if !r1.OK || r1.Applied != len(basicPatches)+len(monitorPatches) || r1.Already != 0 {
		t.Fatalf("1re passe patch = %+v, attendu OK Applied=%d Already=0",
			r1, len(basicPatches)+len(monitorPatches))
	}
	if !r2.OK || r2.Applied != 0 || r2.Already != len(basicPatches)+len(monitorPatches) {
		t.Fatalf("2e passe patch = %+v, attendu OK Applied=0 Already=%d",
			r2, len(basicPatches)+len(monitorPatches))
	}
	for _, p := range basicPatches {
		if got := romBasic[p.off : p.off+len(p.patched)]; !bytes.Equal(got, p.patched) {
			t.Fatalf("basic patch %s non appliqué: got % x want % x", p.desc, got, p.patched)
		}
	}
	for _, p := range monitorPatches {
		if got := romMon[p.off : p.off+len(p.patched)]; !bytes.Equal(got, p.patched) {
			t.Fatalf("monitor patch %s non appliqué: got % x want % x", p.desc, got, p.patched)
		}
	}
	if r := applyROMPatches(make([]byte, romMonSize-1), romBasic); r.OK {
		t.Fatalf("moniteur hors taille accepté : %+v", r)
	}
}

func TestApplyROMPatchesRejectsUnknownVariant(t *testing.T) {
	romBasic := make([]byte, romBasicSize)
	romMon := make([]byte, romMonSize)
	for _, p := range basicPatches {
		copy(romBasic[p.off:], p.original)
	}
	for _, p := range monitorPatches {
		copy(romMon[p.off:], p.original)
	}
	romBasic[basicPatches[0].off] = 0xaa
	if r := applyROMPatches(romMon, romBasic); r.OK {
		t.Fatalf("ROM BASIC inconnue acceptée : %+v", r)
	}
	if got := romMon[monitorPatches[0].off]; got != monitorPatches[0].original[0] {
		t.Fatalf("échec basic a muté le moniteur: got 0x%02x", got)
	}

	romBasic = make([]byte, romBasicSize)
	romMon = make([]byte, romMonSize)
	for _, p := range basicPatches {
		copy(romBasic[p.off:], p.original)
	}
	for _, p := range monitorPatches {
		copy(romMon[p.off:], p.original)
	}
	romMon[monitorPatches[0].off] = 0xaa
	if r := applyROMPatches(romMon, romBasic); r.OK {
		t.Fatalf("ROM moniteur inconnue acceptée : %+v", r)
	}
	if got := romBasic[basicPatches[0].off]; got != basicPatches[0].original[0] {
		t.Fatalf("échec moniteur a muté le BASIC: got 0x%02x", got)
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

func TestApplyROMPatchesMatchesTrackedReference(t *testing.T) {
	blob, err := os.ReadFile(romTestPath())
	if err != nil {
		t.Fatalf("lecture ROM TO9+ : %v", err)
	}
	romBasic, romMon, err := splitROM(blob)
	if err != nil {
		t.Fatalf("split ROM réelle : %v", err)
	}
	rep := applyROMPatches(romMon, romBasic)
	if !rep.OK || rep.Applied != len(basicPatches)+len(monitorPatches) || rep.Already != 0 {
		t.Fatalf("patch ROM réelle = %+v, attendu OK Applied=%d Already=0",
			rep, len(basicPatches)+len(monitorPatches))
	}
	if got := romBasic[0xf2f1 : 0xf2f1+2]; !bytes.Equal(got, []byte{0x45, 0x39}) {
		t.Fatalf("trap écriture cassette BASIC = % x, want 45 39", got)
	}
	if got := romBasic[0xfd1e : 0xfd1e+3]; !bytes.Equal(got, []byte{0x4e, 0x12, 0x20}) {
		t.Fatalf("trap coordonnées souris BASIC = % x, want 4e 12 20", got)
	}
	if got := romMon[0x1aa1 : 0x1aa1+2]; !bytes.Equal(got, []byte{0x4b, 0x39}) {
		t.Fatalf("trap crayon moniteur = % x, want 4b 39", got)
	}
	if got := romMon[0x3193 : 0x3193+2]; !bytes.Equal(got, []byte{0x4e, 0x39}) {
		t.Fatalf("trap souris moniteur = % x, want 4e 39", got)
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
	m, err := newFromROM(blob, testBootDate)
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
	a.ga.Write8(0xe7c3, 0x00) // banque moniteur 0 : offset 0x1a72 visible à 0xfa72.
	if got := a.ga.Read8(0xfa72); got != 0x52 {
		t.Fatalf("trap clic souris moniteur patché = 0x%02x, want 0x52", got)
	}
	if got := a.ga.Read8(0xfa73); got != 0x20 {
		t.Fatalf("suite trap clic souris moniteur patché = 0x%02x, want 0x20", got)
	}
	if got := a.ga.Read8(0xfa74); got != 0x0e {
		t.Fatalf("suite trap clic souris moniteur patché = 0x%02x, want 0x0e", got)
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
