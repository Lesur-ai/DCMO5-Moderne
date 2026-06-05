// Fichier : step.go — boucle d'exécution principale du CPU 6809.
package cpu6809

// Step exécute l'instruction courante et retourne les cycles consommés.
func (c *CPU) Step() int {
	opcode := c.fetchPC8()
	switch opcode {
	// ── Page 0 ──────────────────────────────────────────────────────────────

	// NOP
	case 0x12:
		return 2

	// SYNC
	case 0x13:
		return 4

	// LBRA
	case 0x16:
		off := int16(c.fetchPC16())
		c.pc = uint16(int32(c.pc) + int32(off))
		return 5

	// LBSR
	case 0x17:
		off := int16(c.fetchPC16())
		c.s -= 2
		c.write16(c.s, c.pc)
		c.pc = uint16(int32(c.pc) + int32(off))
		return 9

	// DAA
	case 0x19:
		return c.execDAA()

	// ORCC
	case 0x1A:
		c.cc |= c.fetchPC8()
		return 3

	// ANDCC
	case 0x1C:
		c.cc &= c.fetchPC8()
		return 3

	// SEX
	case 0x1D:
		if c.b&0x80 != 0 {
			c.a = 0xFF
		} else {
			c.a = 0x00
		}
		c.setNZ16(c.D())
		return 2

	// EXG
	case 0x1E:
		pb := c.fetchPC8()
		c.execEXGbyte(pb)
		return 8

	// TFR
	case 0x1F:
		pb := c.fetchPC8()
		c.execTFRbyte(pb)
		return 6

	// BRA
	case 0x20:
		off := int8(c.fetchPC8())
		c.pc = uint16(int32(c.pc) + int32(off))
		return 3

	// BRN
	case 0x21:
		c.pc++
		return 3

	// BHI
	case 0x22:
		return c.branch(c.cc&(FlagZ|FlagC) == 0)
	// BLS
	case 0x23:
		return c.branch(c.cc&(FlagZ|FlagC) != 0)
	// BCC / BHS
	case 0x24:
		return c.branch(c.cc&FlagC == 0)
	// BCS / BLO
	case 0x25:
		return c.branch(c.cc&FlagC != 0)
	// BNE
	case 0x26:
		return c.branch(c.cc&FlagZ == 0)
	// BEQ
	case 0x27:
		return c.branch(c.cc&FlagZ != 0)
	// BVC
	case 0x28:
		return c.branch(c.cc&FlagV == 0)
	// BVS
	case 0x29:
		return c.branch(c.cc&FlagV != 0)
	// BPL
	case 0x2A:
		return c.branch(c.cc&FlagN == 0)
	// BMI
	case 0x2B:
		return c.branch(c.cc&FlagN != 0)
	// BGE
	case 0x2C:
		return c.branch(c.bgeCond())
	// BLT
	case 0x2D:
		return c.branch(!c.bgeCond())
	// BGT
	case 0x2E:
		return c.branch(c.bgtCond())
	// BLE
	case 0x2F:
		return c.branch(!c.bgtCond())

	// LEAX indexed
	case 0x30:
		r := c.AddrIndexed()
		c.leaX(r.Addr)
		return 4 + r.Extra
	// LEAY indexed
	case 0x31:
		r := c.AddrIndexed()
		c.leaY(r.Addr)
		return 4 + r.Extra
	// LEAS indexed
	case 0x32:
		r := c.AddrIndexed()
		c.leaS(r.Addr)
		return 4 + r.Extra
	// LEAU indexed
	case 0x33:
		r := c.AddrIndexed()
		c.leaU(r.Addr)
		return 4 + r.Extra

	// PSHS
	case 0x34:
		mask := c.fetchPC8()
		return 5 + c.pshs(mask)
	// PULS
	case 0x35:
		mask := c.fetchPC8()
		return 5 + c.puls(mask)
	// PSHU
	case 0x36:
		mask := c.fetchPC8()
		return 5 + c.pshu(mask)
	// PULU
	case 0x37:
		mask := c.fetchPC8()
		return 5 + c.pulu(mask)

	// RTS
	case 0x39:
		hi := c.bus.Read8(c.s)
		c.s++
		lo := c.bus.Read8(c.s)
		c.s++
		c.pc = uint16(hi)<<8 | uint16(lo)
		return 5

	// ABX
	case 0x3A:
		c.x += uint16(c.b)
		return 3

	// RTI
	case 0x3B:
		return c.execRTI()

	// CWAI
	case 0x3C:
		c.cc &= c.fetchPC8()
		c.cc |= FlagE
		return 20

	// MUL
	case 0x3D:
		c.execMUL()
		return 11

	// SWI
	case 0x3F:
		return c.execSWI(0xFFFA)

	// ── LD/ST 8 bits — accumulator A ──────────────────────────────────────

	// NEGA
	case 0x40:
		c.a = c.neg8(c.a)
		return 2
	// COMA
	case 0x43:
		c.a = c.com8(c.a)
		return 2
	// LSRA
	case 0x44:
		c.a = c.lsr8(c.a)
		return 2
	// RORA
	case 0x46:
		c.a = c.ror8(c.a)
		return 2
	// ASRA
	case 0x47:
		c.a = c.asr8(c.a)
		return 2
	// ASLA / LSLA
	case 0x48:
		c.a = c.asl8(c.a)
		return 2
	// ROLA
	case 0x49:
		c.a = c.rol8(c.a)
		return 2
	// DECA
	case 0x4A:
		c.a = c.dec8(c.a)
		return 2
	// INCA
	case 0x4C:
		c.a = c.inc8(c.a)
		return 2
	// TSTA
	case 0x4D:
		c.tst8(c.a)
		return 2
	// CLRA
	case 0x4F:
		c.a = c.clr8()
		return 2

	// ── LD/ST 8 bits — accumulator B ──────────────────────────────────────

	case 0x50:
		c.b = c.neg8(c.b)
		return 2
	case 0x53:
		c.b = c.com8(c.b)
		return 2
	case 0x54:
		c.b = c.lsr8(c.b)
		return 2
	case 0x56:
		c.b = c.ror8(c.b)
		return 2
	case 0x57:
		c.b = c.asr8(c.b)
		return 2
	case 0x58:
		c.b = c.asl8(c.b)
		return 2
	case 0x59:
		c.b = c.rol8(c.b)
		return 2
	case 0x5A:
		c.b = c.dec8(c.b)
		return 2
	case 0x5C:
		c.b = c.inc8(c.b)
		return 2
	case 0x5D:
		c.tst8(c.b)
		return 2
	case 0x5F:
		c.b = c.clr8()
		return 2

	// ── Opérations mémoire indexées ───────────────────────────────────────

	case 0x60:
		r := c.AddrIndexed()
		v := c.neg8(c.bus.Read8(r.Addr))
		c.bus.Write8(r.Addr, v)
		return 6 + r.Extra
	case 0x63:
		r := c.AddrIndexed()
		v := c.com8(c.bus.Read8(r.Addr))
		c.bus.Write8(r.Addr, v)
		return 6 + r.Extra
	case 0x64:
		r := c.AddrIndexed()
		v := c.lsr8(c.bus.Read8(r.Addr))
		c.bus.Write8(r.Addr, v)
		return 6 + r.Extra
	case 0x66:
		r := c.AddrIndexed()
		v := c.ror8(c.bus.Read8(r.Addr))
		c.bus.Write8(r.Addr, v)
		return 6 + r.Extra
	case 0x67:
		r := c.AddrIndexed()
		v := c.asr8(c.bus.Read8(r.Addr))
		c.bus.Write8(r.Addr, v)
		return 6 + r.Extra
	case 0x68:
		r := c.AddrIndexed()
		v := c.asl8(c.bus.Read8(r.Addr))
		c.bus.Write8(r.Addr, v)
		return 6 + r.Extra
	case 0x69:
		r := c.AddrIndexed()
		v := c.rol8(c.bus.Read8(r.Addr))
		c.bus.Write8(r.Addr, v)
		return 6 + r.Extra
	case 0x6A:
		r := c.AddrIndexed()
		v := c.dec8(c.bus.Read8(r.Addr))
		c.bus.Write8(r.Addr, v)
		return 6 + r.Extra
	case 0x6C:
		r := c.AddrIndexed()
		v := c.inc8(c.bus.Read8(r.Addr))
		c.bus.Write8(r.Addr, v)
		return 6 + r.Extra
	case 0x6D:
		r := c.AddrIndexed()
		c.tst8(c.bus.Read8(r.Addr))
		return 6 + r.Extra
	case 0x6E:
		r := c.AddrIndexed()
		c.pc = r.Addr
		return 3 + r.Extra
	case 0x6F:
		r := c.AddrIndexed()
		c.bus.Write8(r.Addr, c.clr8())
		return 6 + r.Extra

	// ── Opérations mémoire étendues ───────────────────────────────────────

	case 0x70:
		r := c.AddrExtended()
		v := c.neg8(c.bus.Read8(r.Addr))
		c.bus.Write8(r.Addr, v)
		return 7
	case 0x73:
		r := c.AddrExtended()
		v := c.com8(c.bus.Read8(r.Addr))
		c.bus.Write8(r.Addr, v)
		return 7
	case 0x74:
		r := c.AddrExtended()
		v := c.lsr8(c.bus.Read8(r.Addr))
		c.bus.Write8(r.Addr, v)
		return 7
	case 0x76:
		r := c.AddrExtended()
		v := c.ror8(c.bus.Read8(r.Addr))
		c.bus.Write8(r.Addr, v)
		return 7
	case 0x77:
		r := c.AddrExtended()
		v := c.asr8(c.bus.Read8(r.Addr))
		c.bus.Write8(r.Addr, v)
		return 7
	case 0x78:
		r := c.AddrExtended()
		v := c.asl8(c.bus.Read8(r.Addr))
		c.bus.Write8(r.Addr, v)
		return 7
	case 0x79:
		r := c.AddrExtended()
		v := c.rol8(c.bus.Read8(r.Addr))
		c.bus.Write8(r.Addr, v)
		return 7
	case 0x7A:
		r := c.AddrExtended()
		v := c.dec8(c.bus.Read8(r.Addr))
		c.bus.Write8(r.Addr, v)
		return 7
	case 0x7C:
		r := c.AddrExtended()
		v := c.inc8(c.bus.Read8(r.Addr))
		c.bus.Write8(r.Addr, v)
		return 7
	case 0x7D:
		r := c.AddrExtended()
		c.tst8(c.bus.Read8(r.Addr))
		return 7
	case 0x7E:
		r := c.AddrExtended()
		c.pc = r.Addr
		return 4
	case 0x7F:
		r := c.AddrExtended()
		c.bus.Write8(r.Addr, c.clr8())
		return 7

	// ── LDA / ADDA / SUBA etc. immédiats ─────────────────────────────────

	case 0x80:
		r := c.AddrImmediate(1)
		c.a = c.sub8(c.a, c.bus.Read8(r.Addr), false)
		return 2
	case 0x81:
		r := c.AddrImmediate(1)
		c.cmp8(c.a, c.bus.Read8(r.Addr))
		return 2
	case 0x82:
		r := c.AddrImmediate(1)
		c.a = c.sbc8(c.a, c.bus.Read8(r.Addr))
		return 2
	case 0x83: // SUBD imm
		r := c.AddrImmediate(2)
		c.setD(c.sub16(c.D(), c.read16(r.Addr)))
		return 5
	case 0x84:
		r := c.AddrImmediate(1)
		c.a = c.and8(c.a, c.bus.Read8(r.Addr))
		return 2
	case 0x85:
		r := c.AddrImmediate(1)
		c.bit8(c.a, c.bus.Read8(r.Addr))
		return 2
	case 0x86:
		r := c.AddrImmediate(1)
		c.a = c.ld8(r.Addr)
		return 2
	case 0x88:
		r := c.AddrImmediate(1)
		c.a = c.eor8(c.a, c.bus.Read8(r.Addr))
		return 2
	case 0x89:
		r := c.AddrImmediate(1)
		c.a = c.adc8(c.a, c.bus.Read8(r.Addr))
		return 2
	case 0x8A:
		r := c.AddrImmediate(1)
		c.a = c.or8(c.a, c.bus.Read8(r.Addr))
		return 2
	case 0x8B:
		r := c.AddrImmediate(1)
		c.a = c.add8(c.a, c.bus.Read8(r.Addr), false)
		return 2
	case 0x8C:
		r := c.AddrImmediate(2)
		c.cmp16(c.x, c.read16(r.Addr))
		return 4
	case 0x8D: // BSR
		off := int8(c.fetchPC8())
		retAddr := c.pc
		c.s -= 2
		c.write16(c.s, retAddr)
		c.pc = uint16(int32(c.pc) + int32(off))
		return 7
	case 0x8E:
		r := c.AddrImmediate(2)
		c.x = c.ld16(r.Addr)
		return 3

	// ── LDA / CMPA etc. direct ────────────────────────────────────────────

	case 0x90:
		r := c.AddrDirect()
		c.a = c.sub8(c.a, c.bus.Read8(r.Addr), false)
		return 4
	case 0x91:
		r := c.AddrDirect()
		c.cmp8(c.a, c.bus.Read8(r.Addr))
		return 4
	case 0x92:
		r := c.AddrDirect()
		c.a = c.sbc8(c.a, c.bus.Read8(r.Addr))
		return 4
	case 0x93: // SUBD dir
		r := c.AddrDirect()
		c.setD(c.sub16(c.D(), c.read16(r.Addr)))
		return 6
	case 0x94:
		r := c.AddrDirect()
		c.a = c.and8(c.a, c.bus.Read8(r.Addr))
		return 4
	case 0x95:
		r := c.AddrDirect()
		c.bit8(c.a, c.bus.Read8(r.Addr))
		return 4
	case 0x96:
		r := c.AddrDirect()
		c.a = c.ld8(r.Addr)
		return 4
	case 0x97:
		r := c.AddrDirect()
		c.st8(r.Addr, c.a)
		return 4
	case 0x98:
		r := c.AddrDirect()
		c.a = c.eor8(c.a, c.bus.Read8(r.Addr))
		return 4
	case 0x99:
		r := c.AddrDirect()
		c.a = c.adc8(c.a, c.bus.Read8(r.Addr))
		return 4
	case 0x9A:
		r := c.AddrDirect()
		c.a = c.or8(c.a, c.bus.Read8(r.Addr))
		return 4
	case 0x9B:
		r := c.AddrDirect()
		c.a = c.add8(c.a, c.bus.Read8(r.Addr), false)
		return 4
	case 0x9C:
		r := c.AddrDirect()
		c.cmp16(c.x, c.read16(r.Addr))
		return 6
	case 0x9D:
		r := c.AddrDirect()
		retAddr := c.pc
		c.s -= 2
		c.write16(c.s, retAddr)
		c.pc = r.Addr
		return 7
	case 0x9E:
		r := c.AddrDirect()
		c.x = c.ld16(r.Addr)
		return 5
	case 0x9F:
		r := c.AddrDirect()
		c.st16(r.Addr, c.x)
		return 5

	// ── LDA / CMPA etc. indexés ───────────────────────────────────────────

	case 0xA0:
		r := c.AddrIndexed()
		c.a = c.sub8(c.a, c.bus.Read8(r.Addr), false)
		return 4 + r.Extra
	case 0xA1:
		r := c.AddrIndexed()
		c.cmp8(c.a, c.bus.Read8(r.Addr))
		return 4 + r.Extra
	case 0xA2:
		r := c.AddrIndexed()
		c.a = c.sbc8(c.a, c.bus.Read8(r.Addr))
		return 4 + r.Extra
	case 0xA3: // SUBD indexed
		r := c.AddrIndexed()
		c.setD(c.sub16(c.D(), c.read16(r.Addr)))
		return 6 + r.Extra
	case 0xA4:
		r := c.AddrIndexed()
		c.a = c.and8(c.a, c.bus.Read8(r.Addr))
		return 4 + r.Extra
	case 0xA5:
		r := c.AddrIndexed()
		c.bit8(c.a, c.bus.Read8(r.Addr))
		return 4 + r.Extra
	case 0xA6:
		r := c.AddrIndexed()
		c.a = c.ld8(r.Addr)
		return 4 + r.Extra
	case 0xA7:
		r := c.AddrIndexed()
		c.st8(r.Addr, c.a)
		return 4 + r.Extra
	case 0xA8:
		r := c.AddrIndexed()
		c.a = c.eor8(c.a, c.bus.Read8(r.Addr))
		return 4 + r.Extra
	case 0xA9:
		r := c.AddrIndexed()
		c.a = c.adc8(c.a, c.bus.Read8(r.Addr))
		return 4 + r.Extra
	case 0xAA:
		r := c.AddrIndexed()
		c.a = c.or8(c.a, c.bus.Read8(r.Addr))
		return 4 + r.Extra
	case 0xAB:
		r := c.AddrIndexed()
		c.a = c.add8(c.a, c.bus.Read8(r.Addr), false)
		return 4 + r.Extra
	case 0xAC:
		r := c.AddrIndexed()
		c.cmp16(c.x, c.read16(r.Addr))
		return 6 + r.Extra
	case 0xAD:
		r := c.AddrIndexed()
		retAddr := c.pc
		c.s -= 2
		c.write16(c.s, retAddr)
		c.pc = r.Addr
		return 7 + r.Extra
	case 0xAE:
		r := c.AddrIndexed()
		c.x = c.ld16(r.Addr)
		return 5 + r.Extra
	case 0xAF:
		r := c.AddrIndexed()
		c.st16(r.Addr, c.x)
		return 5 + r.Extra

	// ── LDA / CMPA etc. étendus ───────────────────────────────────────────

	case 0xB0:
		r := c.AddrExtended()
		c.a = c.sub8(c.a, c.bus.Read8(r.Addr), false)
		return 5
	case 0xB1:
		r := c.AddrExtended()
		c.cmp8(c.a, c.bus.Read8(r.Addr))
		return 5
	case 0xB2:
		r := c.AddrExtended()
		c.a = c.sbc8(c.a, c.bus.Read8(r.Addr))
		return 5
	case 0xB3: // SUBD ext
		r := c.AddrExtended()
		c.setD(c.sub16(c.D(), c.read16(r.Addr)))
		return 7
	case 0xB4:
		r := c.AddrExtended()
		c.a = c.and8(c.a, c.bus.Read8(r.Addr))
		return 5
	case 0xB5:
		r := c.AddrExtended()
		c.bit8(c.a, c.bus.Read8(r.Addr))
		return 5
	case 0xB6:
		r := c.AddrExtended()
		c.a = c.ld8(r.Addr)
		return 5
	case 0xB7:
		r := c.AddrExtended()
		c.st8(r.Addr, c.a)
		return 5
	case 0xB8:
		r := c.AddrExtended()
		c.a = c.eor8(c.a, c.bus.Read8(r.Addr))
		return 5
	case 0xB9:
		r := c.AddrExtended()
		c.a = c.adc8(c.a, c.bus.Read8(r.Addr))
		return 5
	case 0xBA:
		r := c.AddrExtended()
		c.a = c.or8(c.a, c.bus.Read8(r.Addr))
		return 5
	case 0xBB:
		r := c.AddrExtended()
		c.a = c.add8(c.a, c.bus.Read8(r.Addr), false)
		return 5
	case 0xBC:
		r := c.AddrExtended()
		c.cmp16(c.x, c.read16(r.Addr))
		return 7
	case 0xBD:
		r := c.AddrExtended()
		retAddr := c.pc
		c.s -= 2
		c.write16(c.s, retAddr)
		c.pc = r.Addr
		return 8
	case 0xBE:
		r := c.AddrExtended()
		c.x = c.ld16(r.Addr)
		return 6
	case 0xBF:
		r := c.AddrExtended()
		c.st16(r.Addr, c.x)
		return 6

	// ── LDB / CMPB etc. ──────────────────────────────────────────────────

	case 0xC0:
		r := c.AddrImmediate(1)
		c.b = c.sub8(c.b, c.bus.Read8(r.Addr), false)
		return 2
	case 0xC1:
		r := c.AddrImmediate(1)
		c.cmp8(c.b, c.bus.Read8(r.Addr))
		return 2
	case 0xC2:
		r := c.AddrImmediate(1)
		c.b = c.sbc8(c.b, c.bus.Read8(r.Addr))
		return 2
	case 0xC3: // ADDD imm
		r := c.AddrImmediate(2)
		c.setD(c.add16(c.D(), c.read16(r.Addr)))
		return 6
	case 0xC4:
		r := c.AddrImmediate(1)
		c.b = c.and8(c.b, c.bus.Read8(r.Addr))
		return 2
	case 0xC5:
		r := c.AddrImmediate(1)
		c.bit8(c.b, c.bus.Read8(r.Addr))
		return 2
	case 0xC6:
		r := c.AddrImmediate(1)
		c.b = c.ld8(r.Addr)
		return 2
	case 0xC8:
		r := c.AddrImmediate(1)
		c.b = c.eor8(c.b, c.bus.Read8(r.Addr))
		return 2
	case 0xC9:
		r := c.AddrImmediate(1)
		c.b = c.adc8(c.b, c.bus.Read8(r.Addr))
		return 2
	case 0xCA:
		r := c.AddrImmediate(1)
		c.b = c.or8(c.b, c.bus.Read8(r.Addr))
		return 2
	case 0xCB:
		r := c.AddrImmediate(1)
		c.b = c.add8(c.b, c.bus.Read8(r.Addr), false)
		return 2
	case 0xCC:
		r := c.AddrImmediate(2)
		c.setD(c.ld16(r.Addr))
		return 3
	case 0xCE:
		r := c.AddrImmediate(2)
		c.u = c.ld16(r.Addr)
		return 3

	// ── LDB / CMPB etc. direct ────────────────────────────────────────────

	case 0xD0:
		r := c.AddrDirect()
		c.b = c.sub8(c.b, c.bus.Read8(r.Addr), false)
		return 4
	case 0xD1:
		r := c.AddrDirect()
		c.cmp8(c.b, c.bus.Read8(r.Addr))
		return 4
	case 0xD2:
		r := c.AddrDirect()
		c.b = c.sbc8(c.b, c.bus.Read8(r.Addr))
		return 4
	case 0xD3: // ADDD dir
		r := c.AddrDirect()
		c.setD(c.add16(c.D(), c.read16(r.Addr)))
		return 6
	case 0xD4:
		r := c.AddrDirect()
		c.b = c.and8(c.b, c.bus.Read8(r.Addr))
		return 4
	case 0xD5:
		r := c.AddrDirect()
		c.bit8(c.b, c.bus.Read8(r.Addr))
		return 4
	case 0xD6:
		r := c.AddrDirect()
		c.b = c.ld8(r.Addr)
		return 4
	case 0xD7:
		r := c.AddrDirect()
		c.st8(r.Addr, c.b)
		return 4
	case 0xD8:
		r := c.AddrDirect()
		c.b = c.eor8(c.b, c.bus.Read8(r.Addr))
		return 4
	case 0xD9:
		r := c.AddrDirect()
		c.b = c.adc8(c.b, c.bus.Read8(r.Addr))
		return 4
	case 0xDA:
		r := c.AddrDirect()
		c.b = c.or8(c.b, c.bus.Read8(r.Addr))
		return 4
	case 0xDB:
		r := c.AddrDirect()
		c.b = c.add8(c.b, c.bus.Read8(r.Addr), false)
		return 4
	case 0xDC:
		r := c.AddrDirect()
		c.setD(c.ld16(r.Addr))
		return 5
	case 0xDD:
		r := c.AddrDirect()
		c.st16(r.Addr, c.D())
		return 5
	case 0xDE:
		r := c.AddrDirect()
		c.u = c.ld16(r.Addr)
		return 5
	case 0xDF:
		r := c.AddrDirect()
		c.st16(r.Addr, c.u)
		return 5

	// ── LDB / CMPB etc. indexés ───────────────────────────────────────────

	case 0xE0:
		r := c.AddrIndexed()
		c.b = c.sub8(c.b, c.bus.Read8(r.Addr), false)
		return 4 + r.Extra
	case 0xE1:
		r := c.AddrIndexed()
		c.cmp8(c.b, c.bus.Read8(r.Addr))
		return 4 + r.Extra
	case 0xE2:
		r := c.AddrIndexed()
		c.b = c.sbc8(c.b, c.bus.Read8(r.Addr))
		return 4 + r.Extra
	case 0xE3: // ADDD indexed
		r := c.AddrIndexed()
		c.setD(c.add16(c.D(), c.read16(r.Addr)))
		return 6 + r.Extra
	case 0xE4:
		r := c.AddrIndexed()
		c.b = c.and8(c.b, c.bus.Read8(r.Addr))
		return 4 + r.Extra
	case 0xE5:
		r := c.AddrIndexed()
		c.bit8(c.b, c.bus.Read8(r.Addr))
		return 4 + r.Extra
	case 0xE6:
		r := c.AddrIndexed()
		c.b = c.ld8(r.Addr)
		return 4 + r.Extra
	case 0xE7:
		r := c.AddrIndexed()
		c.st8(r.Addr, c.b)
		return 4 + r.Extra
	case 0xE8:
		r := c.AddrIndexed()
		c.b = c.eor8(c.b, c.bus.Read8(r.Addr))
		return 4 + r.Extra
	case 0xE9:
		r := c.AddrIndexed()
		c.b = c.adc8(c.b, c.bus.Read8(r.Addr))
		return 4 + r.Extra
	case 0xEA:
		r := c.AddrIndexed()
		c.b = c.or8(c.b, c.bus.Read8(r.Addr))
		return 4 + r.Extra
	case 0xEB:
		r := c.AddrIndexed()
		c.b = c.add8(c.b, c.bus.Read8(r.Addr), false)
		return 4 + r.Extra
	case 0xEC:
		r := c.AddrIndexed()
		c.setD(c.ld16(r.Addr))
		return 5 + r.Extra
	case 0xED:
		r := c.AddrIndexed()
		c.st16(r.Addr, c.D())
		return 5 + r.Extra
	case 0xEE:
		r := c.AddrIndexed()
		c.u = c.ld16(r.Addr)
		return 5 + r.Extra
	case 0xEF:
		r := c.AddrIndexed()
		c.st16(r.Addr, c.u)
		return 5 + r.Extra

	// ── LDB / CMPB etc. étendus ───────────────────────────────────────────

	case 0xF0:
		r := c.AddrExtended()
		c.b = c.sub8(c.b, c.bus.Read8(r.Addr), false)
		return 5
	case 0xF1:
		r := c.AddrExtended()
		c.cmp8(c.b, c.bus.Read8(r.Addr))
		return 5
	case 0xF2:
		r := c.AddrExtended()
		c.b = c.sbc8(c.b, c.bus.Read8(r.Addr))
		return 5
	case 0xF3: // ADDD ext
		r := c.AddrExtended()
		c.setD(c.add16(c.D(), c.read16(r.Addr)))
		return 7
	case 0xF4:
		r := c.AddrExtended()
		c.b = c.and8(c.b, c.bus.Read8(r.Addr))
		return 5
	case 0xF5:
		r := c.AddrExtended()
		c.bit8(c.b, c.bus.Read8(r.Addr))
		return 5
	case 0xF6:
		r := c.AddrExtended()
		c.b = c.ld8(r.Addr)
		return 5
	case 0xF7:
		r := c.AddrExtended()
		c.st8(r.Addr, c.b)
		return 5
	case 0xF8:
		r := c.AddrExtended()
		c.b = c.eor8(c.b, c.bus.Read8(r.Addr))
		return 5
	case 0xF9:
		r := c.AddrExtended()
		c.b = c.adc8(c.b, c.bus.Read8(r.Addr))
		return 5
	case 0xFA:
		r := c.AddrExtended()
		c.b = c.or8(c.b, c.bus.Read8(r.Addr))
		return 5
	case 0xFB:
		r := c.AddrExtended()
		c.b = c.add8(c.b, c.bus.Read8(r.Addr), false)
		return 5
	case 0xFC:
		r := c.AddrExtended()
		c.setD(c.ld16(r.Addr))
		return 6
	case 0xFD:
		r := c.AddrExtended()
		c.st16(r.Addr, c.D())
		return 6
	case 0xFE:
		r := c.AddrExtended()
		c.u = c.ld16(r.Addr)
		return 6
	case 0xFF:
		r := c.AddrExtended()
		c.st16(r.Addr, c.u)
		return 6

	// ── Pages 1 et 2 (préfixes 0x10, 0x11) ────────────────────────────────

	case 0x10:
		return c.stepPage1()
	case 0x11:
		return c.stepPage2()

	default:
		// Opcode illégal : retourner -opcode (convention dcmo5emulation.c).
		// Permet à Machine.Step() de dispatcher l'I/O via entreesortie(-cycles).
		return -int(opcode)
	}
}

// ── Helpers conditions de branchement ────────────────────────────────────────

func (c *CPU) branch(taken bool) int {
	off := int8(c.fetchPC8())
	if taken {
		c.pc = uint16(int32(c.pc) + int32(off))
		return 3
	}
	return 3
}

func (c *CPU) lbranch(taken bool) int {
	off := int16(c.fetchPC16())
	if taken {
		c.pc = uint16(int32(c.pc) + int32(off))
		return 6
	}
	return 5
}

func (c *CPU) bgeCond() bool {
	n := c.cc&FlagN != 0
	v := c.cc&FlagV != 0
	return n == v
}

func (c *CPU) bgtCond() bool {
	z := c.cc&FlagZ != 0
	return !z && c.bgeCond()
}

// ── Page 1 (préfixe 0x10) ────────────────────────────────────────────────────

func (c *CPU) stepPage1() int {
	op := c.fetchPC8()
	switch op {
	case 0x21: // LBRN
		return c.lbranch(false)
	case 0x22:
		return c.lbranch(c.cc&(FlagZ|FlagC) == 0)
	case 0x23:
		return c.lbranch(c.cc&(FlagZ|FlagC) != 0)
	case 0x24:
		return c.lbranch(c.cc&FlagC == 0)
	case 0x25:
		return c.lbranch(c.cc&FlagC != 0)
	case 0x26:
		return c.lbranch(c.cc&FlagZ == 0)
	case 0x27:
		return c.lbranch(c.cc&FlagZ != 0)
	case 0x28:
		return c.lbranch(c.cc&FlagV == 0)
	case 0x29:
		return c.lbranch(c.cc&FlagV != 0)
	case 0x2A:
		return c.lbranch(c.cc&FlagN == 0)
	case 0x2B:
		return c.lbranch(c.cc&FlagN != 0)
	case 0x2C:
		return c.lbranch(c.bgeCond())
	case 0x2D:
		return c.lbranch(!c.bgeCond())
	case 0x2E:
		return c.lbranch(c.bgtCond())
	case 0x2F:
		return c.lbranch(!c.bgtCond())
	case 0x3F:
		return c.execSWI(0xFFF4)
	// CMPD, CMPY, LDY, STY, LDS, STS
	case 0x83:
		r := c.AddrImmediate(2)
		c.cmp16(c.D(), c.read16(r.Addr))
		return 5
	case 0x8C:
		r := c.AddrImmediate(2)
		c.cmp16(c.y, c.read16(r.Addr))
		return 5
	case 0x8E:
		r := c.AddrImmediate(2)
		c.y = c.ld16(r.Addr)
		return 4
	case 0xCE:
		r := c.AddrImmediate(2)
		c.s = c.ld16(r.Addr)
		return 4
	case 0x93:
		r := c.AddrDirect()
		c.cmp16(c.D(), c.read16(r.Addr))
		return 7
	case 0x9C:
		r := c.AddrDirect()
		c.cmp16(c.y, c.read16(r.Addr))
		return 7
	case 0x9E:
		r := c.AddrDirect()
		c.y = c.ld16(r.Addr)
		return 6
	case 0x9F:
		r := c.AddrDirect()
		c.st16(r.Addr, c.y)
		return 6
	case 0xDE:
		r := c.AddrDirect()
		c.s = c.ld16(r.Addr)
		return 6
	case 0xDF:
		r := c.AddrDirect()
		c.st16(r.Addr, c.s)
		return 6
	case 0xA3:
		r := c.AddrIndexed()
		c.cmp16(c.D(), c.read16(r.Addr))
		return 7 + r.Extra
	case 0xAC:
		r := c.AddrIndexed()
		c.cmp16(c.y, c.read16(r.Addr))
		return 7 + r.Extra
	case 0xAE:
		r := c.AddrIndexed()
		c.y = c.ld16(r.Addr)
		return 6 + r.Extra
	case 0xAF:
		r := c.AddrIndexed()
		c.st16(r.Addr, c.y)
		return 6 + r.Extra
	case 0xEE:
		r := c.AddrIndexed()
		c.s = c.ld16(r.Addr)
		return 6 + r.Extra
	case 0xEF:
		r := c.AddrIndexed()
		c.st16(r.Addr, c.s)
		return 6 + r.Extra
	case 0xB3:
		r := c.AddrExtended()
		c.cmp16(c.D(), c.read16(r.Addr))
		return 8
	case 0xBC:
		r := c.AddrExtended()
		c.cmp16(c.y, c.read16(r.Addr))
		return 8
	case 0xBE:
		r := c.AddrExtended()
		c.y = c.ld16(r.Addr)
		return 7
	case 0xBF:
		r := c.AddrExtended()
		c.st16(r.Addr, c.y)
		return 7
	case 0xFE:
		r := c.AddrExtended()
		c.s = c.ld16(r.Addr)
		return 7
	case 0xFF:
		r := c.AddrExtended()
		c.st16(r.Addr, c.s)
		return 7
	default:
		return -int(op)
	}
}

// ── Page 2 (préfixe 0x11) ────────────────────────────────────────────────────

func (c *CPU) stepPage2() int {
	op := c.fetchPC8()
	switch op {
	case 0x3F:
		return c.execSWI(0xFFF2)
	case 0x83:
		r := c.AddrImmediate(2)
		c.cmp16(c.u, c.read16(r.Addr))
		return 5
	case 0x8C:
		r := c.AddrImmediate(2)
		c.cmp16(c.s, c.read16(r.Addr))
		return 5
	case 0x93:
		r := c.AddrDirect()
		c.cmp16(c.u, c.read16(r.Addr))
		return 7
	case 0x9C:
		r := c.AddrDirect()
		c.cmp16(c.s, c.read16(r.Addr))
		return 7
	case 0xA3:
		r := c.AddrIndexed()
		c.cmp16(c.u, c.read16(r.Addr))
		return 7 + r.Extra
	case 0xAC:
		r := c.AddrIndexed()
		c.cmp16(c.s, c.read16(r.Addr))
		return 7 + r.Extra
	case 0xB3:
		r := c.AddrExtended()
		c.cmp16(c.u, c.read16(r.Addr))
		return 8
	case 0xBC:
		r := c.AddrExtended()
		c.cmp16(c.s, c.read16(r.Addr))
		return 8
	default:
		return -int(op)
	}
}

// ── RTI ──────────────────────────────────────────────────────────────────────

func (c *CPU) execRTI() int {
	c.cc = c.bus.Read8(c.s)
	c.s++
	n := 6
	if c.cc&FlagE != 0 {
		c.a = c.bus.Read8(c.s)
		c.s++
		c.b = c.bus.Read8(c.s)
		c.s++
		c.dp = c.bus.Read8(c.s)
		c.s++
		hi := c.bus.Read8(c.s)
		c.s++
		lo := c.bus.Read8(c.s)
		c.s++
		c.x = uint16(hi)<<8 | uint16(lo)
		hi = c.bus.Read8(c.s)
		c.s++
		lo = c.bus.Read8(c.s)
		c.s++
		c.y = uint16(hi)<<8 | uint16(lo)
		hi = c.bus.Read8(c.s)
		c.s++
		lo = c.bus.Read8(c.s)
		c.s++
		c.u = uint16(hi)<<8 | uint16(lo)
		n = 15
	}
	hi := c.bus.Read8(c.s)
	c.s++
	lo := c.bus.Read8(c.s)
	c.s++
	c.pc = uint16(hi)<<8 | uint16(lo)
	return n
}

// ── SWI ──────────────────────────────────────────────────────────────────────

func (c *CPU) execSWI(vector uint16) int {
	c.cc |= FlagE
	c.pshs(0xFF)
	c.cc |= FlagI | FlagF
	c.pc = c.read16(vector)
	return 19
}

// ── MUL ──────────────────────────────────────────────────────────────────────

func (c *CPU) execMUL() {
	result := uint16(c.a) * uint16(c.b)
	c.setD(result)
	c.setFlag(FlagZ, result == 0)
	c.setFlag(FlagC, result&0x0080 != 0)
}

// ── DAA ──────────────────────────────────────────────────────────────────────

func (c *CPU) execDAA() int {
	a := int(c.a)
	cf := false
	if c.cc&FlagH != 0 || a&0x0F > 9 {
		a += 6
	}
	if c.cc&FlagC != 0 || a > 0x99 {
		a += 0x60
		cf = true
	}
	c.a = uint8(a)
	c.setNZ8(c.a)
	c.setFlag(FlagC, cf)
	return 2
}
