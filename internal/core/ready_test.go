package core_test

import (
	"testing"

	"github.com/Lesur-ai/dcmo5/internal/core"
)

func TestROM_ReadyPrompt(t *testing.T) {
	rom := loadROM(t)
	m, _ := newMachineWithROM(t, rom)
	// 5 secondes simulées — le BASIC devrait avoir affiché READY
	m.Step(5_000_000)

	saveFramebuffer(t, m, "/tmp/dcmo5_fb_5s.png")
	t.Log("Framebuffer sauvé dans /tmp/dcmo5_fb_5s.png")

	// La ROM MO5 écrit le texte dans la RAM vidéo en partant de la page 1
	// (formes). Le BASIC affiche du texte via des appels ROM
	// qui écrivent directement en RAM vidéo page.
	// Le contenu texte est stocké dans les formes (page 1 = ram[0x2000-0x3FFF])
	// accessible physiquement depuis m.ram[0x2000...]

	// Dump brut des formes vidéo physiques (pas via Read8 qui passe par le bus)
	// On ne peut pas y accéder directement, mais on peut basculer en page 1 et lire
	m.Write8(0xA7C0, 0x01) // page 1
	t.Log("=== Formes vidéo (page 1, adresses 0x0000-0x0277 = 200*40/40 lignes) ===")
	for line := 0; line < 15; line++ {
		row := make([]byte, 40)
		for col := 0; col < 40; col++ {
			row[col] = m.Read8(uint16(line*40 + col))
		}
		// Convertir les codes MO5 en ASCII (MO5 utilise sa propre table)
		// Les codes texte MO5 sont ~= ASCII + 0x80 pour certains
		t.Logf("Formes ligne %2d: %02X", line, row)
	}
	m.Write8(0xA7C0, 0x00) // page 0

	// Compter les octets non-initiaux dans les formes
	m.Write8(0xA7C0, 0x01)
	nonInit := 0
	for addr := uint16(0); addr < 0x07D0; addr++ { // 200 lignes × 40 = 8000
		b := m.Read8(addr)
		if b != 0x00 && b != 0xFF {
			nonInit++
		}
	}
	m.Write8(0xA7C0, 0x00)
	t.Logf("Formes: %d octets non-initiaux sur 8000 — si >0 : texte affiché", nonInit)
}

func TestROM_Framebuffer_30s(t *testing.T) {
	rom := loadROM(t)
	m, _ := newMachineWithROM(t, rom)
	m.Step(30_000_000) // 30 secondes simulées
	saveFramebuffer(t, m, "/tmp/dcmo5_fb_30s.png")
	t.Log("Framebuffer 30s → /tmp/dcmo5_fb_30s.png")

	// Compter couleurs distinctes dans zone active
	fb := m.Framebuffer()
	colors := map[uint32]int{}
	for y := 8; y < 208; y++ {
		for x := 8; x < 328; x++ {
			colors[fb[y*336+x]]++
		}
	}
	t.Logf("Couleurs distinctes après 30s: %d", len(colors))
}

func TestROM_Framebuffer_60s(t *testing.T) {
	rom := loadROM(t)
	m, _ := newMachineWithROM(t, rom)
	m.Step(60_000_000)
	saveFramebuffer(t, m, "/tmp/dcmo5_fb_60s.png")
	fb := m.Framebuffer()
	colors := map[uint32]int{}
	for y := 8; y < 208; y++ {
		for x := 8; x < 328; x++ {
			colors[fb[y*336+x]]++
		}
	}
	t.Logf("Couleurs distinctes après 60s: %d", len(colors))
}

func TestROM_Framebuffer_WithKeypress(t *testing.T) {
	// Après 30s (demo couleurs), appuyer sur ESPACE → READY devrait s'afficher
	rom := loadROM(t)
	m, _ := newMachineWithROM(t, rom)

	// Attendre la fin de l'init (30s = demo couleurs)
	m.Step(30_000_000)
	saveFramebuffer(t, m, "/tmp/dcmo5_fb_before_key.png")
	t.Log("Avant touche: /tmp/dcmo5_fb_before_key.png")

	// Appuyer sur ESPACE (indice 0x20 dans la table MO5)
	m.SetKey(core.Key(0x20), true) // ESPACE = index 0x20 dans mo5key
	m.Step(200_000)                // maintenir ~200ms
	m.SetKey(core.Key(0x20), false)
	m.Step(1_000_000) // laisser le BASIC répondre

	saveFramebuffer(t, m, "/tmp/dcmo5_fb_after_key.png")
	t.Log("Après touche ESPACE: /tmp/dcmo5_fb_after_key.png")

	fb := m.Framebuffer()
	colors := map[uint32]int{}
	for y := 8; y < 208; y++ {
		for x := 8; x < 328; x++ {
			colors[fb[y*336+x]]++
		}
	}
	t.Logf("Couleurs distinctes après touche: %d", len(colors))
}

func TestROM_Framebuffer_120s(t *testing.T) {
	rom := loadROM(t)
	m, _ := newMachineWithROM(t, rom)
	m.Step(120_000_000)
	saveFramebuffer(t, m, "/tmp/dcmo5_fb_120s.png")
	t.Log("120s: /tmp/dcmo5_fb_120s.png")
}
