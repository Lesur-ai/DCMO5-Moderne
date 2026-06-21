package core_test

// rom_integration_test.go — tests d'intégration avec la vraie ROM MO5.
//
// Ces tests sont SKIPPÉS si la ROM n'est pas disponible localement.
// Ils ne sont pas exécutés en CI (aucune ROM copyright dans le repo).
//
// Pour les lancer localement :
//   go test ./internal/core/... -run TestROM -v
//
// La ROM doit être extraite depuis dcmo5v11.0/mo5-rom.zip :
//   unzip dcmo5v11.0/mo5-rom.zip -d /tmp/mo5rom

import (
	"hash/fnv"
	"os"
	"testing"

	"github.com/Lesur-ai/dcmo5/internal/core"
	"github.com/Lesur-ai/dcmo5/internal/spec"
)

// romPath cherche la ROM MO5 dans les emplacements locaux connus.
func romPath() string {
	candidates := []string{
		"../../rom/mo5-v1.1.rom", // ROM système versionnée dans le dépôt
		"/tmp/mo5rom/mo5-v1.1.rom",
		"/tmp/mo5rom/mo5-v1.0.rom",
		"../../dcmo5v11.0/include/mo5rom.h", // header C (non utilisable directement)
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

func loadROM(t *testing.T) []byte {
	t.Helper()
	path := romPath()
	if path == "" {
		t.Skip("ROM MO5 non disponible localement (extraire dcmo5v11.0/mo5-rom.zip dans /tmp/mo5rom/)")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("Impossible de lire la ROM : %v", err)
	}
	if len(data) != 0x4000 {
		t.Skipf("Taille ROM inattendue : %d (attendu 16384)", len(data))
	}
	return data
}

// ── Test de boot ──────────────────────────────────────────────────────────────

func TestROM_Boot_VectorReset(t *testing.T) {
	rom := loadROM(t)
	m, err := core.NewMachine(core.Options{ROMSys: rom})
	if err != nil {
		t.Fatalf("NewMachine: %v", err)
	}
	m.Reset()
	// Après reset, la machine a chargé le vecteur RESET de la ROM (0xFFFE/0xFFFF).
	// On vérifie que Read8(0xFFFE) donne 0xF0 et 0xFFFF donne 0x03 (→ PC=0xF003).
	hi := m.Read8(0xFFFE)
	lo := m.Read8(0xFFFF)
	target := uint16(hi)<<8 | uint16(lo)
	if target != 0xF003 {
		t.Errorf("vecteur RESET = 0x%04X, want 0xF003", target)
	}
	t.Logf("✓ Vecteur RESET = 0x%04X", target)
}

func TestROM_Boot_ExecutesWithoutPanic(t *testing.T) {
	// Vérifie que la machine peut exécuter 100 000 cycles avec la vraie ROM
	// sans paniquer ni boucler infiniment.
	rom := loadROM(t)
	m, err := core.NewMachine(core.Options{ROMSys: rom})
	if err != nil {
		t.Fatalf("NewMachine: %v", err)
	}
	m.Reset()
	consumed := m.Step(100_000)
	if consumed <= 0 {
		t.Errorf("Step(100000) a consommé %d cycles", consumed)
	}
	t.Logf("✓ 100 000 cycles exécutés (consommé: %d)", consumed)
}

// ── Test déterminisme avec vraie ROM ─────────────────────────────────────────

func TestROM_Determinism(t *testing.T) {
	rom := loadROM(t)
	const N = 50_000

	checksum := func() uint32 {
		m, _ := core.NewMachine(core.Options{ROMSys: rom})
		m.Reset()
		m.Step(N)
		return m.PhysicalRAMChecksum()
	}

	c1, c2 := checksum(), checksum()
	if c1 != c2 {
		t.Errorf("non déterministe : 0x%08X != 0x%08X", c1, c2)
	}
	t.Logf("✓ Checksum RAM après %d cycles = 0x%08X (déterministe)", N, c1)
}

// ── Test framebuffer non vide ─────────────────────────────────────────────────

func TestROM_Framebuffer_NonTrivial(t *testing.T) {
	// Après 1 seconde de cycles, la ROM devrait avoir écrit quelque chose
	// dans la RAM vidéo — le framebuffer ne devrait pas être uniformément noir.
	rom := loadROM(t)
	m, err := core.NewMachine(core.Options{ROMSys: rom})
	if err != nil {
		t.Fatalf("NewMachine: %v", err)
	}
	m.Reset()
	m.Step(spec.CPUClockHz) // 1 seconde simulée

	fb := m.Framebuffer()
	// Compter les pixels non-noirs dans la zone active (lignes 8-207)
	nonBlack := 0
	borderColor := fb[0] // couleur de fond
	for y := 8; y < 208; y++ {
		for x := 8; x < 328; x++ {
			if fb[y*core.FrameWidth+x] != borderColor {
				nonBlack++
			}
		}
	}
	t.Logf("Pixels non-noirs après 1s : %d / %d", nonBlack, 320*200)
	if nonBlack == 0 {
		t.Error("framebuffer entièrement uniforme après 1s — la ROM n'a peut-être rien rendu")
	}
}

// ── Test checksum de référence ────────────────────────────────────────────────

func TestROM_Checksum_Reference(t *testing.T) {
	// Calcule et affiche le checksum de référence pour la ROM v1.1.
	// Ce checksum peut servir de golden value pour détecter les régressions futures.
	rom := loadROM(t)
	if romPath() != "/tmp/mo5rom/mo5-v1.1.rom" {
		t.Skip("test uniquement pour mo5-v1.1.rom")
	}

	h := fnv.New32a()
	h.Write(rom)
	romChecksum := h.Sum32()
	t.Logf("Checksum ROM mo5-v1.1 : 0x%08X", romChecksum)

	m, _ := core.NewMachine(core.Options{ROMSys: rom})
	m.Reset()
	m.Step(10_000)
	ramCS := m.PhysicalRAMChecksum()
	t.Logf("Checksum RAM après 10 000 cycles : 0x%08X", ramCS)
	t.Logf("(conserver ces valeurs comme baseline de régression)")
}

func newMachineWithROM(t *testing.T, rom []byte) (*core.Machine, error) {
	t.Helper()
	m, err := core.NewMachine(core.Options{ROMSys: rom})
	if err != nil {
		t.Fatalf("NewMachine: %v", err)
	}
	m.Reset()
	return m, err
}
