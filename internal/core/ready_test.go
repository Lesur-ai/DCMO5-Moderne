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

	// Capture à 30s : démo couleurs en cours
	m.Step(30_000_000)
	fb30 := m.Framebuffer()
	saveFramebuffer(t, m, "/tmp/dcmo5_fb_30s_snap.png")

	// Avancer jusqu'à 120s
	m.Step(90_000_000)
	fb120 := m.Framebuffer()
	saveFramebuffer(t, m, "/tmp/dcmo5_fb_120s.png")

	// Assertion : le framebuffer doit avoir changé entre 30s et 120s.
	// Si identiques, la démo couleurs ne s'est pas terminée (machine bloquée).
	changed := false
	for i, v := range fb120 {
		if v != fb30[i] {
			changed = true
			break
		}
	}
	if !changed {
		t.Error("framebuffer identique entre 30s et 120s : la démo couleurs ne se termine pas (BASIC READY non atteint)")
	}
	t.Log("Framebuffer 120s → /tmp/dcmo5_fb_120s.png")
}

func TestROM_Long_WithKeypress(t *testing.T) {
	skipIfNotLong(t)
	rom := loadROM(t)
	m, _ := newMachineWithROM(t, rom)
	m.Step(30_000_000)
	fbBefore := m.Framebuffer()
	saveFramebuffer(t, m, "/tmp/dcmo5_fb_before_key.png")

	m.SetKey(core.Key(0x20), true) // ESPACE
	m.Step(200_000)
	m.SetKey(core.Key(0x20), false)
	m.Step(2_000_000)
	fbAfter := m.Framebuffer()
	saveFramebuffer(t, m, "/tmp/dcmo5_fb_after_key.png")

	// Assertion : le framebuffer doit changer après la touche (fin démo, BASIC démarre).
	changed := false
	for i, v := range fbAfter {
		if v != fbBefore[i] {
			changed = true
			break
		}
	}
	if !changed {
		t.Error("framebuffer identique avant/après touche ESPACE : la démo ne répond pas à la touche")
	}
	t.Log("Après ESPACE: /tmp/dcmo5_fb_after_key.png")
}
