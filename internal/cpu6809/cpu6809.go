// Package cpu6809 implémente le microprocesseur Motorola 6809.
// Il ne dépend d'aucune bibliothèque graphique, audio ou fichier.
package cpu6809

// Bus est l'interface que le CPU utilise pour accéder à la mémoire.
// L'implémentation est fournie par internal/core.
type Bus interface {
	Read8(addr uint16) uint8
	Write8(addr uint16, value uint8)
}

// Snapshot capture l'état complet du CPU à un instant donné.
type Snapshot struct {
	PC, X, Y, U, S uint16
	A, B, DP, CC   uint8
}

// CPU représente le Motorola 6809.
type CPU struct {
	bus Bus

	// registres 16 bits
	pc, x, y, u, s uint16

	// registres 8 bits
	a, b, dp, cc uint8
}

// New crée un CPU connecté au bus fourni.
func New(bus Bus) *CPU {
	return &CPU{bus: bus}
}

// Reset initialise le CPU : charge le vecteur de reset et initialise les registres.
func (c *CPU) Reset() {
	hi := c.bus.Read8(0xFFFE)
	lo := c.bus.Read8(0xFFFF)
	c.pc = uint16(hi)<<8 | uint16(lo)
	c.cc = 0x50 // masques IRQ (0x10) et FIRQ (0x40) positionnés au reset
}

// Step exécute une instruction et retourne le nombre de cycles consommés.
// Stub : retourne 0 jusqu'à l'implémentation complète (P2).
func (c *CPU) Step() int {
	return 0
}

// Snapshot retourne une copie de l'état courant du CPU.
func (c *CPU) Snapshot() Snapshot {
	return Snapshot{
		PC: c.pc, X: c.x, Y: c.y, U: c.u, S: c.s,
		A: c.a, B: c.b, DP: c.dp, CC: c.cc,
	}
}
