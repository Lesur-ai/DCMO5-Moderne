// Package gatearray implémente la carte mémoire et le banking de la famille
// Thomson « gate array » (TO8/TO8D/TO9/TO9+). C'est le socle du Device gate-array
// de la v2 : la vidéo (5 modes + palette EF9369), le timer 6846/PIA, les IRQ et
// les traps d'E/S sont ajoutés par les lots suivants (#113, #114, #115).
//
// Référence : dcto8demulation.c (Daniel Coulom, GPLv3) — Mgetto8d/Mputto8d,
// TO8videoram/TO8rambank/TO8rombank, Hardreset/Initprog. La réf C exprime le
// banking par arithmétique de pointeurs avec décalages négatifs (ex.
// « ramvideo = ram - 0x4000 + (page<<13) »). Go n'a pas cette arithmétique : on
// stocke à la place des OFFSETS entiers (éventuellement négatifs) tels que
// l'accès `segment[base + int(a)]` reproduise exactement le pointeur de la réf.
//
// Carte mémoire (dispatch par page de 4 Ko, a>>12) :
//
//	0x0–0x3  espace ROM/cartouche (rombank) — recouvrable par RAM via e7e6
//	0x4–0x5  RAM vidéo (couleurs/formes, page via e7c3 bit0)
//	0x6–0x9  RAM utilisateur fixe
//	0xA–0xD  banque RAM commutable (e7e5 mode TO8 / e7c9 compat TO7-70)
//	0xE      I/O (e7c0–e7e7) + ROM système (2 banques via e7c3 bit4)
//	0xF      ROM système
package gatearray

// Tailles des espaces mémoire (réf C : ram[0x80000], car[0x10000], port[0x40]).
const (
	ramSize      = 0x80000 // 512 Ko de RAM
	carSize      = 0x10000 // espace cartouche : 4 banques de 16 Ko
	portSize     = 0x40    // ports d'E/S e7c0–e7ff (indexés a&0x3f)
	romMonSize   = 0x4000  // ROM moniteur système : 2 banques de 8 Ko
	romBasicSize = 0x10000 // ROM interne (BASIC) : 4 banques de 16 Ko
)

// romTarget indique dans quel espace pointe la banque ROM courante (rombank).
// La réf C utilise un pointeur unique ; en Go il faut savoir quel tableau indexer.
type romTarget int

const (
	targetCart  romTarget = iota // cartouche externe (car[])
	targetBasic                  // ROM interne BASIC (romBasic[])
	targetRAM                    // recouvrement de l'espace ROM par la RAM (e7e6)
)

// GateArray détient la mémoire et l'état de banking d'une machine gate-array.
type GateArray struct {
	ram      [ramSize]byte
	car      [carSize]byte
	port     [portSize]byte
	romMon   [romMonSize]byte   // moniteur système (romsys)
	romBasic [romBasicSize]byte // ROM interne (rombank en mode ROM interne)

	// Offsets de banque : l'accès `<segment>[base + int(a)]` reproduit le pointeur
	// de la réf C (qui peut être négatif, ex. ramuser = ram - 0x2000).
	ramvideoBase int       // dans ram[] (page couleurs/formes)
	ramuserBase  int       // dans ram[] (RAM utilisateur fixe = -0x2000)
	rambankBase  int       // dans ram[] (banque RAM commutable)
	romsysBase   int       // dans romMon[] (banque système)
	rombankBase  int       // dans la cible rombankTgt
	rombankTgt   romTarget // espace pointé par rombank (car/basic/ram)

	cartype  int // 0=simple 1=switch-bank 2=os-9 (réf C)
	carflags int // bits0,1,4=banque, 2=cart-enabled, 3=write-enabled

	// Numéros de banque courants (parité réf C + observabilité/tests).
	nvideopage int // page vidéo (0–1)
	nsystbank  int // banque système (0–1)
	nrambank   int // banque RAM (0–31)
	nrombank   int // banque ROM (-1 si cartouche)
}

// New construit un gate-array. romMon (≤ 16 Ko, moniteur système) et romBasic
// (≤ 64 Ko, ROM interne) sont copiés dans des tampons de taille fixe (tronqués
// au besoin, complétés de zéros). La machine est mise en état de reset matériel.
func New(romMon, romBasic []byte) *GateArray {
	g := &GateArray{}
	copy(g.romMon[:], romMon)
	copy(g.romBasic[:], romBasic)
	g.hardReset()
	return g
}

// hardReset reproduit Hardreset() : RAM en motif 0x00/0xFF (bit 7 de l'adresse),
// ports à zéro sauf e7c9 (port[0x09]=0x0f), cartouche effacée, puis Initprog.
func (g *GateArray) hardReset() {
	for i := range g.ram {
		if i&0x80 != 0 {
			g.ram[i] = 0xFF
		} else {
			g.ram[i] = 0x00
		}
	}
	for i := range g.port {
		g.port[i] = 0
	}
	g.port[0x09] = 0x0f
	for i := range g.car {
		g.car[i] = 0
	}
	g.nvideopage = 0
	g.nrambank = 0
	g.nsystbank = 0
	g.initprog()
}

// initprog reproduit Initprog() (partie mémoire) : recalcule tous les pointeurs
// de banque depuis l'état des ports. ramuser est fixe (ram - 0x2000).
func (g *GateArray) initprog() {
	g.carflags &= 0xec
	g.ramuserBase = -0x2000
	g.updateVideoRAM()
	g.updateRAMBank()
	g.updateROMBank()
}

// Reset relance la machine dans l'état de reset matériel (efface la RAM).
func (g *GateArray) Reset() { g.hardReset() }

// LoadCartridge copie une cartouche (≤ 64 Ko) dans l'espace car[] et fixe le type
// (simple ≤ 16 Ko / commutation de banque au-delà). Le routage ROM interne ↔
// cartouche est piloté par e7c3 bit2 ; au reset (bit2=0) la cartouche est active.
// Le câblage média complet (montage à chaud, OS-9…) relève d'un lot ultérieur.
func (g *GateArray) LoadCartridge(data []byte) {
	for i := range g.car {
		g.car[i] = 0
	}
	n := copy(g.car[:], data)
	g.cartype = 0
	if n > 0x4000 {
		g.cartype = 1
	}
	g.updateROMBank()
}

// ── Sélection de banques (réf C : TO8videoram / TO8rambank / TO8rombank) ──────

// updateVideoRAM positionne la page vidéo (couleurs/formes) et la banque ROM
// système selon e7c3 (port[0x03]). Réf C : TO8videoram().
func (g *GateArray) updateVideoRAM() {
	g.nvideopage = int(g.port[0x03]) & 1
	g.ramvideoBase = -0x4000 + (g.nvideopage << 13)
	g.nsystbank = (int(g.port[0x03]) & 0x10) >> 4
	g.romsysBase = -0xe000 + (g.nsystbank << 13)
}

// updateRAMBank positionne la banque RAM commutable. Deux modes (réf C :
// TO8rambank()) : mode TO8 piloté par e7e5 (port[0x25], 32 banques) quand
// e7e7 bit4 est armé ; sinon compatibilité TO7/70 via e7c9 (port[0x09]).
func (g *GateArray) updateRAMBank() {
	if g.port[0x27]&0x10 != 0 {
		g.nrambank = int(g.port[0x25]) & 0x1f
		g.rambankBase = -0xa000 + (g.nrambank << 14)
		return
	}
	switch g.port[0x09] & 0xf8 {
	case 0x08:
		g.nrambank = 0
	case 0x10:
		g.nrambank = 1
	case 0xe0:
		g.nrambank = 2
	case 0xa0:
		g.nrambank = 3 // banques 5 et 6 inversées (TO770/TO9)
	case 0x60:
		g.nrambank = 4
	case 0x20:
		g.nrambank = 5
	default:
		return
	}
	g.rambankBase = -0x2000 + (g.nrambank << 14)
}

// updateROMBank positionne la banque ROM (réf C : TO8rombank()). Trois cas :
// recouvrement par RAM (e7e6 bit5), ROM interne BASIC (e7c3 bit2) ou cartouche.
func (g *GateArray) updateROMBank() {
	// e7e6 bit5 : l'espace ROM est recouvert par la banque RAM des 5 bits de
	// poids faible de e7e6 (les deux segments de 8 Ko sont inversés à l'accès).
	if g.port[0x26]&0x20 != 0 {
		g.rombankTgt = targetRAM
		g.rombankBase = (int(g.port[0x26]) & 0x1f) << 14
		return
	}
	// e7c3 bit2 : commutation ROM interne (BASIC) vs cartouche.
	if g.port[0x03]&0x04 != 0 {
		g.nrombank = g.carflags & 3
		g.rombankTgt = targetBasic
		g.rombankBase = g.nrombank << 14
	} else {
		g.nrombank = -1
		g.rombankTgt = targetCart
		g.rombankBase = (g.carflags & 3) << 14
	}
}

// rombankRead lit dans l'espace ROM courant à l'offset off (déjà ajusté pour
// l'inversion des segments en mode recouvrement).
func (g *GateArray) rombankRead(off int) byte {
	switch g.rombankTgt {
	case targetRAM:
		return g.ram[g.rombankBase+off]
	case targetBasic:
		return g.romBasic[g.rombankBase+off]
	default:
		return g.car[g.rombankBase+off]
	}
}

// rombankWrite écrit dans l'espace ROM courant (en pratique seulement la RAM en
// mode recouvrement ; la réf C écrit néanmoins dans la cible courante).
func (g *GateArray) rombankWrite(off int, c byte) {
	switch g.rombankTgt {
	case targetRAM:
		g.ram[g.rombankBase+off] = c
	case targetBasic:
		g.romBasic[g.rombankBase+off] = c
	default:
		g.car[g.rombankBase+off] = c
	}
}

// romsysRead lit la ROM moniteur système (banque via e7c3 bit4).
func (g *GateArray) romsysRead(a int) byte { return g.romMon[g.romsysBase+a] }

// ── Bus mémoire (cpu6809.Bus) ─────────────────────────────────────────────────

// Read8 lit un octet sur le bus gate-array. Réf C : Mgetto8d().
func (g *GateArray) Read8(a uint16) uint8 {
	switch a >> 12 {
	case 0x0, 0x1:
		// Recouvrement : les 2 segments de 8 Ko sont inversés (0x0–0x1 ↔ 0x2–0x3).
		if g.port[0x26]&0x20 != 0 {
			return g.rombankRead(int(a) + 0x2000)
		}
		return g.rombankRead(int(a))
	case 0x2, 0x3:
		if g.port[0x26]&0x20 != 0 {
			return g.rombankRead(int(a) - 0x2000)
		}
		return g.rombankRead(int(a))
	case 0x4, 0x5:
		return g.ram[g.ramvideoBase+int(a)]
	case 0x6, 0x7, 0x8, 0x9:
		return g.ram[g.ramuserBase+int(a)]
	case 0xa, 0xb, 0xc, 0xd:
		return g.ram[g.rambankBase+int(a)]
	case 0xe:
		return g.readIO(a)
	default:
		return g.romsysRead(int(a))
	}
}

// Write8 écrit un octet sur le bus gate-array. Réf C : Mputto8d().
func (g *GateArray) Write8(a uint16, c uint8) {
	switch a >> 12 {
	case 0x0, 0x1:
		// Hors recouvrement, écrire dans l'espace ROM commute la banque cartouche
		// (carflags = a&3). Réf C : Switchmemo7 inline.
		if g.port[0x26]&0x20 == 0 {
			g.carflags = (g.carflags & 0xfc) | (int(a) & 3)
			g.updateROMBank()
		}
		// Écriture mémoire autorisée seulement si e7e6 bits 5 ET 6 sont armés.
		if g.port[0x26]&0x60 != 0x60 {
			return
		}
		if g.port[0x26]&0x20 != 0 {
			g.rombankWrite(int(a)+0x2000, c)
		} else {
			g.rombankWrite(int(a), c)
		}
	case 0x2, 0x3:
		if g.port[0x26]&0x60 != 0x60 {
			return
		}
		if g.port[0x26]&0x20 != 0 {
			g.rombankWrite(int(a)-0x2000, c)
		} else {
			g.rombankWrite(int(a), c)
		}
	case 0x4, 0x5:
		g.ram[g.ramvideoBase+int(a)] = c
	case 0x6, 0x7, 0x8, 0x9:
		g.ram[g.ramuserBase+int(a)] = c
	case 0xa, 0xb, 0xc, 0xd:
		g.ram[g.rambankBase+int(a)] = c
	case 0xe:
		g.writeIO(a, c)
	default:
		// 0xF : ROM système, lecture seule.
	}
}

// ── Ports d'E/S — registres de banking (lot #112) ─────────────────────────────
//
// Seuls les registres qui pilotent le banking sont traités ici ; les ports vidéo
// (e7da/dc/dd), timer 6846 (e7c5/c6/c7), PIA/son (e7cd…) et leurs effets sont
// ajoutés par les lots #113/#114. Les autres écritures sont simplement stockées
// dans port[] (comportement minimal, étendu plus tard).

func (g *GateArray) writeIO(a uint16, c byte) {
	switch a {
	case 0xe7c3:
		// p0=page vidéo, p2=commutation ROM, p4=banque système (cf. réf C).
		g.port[0x03] = c & 0x3d
		g.updateVideoRAM()
		g.updateROMBank()
	case 0xe7c9:
		g.port[0x09] = c
		g.updateRAMBank()
	case 0xe7e4:
		g.port[0x24] = c
	case 0xe7e5:
		g.port[0x25] = c
		g.updateRAMBank()
	case 0xe7e6:
		g.port[0x26] = c
		g.updateROMBank()
	case 0xe7e7:
		g.port[0x27] = c
		g.updateRAMBank()
	default:
		if a >= 0xe7c0 && a < 0xe800 {
			g.port[a&0x3f] = c
		}
	}
}

func (g *GateArray) readIO(a uint16) byte {
	switch a {
	case 0xe7e4:
		return g.port[0x1d] & 0xf0
	case 0xe7e5:
		return g.port[0x25] & 0x1f
	case 0xe7e6:
		return g.port[0x26] & 0x7f
	case 0xe7e7:
		// Bits de synchro vidéo (Initn/Iniln) ajoutés au lot #113 ; ici le seul
		// bit significatif est port[0x24] bit0.
		return g.port[0x24] & 0x01
	default:
		if a < 0xe7c0 {
			return g.romsysRead(int(a))
		}
		if a < 0xe800 {
			return g.port[a&0x3f]
		}
		return g.romsysRead(int(a))
	}
}
