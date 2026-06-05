// Package cpu6809 implémente le microprocesseur Motorola 6809.
// Il ne dépend d'aucune bibliothèque graphique, audio ou fichier.
package cpu6809

// Bus est l'interface que le CPU utilise pour accéder à la mémoire.
type Bus interface {
	Read8(addr uint16) uint8
	Write8(addr uint16, value uint8)
}

// Flags du registre CC (condition codes) — EFHINZVC
const (
	FlagC uint8 = 0x01 // Carry
	FlagV uint8 = 0x02 // oVerflow
	FlagZ uint8 = 0x04 // Zero
	FlagN uint8 = 0x08 // Negative
	FlagI uint8 = 0x10 // IRQ mask
	FlagH uint8 = 0x20 // Half-carry
	FlagF uint8 = 0x40 // FIRQ mask
	FlagE uint8 = 0x80 // Entire state saved
)

// ResetCC est la valeur de CC après un reset matériel (IRQ + FIRQ masqués).
const ResetCC = FlagI | FlagF

// Snapshot capture l'état complet du CPU à un instant donné.
type Snapshot struct {
	PC, X, Y, U, S uint16
	A, B, DP, CC   uint8
	Cycles         int64 // cycles totaux consommés depuis la création
}

// CPU représente le Motorola 6809.
type CPU struct {
	bus Bus

	pc, x, y, u, s uint16
	a, b, dp, cc   uint8

	cycles int64 // compteur de cycles cumulés
}

// New crée un CPU connecté au bus fourni.
func New(bus Bus) *CPU {
	return &CPU{bus: bus}
}

// ── Accès registres ───────────────────────────────────────────────────────────

// D retourne le registre 16 bits D = A<<8 | B.
func (c *CPU) D() uint16 { return uint16(c.a)<<8 | uint16(c.b) }

// setD écrit A et B depuis un uint16.
func (c *CPU) setD(v uint16) { c.a = uint8(v >> 8); c.b = uint8(v) }

// read16 lit deux octets consécutifs big-endian depuis le bus.
func (c *CPU) read16(addr uint16) uint16 {
	return uint16(c.bus.Read8(addr))<<8 | uint16(c.bus.Read8(addr+1))
}

// write16 écrit deux octets big-endian sur le bus.
func (c *CPU) write16(addr uint16, v uint16) {
	c.bus.Write8(addr, uint8(v>>8))
	c.bus.Write8(addr+1, uint8(v))
}

// ── Gestion des flags ─────────────────────────────────────────────────────────

// setFlag positionne ou efface un flag dans CC.
func (c *CPU) setFlag(f uint8, v bool) {
	if v {
		c.cc |= f
	} else {
		c.cc &^= f
	}
}

// testFlag retourne vrai si le flag f est positionné.
func (c *CPU) testFlag(f uint8) bool { return c.cc&f != 0 }

// setNZ positionne N et Z d'après une valeur 8 bits.
func (c *CPU) setNZ8(v uint8) {
	c.setFlag(FlagN, v&0x80 != 0)
	c.setFlag(FlagZ, v == 0)
}

// setNZ16 positionne N et Z d'après une valeur 16 bits.
func (c *CPU) setNZ16(v uint16) {
	c.setFlag(FlagN, v&0x8000 != 0)
	c.setFlag(FlagZ, v == 0)
}

// ── Modes d'adressage ────────────────────────────────────────────────────────

// fetchPC lit un octet à PC et avance PC.
func (c *CPU) fetchPC8() uint8 {
	v := c.bus.Read8(c.pc)
	c.pc++
	return v
}

// fetchPC16 lit deux octets big-endian à PC et avance PC de 2.
func (c *CPU) fetchPC16() uint16 {
	hi := c.bus.Read8(c.pc)
	lo := c.bus.Read8(c.pc + 1)
	c.pc += 2
	return uint16(hi)<<8 | uint16(lo)
}

// indexedReg retourne un pointeur vers l'un des registres X, Y, U, S
// selon les bits 6:5 du post-byte indexé.
func (c *CPU) indexedReg(postbyte uint8) *uint16 {
	switch postbyte & 0x60 {
	case 0x00:
		return &c.x
	case 0x20:
		return &c.y
	case 0x40:
		return &c.u
	default:
		return &c.s
	}
}

// AddrResult est l'adresse effective calculée par un décodeur de mode.
type AddrResult struct {
	Addr  uint16
	Extra int // cycles supplémentaires dus au mode d'adressage
}

// AddrImmediate retourne l'adresse courante (PC) pour le mode immédiat
// et avance PC de size octets.
func (c *CPU) AddrImmediate(size uint16) AddrResult {
	addr := c.pc
	c.pc += size
	return AddrResult{Addr: addr}
}

// AddrDirect retourne l'adresse effective en mode direct (DP:byte).
func (c *CPU) AddrDirect() AddrResult {
	offset := c.fetchPC8()
	return AddrResult{Addr: uint16(c.dp)<<8 | uint16(offset)}
}

// AddrExtended retourne l'adresse effective en mode étendu (2 octets).
func (c *CPU) AddrExtended() AddrResult {
	return AddrResult{Addr: c.fetchPC16()}
}

// AddrIndexed décode le post-byte et retourne l'adresse effective en mode indexé.
// Ref: dc6809emul.c Mgeti()
func (c *CPU) AddrIndexed() AddrResult {
	postbyte := c.fetchPC8()
	reg := c.indexedReg(postbyte)

	// 5-bit offset signé (bit 7 = 0)
	if postbyte&0x80 == 0 {
		offset := int8(postbyte<<3) >> 3 // signe-extend 5 bits
		return AddrResult{Addr: *reg + uint16(int16(offset))}
	}

	switch postbyte & 0x9F {
	case 0x80: // ,R+
		addr := *reg
		*reg++
		return AddrResult{Addr: addr, Extra: 2}
	case 0x81: // ,R++
		addr := *reg
		*reg += 2
		return AddrResult{Addr: addr, Extra: 3}
	case 0x82: // ,-R
		*reg--
		return AddrResult{Addr: *reg, Extra: 2}
	case 0x83: // ,--R
		*reg -= 2
		return AddrResult{Addr: *reg, Extra: 3}
	case 0x84: // ,R
		return AddrResult{Addr: *reg}
	case 0x85: // B,R
		return AddrResult{Addr: *reg + uint16(int16(int8(c.b))), Extra: 1}
	case 0x86: // A,R
		return AddrResult{Addr: *reg + uint16(int16(int8(c.a))), Extra: 1}
	case 0x88: // char,R (offset 8 bits signé)
		off := int8(c.fetchPC8())
		return AddrResult{Addr: *reg + uint16(int16(off)), Extra: 1}
	case 0x89: // word,R (offset 16 bits)
		off := c.fetchPC16()
		return AddrResult{Addr: *reg + off, Extra: 4}
	case 0x8B: // D,R
		return AddrResult{Addr: *reg + c.D(), Extra: 4}
	case 0x8C: // char,PCR
		off := int8(c.fetchPC8())
		return AddrResult{Addr: c.pc + uint16(int16(off)), Extra: 1}
	case 0x8D: // word,PCR
		off := c.fetchPC16()
		return AddrResult{Addr: c.pc + off, Extra: 5}
	case 0x91: // [,R++]
		*reg += 2
		return AddrResult{Addr: c.read16(*reg - 2), Extra: 6}
	case 0x93: // [,--R]
		*reg -= 2
		return AddrResult{Addr: c.read16(*reg), Extra: 6}
	case 0x94: // [,R]
		return AddrResult{Addr: c.read16(*reg), Extra: 3}
	case 0x95: // [B,R]
		return AddrResult{Addr: c.read16(*reg + uint16(int16(int8(c.b)))), Extra: 4}
	case 0x96: // [A,R]
		return AddrResult{Addr: c.read16(*reg + uint16(int16(int8(c.a)))), Extra: 4}
	case 0x98: // [char,R]
		off := int8(c.fetchPC8())
		return AddrResult{Addr: c.read16(*reg + uint16(int16(off))), Extra: 4}
	case 0x99: // [word,R]
		off := c.fetchPC16()
		return AddrResult{Addr: c.read16(*reg + off), Extra: 7}
	case 0x9B: // [D,R]
		return AddrResult{Addr: c.read16(*reg + c.D()), Extra: 7}
	case 0x9C: // [char,PCR]
		off := int8(c.fetchPC8())
		return AddrResult{Addr: c.read16(c.pc + uint16(int16(off))), Extra: 4}
	case 0x9D: // [word,PCR]
		off := c.fetchPC16()
		return AddrResult{Addr: c.read16(c.pc + off), Extra: 8}
	case 0x9F: // [word] indirect étendu
		addr := c.fetchPC16()
		return AddrResult{Addr: c.read16(addr), Extra: 5}
	default:
		// Invalides : comportement identique à ,R (ref C)
		return AddrResult{Addr: *reg, Extra: 3}
	}
}

// ── Interface publique ────────────────────────────────────────────────────────

// Reset initialise le CPU : charge le vecteur de reset et initialise CC.
func (c *CPU) Reset() {
	hi := c.bus.Read8(0xFFFE)
	lo := c.bus.Read8(0xFFFF)
	c.pc = uint16(hi)<<8 | uint16(lo)
	c.cc = ResetCC
}

// Step exécute une instruction et retourne le nombre de cycles consommés.
// Stub : retourne 0 jusqu'à l'implémentation complète (P2.2+).
func (c *CPU) Step() int {
	return 0
}

// Snapshot retourne une copie de l'état courant du CPU.
func (c *CPU) Snapshot() Snapshot {
	return Snapshot{
		PC: c.pc, X: c.x, Y: c.y, U: c.u, S: c.s,
		A: c.a, B: c.b, DP: c.dp, CC: c.cc,
		Cycles: c.cycles,
	}
}
