// Package core représente la machine MO5 complète.
// Il ne dépend d'aucune bibliothèque graphique, audio ni de chemins fichiers.
package core

import (
	"github.com/Lesur-ai/dcmo5/internal/cpu6809"
	"github.com/Lesur-ai/dcmo5/internal/media"
	"github.com/Lesur-ai/dcmo5/internal/spec"
)

// Key identifie une touche du clavier MO5 (index dans [0, spec.KeyMax)).
type Key int

// JoystickInput décrit l'état instantané des deux manettes.
type JoystickInput struct {
	// Position encodes les axes des deux manettes (4 bits par manette).
	Position uint8
	// Action encodes les boutons d'action.
	Action uint8
}

// Options configure la machine au démarrage.
type Options struct {
	// ROMSys est le contenu de la ROM système (16 Ko). Nil = ROM absente.
	ROMSys []byte
	// Tape est la cassette à monter, ou nil.
	Tape media.Tape
	// Disk est la disquette à monter, ou nil.
	Disk media.Disk
	// Cartridge est la cartouche à monter, ou nil.
	Cartridge media.Cartridge
	// Printer reçoit les octets imprimante, ou nil.
	Printer media.PrinterSink
}

// Machine représente le Thomson MO5 complet.
type Machine struct {
	cpu  *cpu6809.CPU
	opts Options

	ram  [spec.RAMTotalSize]uint8
	rom  [0x4000]uint8 // 16 Ko ROM système
	port [spec.PortSize]uint8
}

// NewMachine crée une machine avec les options fournies.
// Retourne une erreur si les options sont invalides (ex: ROM de mauvaise taille).
func NewMachine(opts Options) (*Machine, error) {
	m := &Machine{opts: opts}
	if len(opts.ROMSys) == 0x4000 {
		copy(m.rom[:], opts.ROMSys)
	}
	m.cpu = cpu6809.New(m)
	return m, nil
}

// Reset réinitialise la machine (CPU + état MO5).
func (m *Machine) Reset() {
	m.cpu.Reset()
}

// Step avance l'émulation d'au plus n cycles et retourne les cycles consommés.
// Stub : retourne 0 jusqu'à l'implémentation complète (P3).
func (m *Machine) Step(cycles int) int {
	return 0
}

// Framebuffer retourne le framebuffer logique courant (336×216 pixels RGBA).
// Stub : retourne un slice de zéros jusqu'à P4.
func (m *Machine) Framebuffer() []uint32 {
	return make([]uint32, spec.FrameWidth*spec.FrameHeight)
}

// SetKey met à jour l'état d'une touche MO5.
func (m *Machine) SetKey(key Key, pressed bool) {}

// SetJoystick met à jour l'état des manettes.
func (m *Machine) SetJoystick(input JoystickInput) {}

// SetPen met à jour la position et l'état du crayon optique.
func (m *Machine) SetPen(x, y int, pressed bool) {}

// Read8 implémente cpu6809.Bus — lecture d'un octet sur le bus MO5.
// Stub : renvoie 0xFF (bus flottant) jusqu'à P3.
func (m *Machine) Read8(addr uint16) uint8 {
	return 0xFF
}

// Write8 implémente cpu6809.Bus — écriture d'un octet sur le bus MO5.
// Stub : sans effet jusqu'à P3.
func (m *Machine) Write8(addr uint16, value uint8) {}
