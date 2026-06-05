package core_test

import (
	"fmt"
	"testing"

	"github.com/Lesur-ai/dcmo5/internal/core"
)

func TestROM_Boot_3seconds(t *testing.T) {
	rom := loadROM(t)
	m, _ := core.NewMachine(core.Options{ROMSys: rom})
	m.Reset()

	consumed := m.Step(3_000_000)
	t.Logf("Cycles consommés: %d", consumed)

	// Chercher du texte lisible dans la RAM (indices d'un BASIC actif)
	found := 0
	for addr := uint16(0); addr < 0xA000 && found < 15; addr += 40 {
		line := make([]byte, 40)
		allPrint := true
		hasNonZero := false
		for i := range line {
			b := m.Read8(addr + uint16(i))
			line[i] = b
			if b != 0 {
				hasNonZero = true
			}
			if b != 0 && (b < 0x20 || b > 0x7E) {
				allPrint = false
				break
			}
		}
		if allPrint && hasNonZero {
			t.Logf("  RAM@%04X: %q", addr, fmt.Sprintf("%s", line))
			found++
		}
	}

	// Compter les couleurs dans le framebuffer
	fb := m.Framebuffer()
	colors := map[uint32]int{}
	for y := 8; y < 208; y++ {
		for x := 8; x < 328; x++ {
			colors[fb[y*336+x]]++
		}
	}
	t.Logf("Couleurs distinctes dans zone active: %d", len(colors))
	if len(colors) < 2 {
		t.Error("framebuffer uniformément monochrome après 3s — le BASIC n'a peut-être pas démarré")
	}
}
