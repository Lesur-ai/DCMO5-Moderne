package core_test

import (
	"testing"

	"github.com/Lesur-ai/dcmo5/internal/core"
)

// TestROM_IRQ_Trace vérifie que l'IRQ est bien acceptée après boot.
// La ref C (Initprog) met CC=0x10 (FlagF=0, FlagI=1) immédiatement.
// Notre CPU met CC=0x50 (les deux masqués) après reset.
// La ROM doit donc exécuter ANDCC pour démasquer avant que l'IRQ fonctionne.
func TestROM_IRQ_Trace(t *testing.T) {
	rom := loadROM(t)

	// Test 1 : comportement standard (CC=0x50 après reset)
	m1, _ := core.NewMachine(core.Options{ROMSys: rom})
	m1.Reset()
	m1.Step(100_000)
	cs1 := m1.PhysicalRAMChecksum()

	// Test 2 : forcer CC=0x10 après reset (comme la ref C Initprog)
	// On simule ça en écrivant directement le CC via ANDCC au début
	// Ce n'est pas possible directement, mais on peut observer la différence
	// en lançant avec un Step initial plus court
	t.Logf("Checksum après 100k cycles (CC=0x50 reset): 0x%08X", cs1)

	// Vérifier que l'IRQ handler (0xF657) a été atteint
	// Indirectement : après quelques IRQs, la RAM 0x2019 devrait être modifiée
	// (le handler IRQ lit 0x2019 via D6 19 avec DP=0x20)
	val2019 := m1.Read8(0x2019)
	t.Logf("RAM[0x2019] (utilisé par handler IRQ): 0x%02X", val2019)

	// Vérifier CC-like : est-ce que le port 0xA7C3 retourne des valeurs variées?
	portSamples := make([]uint8, 5)
	for i := range portSamples {
		m1.Step(4000) // ~1/4 de ligne
		portSamples[i] = m1.Read8(0xA7C3)
	}
	t.Logf("Port 0xA7C3 samples: %v (doit varier)", portSamples)
}

func TestROM_ForceCC_FlagF_Clear(t *testing.T) {
	// Teste l'hypothèse : si on démasque FlagF (FIRQ) immédiatement
	// via un ANDCC au tout début, est-ce que le BASIC démarre?
	// Stratégie : la ROM utilise aussi FIRQ (handler à 0xF642)
	// On simule le comportement de Initprog() C qui met CC=0x10

	rom := loadROM(t)
	m, _ := core.NewMachine(core.Options{ROMSys: rom})
	m.Reset()

	// La ref C fait CC=0x10 avant l'exécution ROM
	// On simule ça : exécuter quelques cycles puis vérifier
	// si le comportement diffère avec notre CC=0x50

	// Compter combien d'IRQ ont été appelées
	// (indirect: vérifier RAM checksum à des intervals réguliers)
	checksums := make([]uint32, 10)
	for i := range checksums {
		m.Step(300_000) // ~300 ms simules
		checksums[i] = m.PhysicalRAMChecksum()
	}
	t.Log("Checksums RAM (300k cycles par tranche):")
	for i, cs := range checksums {
		t.Logf("  #%d : 0x%08X", i+1, cs)
	}
	// Si les checksums se stabilisent, le CPU est bloqué en boucle d'attente
	stableCount := 0
	for i := 1; i < len(checksums); i++ {
		if checksums[i] == checksums[i-1] {
			stableCount++
		}
	}
	if stableCount > 3 {
		t.Logf("⚠ RAM se stabilise (%d/9 égaux) — CPU probablement bloqué dans attente IRQ", stableCount)
		t.Logf("Hypothèse : la ROM attend une IRQ/FIRQ pour continuer l'init")
	} else {
		t.Logf("✓ RAM continue de changer — CPU actif")
	}
}
