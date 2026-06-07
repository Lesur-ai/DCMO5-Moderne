// Fichier : transfer.go — instructions de transfert, chargement et stockage.
package cpu6809

// ── TFR / EXG ────────────────────────────────────────────────────────────────

// execTFR exécute TFR postbyte : copie src → dst.
// Ref: dc6809emul.c Tfr()
func (c *CPU) execTFR(postbyte uint8) {
	src := c.regByID(postbyte >> 4)
	dst := c.regByID(postbyte & 0x0F)
	*dst = *src
}

// execEXG exécute EXG postbyte : échange src ↔ dst.
// Ref: dc6809emul.c Exg()
func (c *CPU) execEXG(postbyte uint8) {
	src := c.regByID(postbyte >> 4)
	dst := c.regByID(postbyte & 0x0F)
	*src, *dst = *dst, *src
}

// regByID retourne un pointeur vers le registre 16 bits identifié par id (4 bits).
// Les registres 8 bits sont promus en uint16 via un proxy dans le registre D ou DA.
// Ref: codes TFR/EXG de dc6809emul.c
//
//	0x0=D 0x1=X 0x2=Y 0x3=U 0x4=S 0x5=PC
//	0x8=A(via D high) 0x9=B(via D low) 0xA=CC(via DA low) 0xB=DP(via DA high)
func (c *CPU) regByID(id uint8) *uint16 {
	switch id & 0x0F {
	case 0x0:
		return (*uint16)(nil) // D — géré via helper dPtr()
	case 0x1:
		return &c.x
	case 0x2:
		return &c.y
	case 0x3:
		return &c.u
	case 0x4:
		return &c.s
	case 0x5:
		return &c.pc
	default:
		return &c.x // invalide → X par défaut
	}
}

// execTFR8 et execEXG8 gèrent les registres 8 bits (A, B, CC, DP).
// Les combinaisons 8-bit↔8-bit et 8-bit↔16-bit utilisent des règles spécifiques.
// On implémente directement le tableau de la ref C.
func (c *CPU) execTFRbyte(postbyte uint8) {
	switch postbyte {
	case 0x01:
		c.x = c.D()
	case 0x02:
		c.y = c.D()
	case 0x03:
		c.u = c.D()
	case 0x04:
		c.s = c.D()
	case 0x05:
		c.pc = c.D()
	case 0x10:
		c.setD(c.x)
	case 0x12:
		c.y = c.x
	case 0x13:
		c.u = c.x
	case 0x14:
		c.s = c.x
	case 0x15:
		c.pc = c.x
	case 0x20:
		c.setD(c.y)
	case 0x21:
		c.x = c.y
	case 0x23:
		c.u = c.y
	case 0x24:
		c.s = c.y
	case 0x25:
		c.pc = c.y
	case 0x30:
		c.setD(c.u)
	case 0x31:
		c.x = c.u
	case 0x32:
		c.y = c.u
	case 0x34:
		c.s = c.u
	case 0x35:
		c.pc = c.u
	case 0x40:
		c.setD(c.s)
	case 0x41:
		c.x = c.s
	case 0x42:
		c.y = c.s
	case 0x43:
		c.u = c.s
	case 0x45:
		c.pc = c.s
	case 0x50:
		c.setD(c.pc)
	case 0x51:
		c.x = c.pc
	case 0x52:
		c.y = c.pc
	case 0x53:
		c.u = c.pc
	case 0x54:
		c.s = c.pc
	case 0x89:
		c.b = c.a
	case 0x8A:
		c.cc = c.a
	case 0x8B:
		c.dp = c.a
	case 0x98:
		c.a = c.b
	case 0x9A:
		c.cc = c.b
	case 0x9B:
		c.dp = c.b
	case 0xA8:
		c.a = c.cc
	case 0xA9:
		c.b = c.cc
	case 0xAB:
		c.dp = c.cc
	case 0xB8:
		c.a = c.dp
	case 0xB9:
		c.b = c.dp
	case 0xBA:
		c.cc = c.dp
	}
}

func (c *CPU) execEXGbyte(postbyte uint8) {
	// EXG est SYMÉTRIQUE : EXG R1,R2 == EXG R2,R1. La table ci-dessous ne liste
	// les paires 16 bits (IDs 0..5) que dans l'ordre canonique (petit nibble en
	// premier). On normalise donc les encodages inversés (ex. EXG Y,X = 0x21 →
	// 0x12) pour les couvrir tous — sans cela, EXG Y,X ne faisait RIEN.
	// Ref: dc6809emul.c Exg() (qui énumère explicitement les deux sens).
	if hi, lo := postbyte>>4, postbyte&0x0F; hi <= 0x05 && lo <= 0x05 && hi > lo {
		postbyte = lo<<4 | hi
	}
	switch postbyte {
	case 0x01:
		d := c.D()
		c.setD(c.x)
		c.x = d
	case 0x02:
		d := c.D()
		c.setD(c.y)
		c.y = d
	case 0x03:
		d := c.D()
		c.setD(c.u)
		c.u = d
	case 0x04:
		d := c.D()
		c.setD(c.s)
		c.s = d
	case 0x05:
		d := c.D()
		c.setD(c.pc)
		c.pc = d
	case 0x12:
		c.x, c.y = c.y, c.x
	case 0x13:
		c.x, c.u = c.u, c.x
	case 0x14:
		c.x, c.s = c.s, c.x
	case 0x15:
		c.x, c.pc = c.pc, c.x
	case 0x23:
		c.y, c.u = c.u, c.y
	case 0x24:
		c.y, c.s = c.s, c.y
	case 0x25:
		c.y, c.pc = c.pc, c.y
	case 0x34:
		c.u, c.s = c.s, c.u
	case 0x35:
		c.u, c.pc = c.pc, c.u
	case 0x45:
		c.s, c.pc = c.pc, c.s
	// 8-bit registers — EXG est symétrique, les deux encodages doivent être gérés.
	case 0x89, 0x98: // A↔B
		c.a, c.b = c.b, c.a
	case 0x8A, 0xA8: // A↔CC
		c.a, c.cc = c.cc, c.a
	case 0x8B, 0xB8: // A↔DP
		c.a, c.dp = c.dp, c.a
	case 0x9A, 0xA9: // B↔CC
		c.b, c.cc = c.cc, c.b
	case 0x9B, 0xB9: // B↔DP
		c.b, c.dp = c.dp, c.b
	case 0xAB, 0xBA: // CC↔DP
		c.cc, c.dp = c.dp, c.cc
	}
}

// ── PSHS / PULS / PSHU / PULU ────────────────────────────────────────────────

// pshs empile les registres sélectionnés sur S. Retourne les cycles extra.
// Ref: dc6809emul.c Pshs()
func (c *CPU) pshs(mask uint8) int {
	n := 0
	if mask&0x80 != 0 {
		c.s--
		c.bus.Write8(c.s, uint8(c.pc))
		c.s--
		c.bus.Write8(c.s, uint8(c.pc>>8))
		n += 2
	}
	if mask&0x40 != 0 {
		c.s--
		c.bus.Write8(c.s, uint8(c.u))
		c.s--
		c.bus.Write8(c.s, uint8(c.u>>8))
		n += 2
	}
	if mask&0x20 != 0 {
		c.s--
		c.bus.Write8(c.s, uint8(c.y))
		c.s--
		c.bus.Write8(c.s, uint8(c.y>>8))
		n += 2
	}
	if mask&0x10 != 0 {
		c.s--
		c.bus.Write8(c.s, uint8(c.x))
		c.s--
		c.bus.Write8(c.s, uint8(c.x>>8))
		n += 2
	}
	if mask&0x08 != 0 {
		c.s--
		c.bus.Write8(c.s, c.dp)
		n++
	}
	if mask&0x04 != 0 {
		c.s--
		c.bus.Write8(c.s, c.b)
		n++
	}
	if mask&0x02 != 0 {
		c.s--
		c.bus.Write8(c.s, c.a)
		n++
	}
	if mask&0x01 != 0 {
		c.s--
		c.bus.Write8(c.s, c.cc)
		n++
	}
	return n
}

// puls dépile les registres sélectionnés depuis S. Retourne les cycles extra.
// Ref: dc6809emul.c Puls()
func (c *CPU) puls(mask uint8) int {
	n := 0
	if mask&0x01 != 0 {
		c.cc = c.bus.Read8(c.s)
		c.s++
		n++
	}
	if mask&0x02 != 0 {
		c.a = c.bus.Read8(c.s)
		c.s++
		n++
	}
	if mask&0x04 != 0 {
		c.b = c.bus.Read8(c.s)
		c.s++
		n++
	}
	if mask&0x08 != 0 {
		c.dp = c.bus.Read8(c.s)
		c.s++
		n++
	}
	if mask&0x10 != 0 {
		hi := c.bus.Read8(c.s)
		c.s++
		lo := c.bus.Read8(c.s)
		c.s++
		c.x = uint16(hi)<<8 | uint16(lo)
		n += 2
	}
	if mask&0x20 != 0 {
		hi := c.bus.Read8(c.s)
		c.s++
		lo := c.bus.Read8(c.s)
		c.s++
		c.y = uint16(hi)<<8 | uint16(lo)
		n += 2
	}
	if mask&0x40 != 0 {
		hi := c.bus.Read8(c.s)
		c.s++
		lo := c.bus.Read8(c.s)
		c.s++
		c.u = uint16(hi)<<8 | uint16(lo)
		n += 2
	}
	if mask&0x80 != 0 {
		hi := c.bus.Read8(c.s)
		c.s++
		lo := c.bus.Read8(c.s)
		c.s++
		c.pc = uint16(hi)<<8 | uint16(lo)
		n += 2
	}
	return n
}

// pshu empile les registres sélectionnés sur U. Retourne les cycles extra.
func (c *CPU) pshu(mask uint8) int {
	n := 0
	if mask&0x80 != 0 {
		c.u--
		c.bus.Write8(c.u, uint8(c.pc))
		c.u--
		c.bus.Write8(c.u, uint8(c.pc>>8))
		n += 2
	}
	if mask&0x40 != 0 {
		c.u--
		c.bus.Write8(c.u, uint8(c.s))
		c.u--
		c.bus.Write8(c.u, uint8(c.s>>8))
		n += 2
	}
	if mask&0x20 != 0 {
		c.u--
		c.bus.Write8(c.u, uint8(c.y))
		c.u--
		c.bus.Write8(c.u, uint8(c.y>>8))
		n += 2
	}
	if mask&0x10 != 0 {
		c.u--
		c.bus.Write8(c.u, uint8(c.x))
		c.u--
		c.bus.Write8(c.u, uint8(c.x>>8))
		n += 2
	}
	if mask&0x08 != 0 {
		c.u--
		c.bus.Write8(c.u, c.dp)
		n++
	}
	if mask&0x04 != 0 {
		c.u--
		c.bus.Write8(c.u, c.b)
		n++
	}
	if mask&0x02 != 0 {
		c.u--
		c.bus.Write8(c.u, c.a)
		n++
	}
	if mask&0x01 != 0 {
		c.u--
		c.bus.Write8(c.u, c.cc)
		n++
	}
	return n
}

// pulu dépile les registres sélectionnés depuis U. Retourne les cycles extra.
func (c *CPU) pulu(mask uint8) int {
	n := 0
	if mask&0x01 != 0 {
		c.cc = c.bus.Read8(c.u)
		c.u++
		n++
	}
	if mask&0x02 != 0 {
		c.a = c.bus.Read8(c.u)
		c.u++
		n++
	}
	if mask&0x04 != 0 {
		c.b = c.bus.Read8(c.u)
		c.u++
		n++
	}
	if mask&0x08 != 0 {
		c.dp = c.bus.Read8(c.u)
		c.u++
		n++
	}
	if mask&0x10 != 0 {
		hi := c.bus.Read8(c.u)
		c.u++
		lo := c.bus.Read8(c.u)
		c.u++
		c.x = uint16(hi)<<8 | uint16(lo)
		n += 2
	}
	if mask&0x20 != 0 {
		hi := c.bus.Read8(c.u)
		c.u++
		lo := c.bus.Read8(c.u)
		c.u++
		c.y = uint16(hi)<<8 | uint16(lo)
		n += 2
	}
	if mask&0x40 != 0 {
		hi := c.bus.Read8(c.u)
		c.u++
		lo := c.bus.Read8(c.u)
		c.u++
		c.s = uint16(hi)<<8 | uint16(lo)
		n += 2
	}
	if mask&0x80 != 0 {
		hi := c.bus.Read8(c.u)
		c.u++
		lo := c.bus.Read8(c.u)
		c.u++
		c.pc = uint16(hi)<<8 | uint16(lo)
		n += 2
	}
	return n
}

// ── LD / ST 8 bits ───────────────────────────────────────────────────────────

// ld8 charge un octet depuis l'adresse addr, positionne N et Z, retourne la valeur.
func (c *CPU) ld8(addr uint16) uint8 {
	v := c.bus.Read8(addr)
	c.setNZ8(v)
	c.setFlag(FlagV, false)
	return v
}

// st8 stocke un octet à l'adresse addr, positionne N et Z.
func (c *CPU) st8(addr uint16, v uint8) {
	c.bus.Write8(addr, v)
	c.setNZ8(v)
	c.setFlag(FlagV, false)
}

// ── LD / ST 16 bits ──────────────────────────────────────────────────────────

// ld16 charge deux octets depuis addr, positionne N et Z, retourne la valeur.
func (c *CPU) ld16(addr uint16) uint16 {
	v := c.read16(addr)
	c.setNZ16(v)
	c.setFlag(FlagV, false)
	return v
}

// st16 stocke deux octets à addr, positionne N et Z.
func (c *CPU) st16(addr uint16, v uint16) {
	c.write16(addr, v)
	c.setNZ16(v)
	c.setFlag(FlagV, false)
}

// ── LEA ──────────────────────────────────────────────────────────────────────

// leaX/Y calcule l'adresse effective et la stocke dans X ou Y (positionne Z).
func (c *CPU) leaX(addr uint16) {
	c.x = addr
	c.setFlag(FlagZ, addr == 0)
}

func (c *CPU) leaY(addr uint16) {
	c.y = addr
	c.setFlag(FlagZ, addr == 0)
}

// leaU/S ne modifient pas Z (spec 6809).
func (c *CPU) leaU(addr uint16) { c.u = addr }
func (c *CPU) leaS(addr uint16) { c.s = addr }
