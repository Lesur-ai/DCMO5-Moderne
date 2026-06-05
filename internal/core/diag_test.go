package core_test

import (
	"testing"

	"github.com/Lesur-ai/dcmo5/internal/core"
	"github.com/Lesur-ai/dcmo5/internal/cpu6809"
)

// diagMachine expose le CPU pour les diagnostics.
type diagMachine struct {
	*core.Machine
	cpu *cpu6809.CPU
}

func TestROM_Diagnostics(t *testing.T) {
	rom := loadROM(t)
	m, _ := core.NewMachine(core.Options{ROMSys: rom})
	m.Reset()

	// Exécuter par tranches et observer le PC
	type snapshot struct {
		cycles int
		ramCS  uint32
	}
	snapshots := []snapshot{}

	for i := 0; i < 10; i++ {
		m.Step(100_000)
		cs := m.PhysicalRAMChecksum()
		snapshots = append(snapshots, snapshot{(i + 1) * 100_000, cs})
	}

	t.Log("Évolution checksum RAM (doit changer si le CPU tourne) :")
	prev := uint32(0)
	stable := 0
	for _, s := range snapshots {
		changed := "✓ change"
		if s.ramCS == prev {
			changed = "⚠ STABLE"
			stable++
		}
		t.Logf("  @%7d cycles : 0x%08X  %s", s.cycles, s.ramCS, changed)
		prev = s.ramCS
	}
	if stable > 5 {
		t.Errorf("RAM stable sur %d/9 tranches — CPU peut-être bloqué", stable)
	}
}

func TestROM_CC_AfterBoot(t *testing.T) {
	// Vérifier que l'IRQ est bien démasquée après quelques cycles
	// (le ROM doit faire ANDCC #0xEF ou TFR pour effacer FlagI)
	rom := loadROM(t)
	m, _ := core.NewMachine(core.Options{ROMSys: rom})
	m.Reset()

	// Après reset : CC = 0x50 (FlagI + FlagF)
	// Mais la ref C (Initprog) met CC = 0x10 (FlagI seulement)
	// → FlagF (FIRQ) est démasqué immédiatement dans la ref C
	// Notre reset met CC=0x50, la ROM doit le corriger

	// Exécuter quelques instructions
	for i := 0; i < 100; i++ {
		m.Step(100)
		// Lire port 0xA7C3 — si Initn() fonctionne, la valeur doit changer
		_ = m.Read8(0xA7C3)
	}

	// Vérifier que la RAM a changé (preuve que le CPU tourne)
	cs := m.PhysicalRAMChecksum()
	t.Logf("Checksum RAM après 10000 cycles: 0x%08X", cs)

	// Lire CC via un port accessible — pas directement accessible depuis l'extérieur
	// Vérifier indirectement : est-ce que l'IRQ a été acceptée ?
	// Si la RAM a changé, le CPU a tourné
	t.Logf("Port 0xA7C3 (Initn): 0x%02X", m.Read8(0xA7C3))
	t.Logf("Port 0xA7E7 (Initn): 0x%02X", m.Read8(0xA7E7))
	t.Logf("Port 0xA7E6 (Iniln): 0x%02X", m.Read8(0xA7E6))
}
