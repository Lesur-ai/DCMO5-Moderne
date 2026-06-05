// Fichier : interrupt.go — traitement des interruptions hardware du 6809.
package cpu6809

// NMI déclenche une interruption non-masquable.
// Empile tout (E=1), charge le vecteur 0xFFFC.
func (c *CPU) NMI() {
	c.cc |= FlagE
	c.pshs(0xFF)
	c.cc |= FlagI | FlagF
	c.pc = c.read16(0xFFFC)
}

// IRQ déclenche une interruption masquable (si FlagI = 0).
// Empile tout (E=1), charge le vecteur 0xFFF8.
func (c *CPU) IRQ() {
	if c.cc&FlagI != 0 {
		return
	}
	c.cc |= FlagE
	c.pshs(0xFF)
	c.cc |= FlagI
	c.pc = c.read16(0xFFF8)
}

// FIRQ déclenche une interruption rapide masquable (si FlagF = 0).
// N'empile que CC + PC (E=0), charge le vecteur 0xFFF6.
func (c *CPU) FIRQ() {
	if c.cc&FlagF != 0 {
		return
	}
	c.cc &^= FlagE
	c.pshs(0x81) // CC + PC uniquement
	c.cc |= FlagI | FlagF
	c.pc = c.read16(0xFFF6)
}
