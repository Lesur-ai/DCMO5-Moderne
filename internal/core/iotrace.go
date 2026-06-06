// Fichier : iotrace.go — instrumentation optionnelle des traps d'E/S MO5.
//
// But : observer si la ROM atteint réellement les traps d'E/S (disque, cassette,
// crayon, imprimante) et avec quels paramètres, sans modifier le comportement.
// Préalable de diagnostic à la Phase P10 (corrections #84/#85/#86).
//
// La trace est entièrement OPT-IN : tant que EnableIOTrace n'a pas été appelé,
// le hook est nil et le coût est nul. Le gating par variable d'environnement est
// fait au bord (cmd/dcmo5), jamais dans le cœur — qui reste pur et sans dépendance
// au système de fichiers ni à l'environnement.
package core

import (
	"fmt"
	"io"
	"sync"
)

// ioTrace capture les traps d'E/S : compteurs par code + journal détaillé.
type ioTrace struct {
	mu     sync.Mutex
	w      io.Writer
	counts map[int]int
}

// EnableIOTrace active la trace des traps d'E/S vers w. Idempotent : un second
// appel remplace la destination et remet les compteurs à zéro. Passer w == nil
// désactive la trace.
//
// Contrat de concurrence : c'est un appel de contrôle PRÉ-RUN. Il doit être
// fait avant de démarrer la goroutine d'émulation (emu.Host) ou tout appel
// concurrent à Step()/entreesortie(). Une fois la trace active, les compteurs
// (record/IOTraceCounts) sont protégés par un mutex, mais le pointeur lui-même
// ne doit pas être modifié pendant que l'émulation tourne.
func (m *Machine) EnableIOTrace(w io.Writer) {
	if w == nil {
		m.ioTrace = nil
		return
	}
	m.ioTrace = &ioTrace{w: w, counts: make(map[int]int)}
}

// IOTraceCounts retourne une copie des compteurs de traps observés (clé = code
// trap, valeur = nombre d'occurrences). Vide si la trace est désactivée.
// Sert aux assertions observables des tests.
func (m *Machine) IOTraceCounts() map[int]int {
	if m.ioTrace == nil {
		return map[int]int{}
	}
	m.ioTrace.mu.Lock()
	defer m.ioTrace.mu.Unlock()
	out := make(map[int]int, len(m.ioTrace.counts))
	for k, v := range m.ioTrace.counts {
		out[k] = v
	}
	return out
}

// ioTrapName donne un libellé lisible pour un code trap d'E/S.
func ioTrapName(io int) string {
	switch io {
	case 0x14:
		return "READSECTOR"
	case 0x15:
		return "WRITESECTOR"
	case 0x18:
		return "FORMATDISK"
	case 0x41:
		return "READBITK7"
	case 0x42:
		return "READOCTETK7"
	case 0x45:
		return "WRITEOCTETK7"
	case 0x4B:
		return "READPENXY"
	case 0x51:
		return "IMPRIME"
	default:
		return "UNKNOWN"
	}
}

// record incrémente le compteur du trap et journalise une ligne détaillée.
// Appelé depuis entreesortie() avant le dispatch, pour figer l'état d'entrée
// (paramètres en RAM, registres) tel que vu par la ROM.
func (t *ioTrace) record(io int, m *Machine) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.counts[io]++

	// Adresse de l'opcode-trap : CPU.Step() a déjà avancé le PC d'un octet en
	// fetchant l'opcode illégal avant de retourner -code, donc PC-1 pointe sur
	// le trap lui-même (les codes d'E/S MO5 sont des opcodes mono-octet).
	pc := m.cpu.Snapshot().PC - 1
	name := ioTrapName(io)

	switch io {
	case 0x14, 0x15, 0x18: // disque : paramètres en RAM (ref dcmo5devices.c)
		u := m.Read8(0x2049)
		ph := m.Read8(0x204A)
		pl := m.Read8(0x204B)
		sec := m.Read8(0x204C)
		dest := uint16(m.Read8(0x204F))<<8 | uint16(m.Read8(0x2050))
		fmt.Fprintf(t.w, "IOTRACE pc=%04X io=%02X %s u=%d trkHi=%d trk=%d sec=%d dest=%04X\n",
			pc, io, name, u, ph, pl, sec, dest)
	case 0x41, 0x42, 0x45: // cassette : état bit-level + registre A
		fmt.Fprintf(t.w, "IOTRACE pc=%04X io=%02X %s k7bit=%02X k7octet=%02X A=%02X tape=%t\n",
			pc, io, name, m.k7bit, m.k7octet, m.cpu.RegA(), m.opts.Tape != nil)
	case 0x4B: // crayon optique : coordonnées courantes
		fmt.Fprintf(t.w, "IOTRACE pc=%04X io=%02X %s xpen=%d ypen=%d button=%t\n",
			pc, io, name, m.xpen, m.ypen, m.penbutton)
	case 0x51: // imprimante : registre B
		fmt.Fprintf(t.w, "IOTRACE pc=%04X io=%02X %s B=%02X\n", pc, io, name, m.cpu.RegB())
	default:
		fmt.Fprintf(t.w, "IOTRACE pc=%04X io=%02X %s\n", pc, io, name)
	}
}
