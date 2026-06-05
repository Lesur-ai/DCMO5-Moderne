package core_test

// ready_test.go — tests d'intégration longs avec la vraie ROM MO5.
//
// Ces tests sont LENTS (30-120s simulées) et SKIPPÉS par défaut.
// Ils nécessitent la ROM ET la variable d'environnement DCMO5_LONG_TESTS=1.
//
//   DCMO5_LONG_TESTS=1 go test ./internal/core/... -run TestROM_Long -v

import (
	"os"
	"testing"

	"github.com/Lesur-ai/dcmo5/internal/core"
)

// skipIfNotLong saute si DCMO5_LONG_TESTS n'est pas défini.
func skipIfNotLong(t *testing.T) {
	t.Helper()
	if os.Getenv("DCMO5_LONG_TESTS") == "" {
		t.Skip("test long — définir DCMO5_LONG_TESTS=1 pour l'activer")
	}
}

func TestROM_Long_Framebuffer_30s(t *testing.T) {
	skipIfNotLong(t)
	rom := loadROM(t)
	m, _ := newMachineWithROM(t, rom)
	m.Step(30_000_000)
	saveFramebuffer(t, m, "/tmp/dcmo5_fb_30s.png")
	fb := m.Framebuffer()
	colors := map[uint32]int{}
	for y := 8; y < 208; y++ {
		for x := 8; x < 328; x++ {
			colors[fb[y*336+x]]++
		}
	}
	t.Logf("Couleurs distinctes après 30s: %d (attendu ≥3 pour écran démo)", len(colors))
	if len(colors) < 3 {
		t.Error("écran démo non atteint après 30s simulées")
	}
}

func TestROM_Long_Framebuffer_120s(t *testing.T) {
	skipIfNotLong(t)
	rom := loadROM(t)
	m, _ := newMachineWithROM(t, rom)
	m.Step(120_000_000)
	saveFramebuffer(t, m, "/tmp/dcmo5_fb_120s.png")
	t.Log("Framebuffer 120s → /tmp/dcmo5_fb_120s.png")
}

func TestROM_Long_WithKeypress(t *testing.T) {
	skipIfNotLong(t)
	rom := loadROM(t)
	m, _ := newMachineWithROM(t, rom)
	m.Step(30_000_000)
	saveFramebuffer(t, m, "/tmp/dcmo5_fb_before_key.png")
	m.SetKey(core.Key(0x20), true) // ESPACE
	m.Step(200_000)
	m.SetKey(core.Key(0x20), false)
	m.Step(1_000_000)
	saveFramebuffer(t, m, "/tmp/dcmo5_fb_after_key.png")
	t.Log("Après ESPACE: /tmp/dcmo5_fb_after_key.png")
}
