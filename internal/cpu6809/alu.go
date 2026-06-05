// Fichier : alu.go — opérations arithmétiques et logiques du 6809.
package cpu6809

// ── ADD / SUB 8 bits ─────────────────────────────────────────────────────────

func (c *CPU) add8(a, b uint8, withCarry bool) uint8 {
	ci := uint8(0)
	if withCarry && c.cc&FlagC != 0 {
		ci = 1
	}
	res := uint16(a) + uint16(b) + uint16(ci)
	result := uint8(res)
	c.setFlag(FlagH, (a&0x0F+b&0x0F+ci) > 0x0F)
	c.setFlag(FlagN, result&0x80 != 0)
	c.setFlag(FlagZ, result == 0)
	c.setFlag(FlagV, (a&0x80 == b&0x80) && (result&0x80 != a&0x80))
	c.setFlag(FlagC, res > 0xFF)
	return result
}

func (c *CPU) adc8(a, b uint8) uint8 { return c.add8(a, b, true) }

func (c *CPU) sub8(a, b uint8, withBorrow bool) uint8 {
	bi := uint8(0)
	if withBorrow && c.cc&FlagC != 0 {
		bi = 1
	}
	res := uint16(a) - uint16(b) - uint16(bi)
	result := uint8(res)
	c.setFlag(FlagN, result&0x80 != 0)
	c.setFlag(FlagZ, result == 0)
	c.setFlag(FlagV, (a&0x80 != b&0x80) && (result&0x80 != a&0x80))
	c.setFlag(FlagC, a < b+bi)
	return result
}

func (c *CPU) sbc8(a, b uint8) uint8 { return c.sub8(a, b, true) }

// ── CMP 8 / 16 bits ──────────────────────────────────────────────────────────

func (c *CPU) cmp8(a, b uint8) {
	c.sub8(a, b, false)
}

func (c *CPU) cmp16(a, b uint16) {
	res := uint32(a) - uint32(b)
	result := uint16(res)
	c.setFlag(FlagN, result&0x8000 != 0)
	c.setFlag(FlagZ, result == 0)
	c.setFlag(FlagV, (a&0x8000 != b&0x8000) && (result&0x8000 != a&0x8000))
	c.setFlag(FlagC, a < b)
}

// ── AND / OR / EOR / BIT ─────────────────────────────────────────────────────

func (c *CPU) and8(a, b uint8) uint8 {
	result := a & b
	c.setNZ8(result)
	c.setFlag(FlagV, false)
	return result
}

func (c *CPU) or8(a, b uint8) uint8 {
	result := a | b
	c.setNZ8(result)
	c.setFlag(FlagV, false)
	return result
}

func (c *CPU) eor8(a, b uint8) uint8 {
	result := a ^ b
	c.setNZ8(result)
	c.setFlag(FlagV, false)
	return result
}

func (c *CPU) bit8(a, b uint8) {
	c.and8(a, b)
}

// ── INC / DEC / NEG / COM / CLR / TST ────────────────────────────────────────

func (c *CPU) inc8(v uint8) uint8 {
	c.setFlag(FlagV, v == 0x7F)
	v++
	c.setNZ8(v)
	return v
}

func (c *CPU) dec8(v uint8) uint8 {
	c.setFlag(FlagV, v == 0x80)
	v--
	c.setNZ8(v)
	return v
}

func (c *CPU) neg8(v uint8) uint8 {
	c.setFlag(FlagV, v == 0x80)
	result := uint8(-int8(v))
	c.setNZ8(result)
	c.setFlag(FlagC, result != 0)
	return result
}

func (c *CPU) com8(v uint8) uint8 {
	result := ^v
	c.setNZ8(result)
	c.setFlag(FlagV, false)
	c.setFlag(FlagC, true)
	return result
}

func (c *CPU) clr8() uint8 {
	c.cc = (c.cc & 0xF0) | FlagZ
	return 0
}

func (c *CPU) tst8(v uint8) {
	c.setNZ8(v)
	c.setFlag(FlagV, false)
}

// ── Décalages et rotations ────────────────────────────────────────────────────

func (c *CPU) lsr8(v uint8) uint8 {
	c.setFlag(FlagC, v&0x01 != 0)
	result := v >> 1
	c.setFlag(FlagN, false)
	c.setFlag(FlagZ, result == 0)
	return result
}

func (c *CPU) asr8(v uint8) uint8 {
	c.setFlag(FlagC, v&0x01 != 0)
	result := uint8(int8(v) >> 1)
	c.setNZ8(result)
	return result
}

func (c *CPU) asl8(v uint8) uint8 {
	c.setFlag(FlagC, v&0x80 != 0)
	c.setFlag(FlagV, (v&0x80)^((v<<1)&0x80) != 0)
	result := v << 1
	c.setNZ8(result)
	return result
}

func (c *CPU) ror8(v uint8) uint8 {
	old := c.cc & FlagC
	c.setFlag(FlagC, v&0x01 != 0)
	result := (v >> 1) | (old << 7)
	c.setNZ8(result)
	return result
}

func (c *CPU) rol8(v uint8) uint8 {
	old := c.cc & FlagC
	c.setFlag(FlagC, v&0x80 != 0)
	c.setFlag(FlagV, (v&0x80)^((v<<1|old)&0x80) != 0)
	result := v<<1 | old
	c.setNZ8(result)
	return result
}
