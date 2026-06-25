// Fichier : timer.go — timer 6846 et lignes d'IRQ (timer + clavier) du gate-array.
//
// Référence : dcto8demulation.c — Run() (countdown + déclenchement) et
// Timercontrol(). Le timer est décrémenté PAR INSTRUCTION (OnInstructionCycles),
// pas par un tick indépendant : le moteur (internal/engine) appelle cette méthode
// après chaque instruction avec le coût en cycles. Au timeout, le timer recharge
// son latch et arme le signal d'IRQ. Les signaux (timer, clavier) ont une durée
// résiduelle en cycles ; les lignes d'IRQ du moteur sont assertées/relâchées
// selon les drapeaux du CSR (modèle niveau-déclenché du lot #108).
package gatearray

import "github.com/Lesur-ai/dcmo5/internal/machine"

const (
	// Durée du signal d'IRQ timer après un timeout (cycles ; réf C : 100).
	timerIRQPulse = 100
	// Durée du signal d'IRQ clavier après une frappe (cycles ; réf C : 500000 ≈ 500 ms).
	keybIRQPulse = 500000
)

// timerControl reproduit Timercontrol() : recharge le compteur depuis le latch
// quand le bit0 du registre de contrôle e7c5 est armé.
func (g *GateArray) timerControl() {
	if g.port[0x05]&0x01 != 0 {
		g.timer6846 = g.latch6846 << 3
	}
}

// OnInstructionCycles fait avancer le timer 6846 de c cycles et met à jour les
// lignes d'IRQ. Appelé par le moteur après chaque instruction (contrat
// engine.Device). Traduction fidèle de la section timer/IRQ de Run().
func (g *GateArray) OnInstructionCycles(c int, irq *machine.IRQLines) {
	// Décompte du temps de présence du signal d'IRQ timer.
	if g.timerIRQCount > 0 {
		g.timerIRQCount -= c
	}
	if g.timerIRQCount <= 0 {
		g.port[0x00] &^= 0x01 // efface le drapeau d'IRQ timer
	}
	// Décompte du temps de présence du signal d'IRQ clavier (cp1).
	if g.keybIRQCount > 0 {
		g.keybIRQCount -= c
	}
	if g.keybIRQCount <= 0 {
		g.port[0x00] &^= 0x02 // efface le drapeau d'IRQ clavier
	}
	// Drapeau composite : effacé s'il ne reste aucune source active.
	if g.port[0x00]&0x07 == 0 {
		g.port[0x00] &^= 0x80
	}
	// Countdown du timer 6846 si activé (bit0 de e7c5 à 0). Le prescaler (bit2)
	// choisit le décompte direct (c) ou ×8 (c<<3).
	if g.port[0x05]&0x01 == 0 {
		if g.port[0x05]&0x04 != 0 {
			g.timer6846 -= c
		} else {
			g.timer6846 -= c << 3
		}
	}
	// Timeout : recharge le compteur, arme le signal et le drapeau d'IRQ timer.
	if g.timer6846 <= 5 {
		g.timerIRQCount = timerIRQPulse
		g.timer6846 = g.latch6846 << 3
		g.port[0x00] |= 0x81 // drapeau timer + composite
	}
	// Synchronise les lignes d'IRQ du moteur avec les drapeaux du CSR.
	g.syncIRQ(irq)
}

// SuppressFrameIRQ indique au moteur de NE PAS générer d'IRQ de fin de trame 50 Hz
// (modèle MO5). Sur la famille gate-array (TO8D/TO9+), l'interruption périodique est
// fournie par le timer 6846 (OnInstructionCycles) — la réf C dcto8demulation.c Run()
// ne déclenche aucune IRQ de trame. Sans cette suppression, le système recevrait un
// double tick (trame + timer) faussant l'horloge et les cadences.
func (g *GateArray) SuppressFrameIRQ() bool { return true }

// syncIRQ asserte ou relâche les lignes d'IRQ du moteur selon les drapeaux du CSR
// (modèle niveau-déclenché : la ligne reste assertée tant que la source l'est).
func (g *GateArray) syncIRQ(irq *machine.IRQLines) {
	if g.port[0x00]&0x01 != 0 {
		irq.Assert(machine.IRQTimer)
	} else {
		irq.Clear(machine.IRQTimer)
	}
	if g.port[0x00]&0x02 != 0 {
		irq.Assert(machine.IRQKeyboard)
	} else {
		irq.Clear(machine.IRQKeyboard)
	}
}

// TriggerKeyboardIRQ arme le signal d'IRQ clavier (cp1) pour keybIRQPulse cycles
// et positionne le drapeau composite (réf C : TO8key). L'injection des scancodes
// dans la RAM moniteur relève du lot clavier TO8D (#116) ; ce mécanisme d'IRQ est
// posé ici car il appartient au 6846/PIA système.
func (g *GateArray) TriggerKeyboardIRQ() {
	g.port[0x00] |= 0x82 // drapeau clavier (cp1) + composite
	g.keybIRQCount = keybIRQPulse
}
