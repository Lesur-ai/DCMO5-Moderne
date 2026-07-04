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

import (
	"github.com/Lesur-ai/dcmo5/internal/cpu6809"
	"github.com/Lesur-ai/dcmo5/internal/media"
)

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

	// Vidéo (lot #113). x7da : palette EF9369 (16 couleurs × 2 octets : r|v puis b).
	// pagevideoBase : offset dans ram[] de la page AFFICHÉE par le balayage (e7dd,
	// distincte de ramvideo qui est la page vue par le CPU). bordercolor : index
	// palette de la bordure. vmode : mode de décodage courant (e7dc).
	x7da          [32]byte
	pcolor        [16]uint32 // palette RGBA RENDUE (latchée), cf. paletteWrite
	scanout       [xbitmap * ybitmap]uint32
	scanoutValid  bool
	scanLine      int
	scanSegment   int
	pagevideoBase int
	bordercolor   int
	vmode         videoMode

	// Timer 6846 + lignes d'IRQ (lot #114). timer6846 : compteur courant (en
	// 1/8 de cycle, réf C) ; latch6846 : valeur de rechargement. timerIRQCount /
	// keybIRQCount : durée résiduelle (en cycles) du signal d'IRQ correspondant.
	timer6846     int
	latch6846     int
	timerIRQCount int
	keybIRQCount  int

	// Son (lot #115) : niveau du haut-parleur 0..0x3F (e7cd), exposé au moteur via
	// SoundLevel().
	sound uint8

	// Joystick TO8D (Inc J1a) : état idempotent des deux manettes, en LOGIQUE
	// INVERSÉE (0 bit = direction/bouton appuyé). joysPosition = 4 bits J1 dans le
	// nibble bas + 4 bits J2 dans le nibble haut ; joysAction = bits 6/7 pour
	// fire J1/J2 (bits 0..5 inutilisés, restent à 1). Repos : 0xFF, 0xC0 (cf.
	// machine.NeutralJoystick). Lus via 0xe7cc / 0xe7cd quand port[0x0e/0x0f]
	// bit2 sélectionne le mode joystick (mux hardware, cf. readIO).
	joysPosition uint8
	joysAction   uint8

	// CPU et périphériques (lot #115). cpu : référence pour les handlers d'E/S
	// (registres A/B/X/Y/S/CC) ; attachée par AttachCPU à l'intégration moteur
	// (#118). tape/disk/printer : médias montés. xpen/ypen/penbutton : pointeur
	// (crayon optique / souris), en repère écran TO8D.
	cpu *cpu6809.CPU

	// beam fournit la position courante du balayage vidéo (cycle dans la ligne,
	// numéro de ligne). Nécessaire aux registres de synchro faisceau lus par le
	// firmware : e7e7 (Initn trame | Iniln ligne) et e7ca. Injecté par la machine à
	// l'intégration moteur via AttachBeam(eng.VideoBeam) (#118) ; nil ⇒ (0,0) pour
	// les tests isolés du gate-array.
	beam       func() (linecycle, linenumber int)
	tape       media.Tape
	disk       media.Disk
	printer    media.PrinterSink
	xpen, ypen int
	penbutton  bool

	// k7bit/k7octet : état du lecteur cassette bit à bit (réf C dcto8ddevices.c).
	k7bit   uint8
	k7octet uint8

	// Clavier gate-array. touche[k] : état idempotent par touche machine
	// (0x00 = enfoncée, 0x80 = relâchée). keyboard décrit la variante matérielle
	// (TO8D aujourd'hui ; TO9+ plus tard). capslock : verrou majuscules, posé à
	// true au hard reset uniquement (réf C : Hardreset).
	touche   [keyboardKeyMax]byte
	capslock bool
	keyboard keyboardDef
}

// videoMode est le mode de décodage vidéo gate-array (sélection par e7dc).
type videoMode int

const (
	mode320x16       videoMode = iota // standard TO : 320×16 couleurs (défaut)
	mode320x4                         // bitmap4 : 320×200, 4 couleurs
	mode320x4special                  // bitmap4 spécial : 320×200, 4 couleurs
	mode160x16                        // bitmap16 : 160×200, 16 couleurs
	mode640x2                         // 80 colonnes : 640×200, 2 couleurs
)

// New construit un gate-array avec le clavier TO8D historique. romMon (≤ 16 Ko,
// moniteur système) et romBasic (≤ 64 Ko, ROM interne) sont copiés dans des
// tampons de taille fixe (tronqués au besoin, complétés de zéros). La machine
// est mise en état de reset matériel.
func New(romMon, romBasic []byte) *GateArray {
	return newWithKeyboard(romMon, romBasic, to8dKeyboardDef)
}

// NewTO9P construit un gate-array avec la variante clavier TO9+ : les touches
// publient un code ASCII sur E7DF via le flag E7DE, au lieu d'écrire le scancode
// dans les offsets moniteur TO8D. Le reste du Device gate-array reste partagé.
func NewTO9P(romMon, romBasic []byte) *GateArray {
	return newWithKeyboard(romMon, romBasic, to9pKeyboardDef)
}

func newWithKeyboard(romMon, romBasic []byte, keyboard keyboardDef) *GateArray {
	g := &GateArray{}
	g.keyboard = keyboard
	copy(g.romMon[:], romMon)
	copy(g.romBasic[:], romBasic)
	g.hardReset()
	return g
}

// resetRAM réamorce les n premiers octets de la RAM dans le motif de mise sous
// tension : 0x00 si le bit 7 de l'index physique est à 0, 0xFF sinon (réf C :
// ram[i] = -((i & 0x80) >> 7)). L'étendue distingue les deux appelants : Hardreset
// réamorce TOUTE la RAM (n = len(ram)), tandis que Loadmemo (montage cartouche)
// n'en réamorce que les premiers 0xc000 octets, laissant les banques RAM hautes
// intactes (réf C dcto8ddevices.c:237).
func (g *GateArray) resetRAM(n int) {
	for i := 0; i < n; i++ {
		if i&0x80 != 0 {
			g.ram[i] = 0xFF
		} else {
			g.ram[i] = 0x00
		}
	}
}

// hardReset reproduit Hardreset() : RAM en motif 0x00/0xFF (bit 7 de l'adresse),
// ports à zéro sauf e7c9 (port[0x09]=0x0f), cartouche effacée, puis Initprog.
func (g *GateArray) hardReset() {
	g.resetRAM(len(g.ram))
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
	g.timerIRQCount = 0
	g.keybIRQCount = 0
	g.initprog()
	g.capslock = true  // réf C : capslock = 1 posé dans Hardreset uniquement (pas Initprog)
	g.refreshPalette() // initialise la palette rendue depuis x7da
	g.latch6846 = 65535
	g.timer6846 = 65535
	g.sound = 0
	g.joysPosition = 0xFF // Inc J1a : repos = toutes directions relâchées (logique inversée)
	g.joysAction = 0xC0   // Inc J1a : repos = boutons fire J1/J2 relâchés (bits 6/7), bits 0..5 à 1
	g.penbutton = false
	g.xpen, g.ypen = 0, 0
	g.k7bit, g.k7octet = 0, 0
	g.scanoutValid = false
	g.scanLine = -1
	g.scanSegment = 0
}

// AttachCPU relie le CPU utilisé par les handlers d'E/S (lecture/écriture des
// registres A/B/X/Y/S/CC). Appelé à la construction de la machine TO8D, lors de
// l'intégration au moteur (#118), avec eng.CPU().
func (g *GateArray) AttachCPU(cpu *cpu6809.CPU) { g.cpu = cpu }

// AttachBeam injecte la source de position du balayage vidéo (typiquement
// engine.Engine.VideoBeam), lue par les registres de synchro faisceau e7e7 et e7ca.
// Sans elle, le firmware boucle indéfiniment sur la synchro vidéo au boot (#118).
func (g *GateArray) AttachBeam(beam func() (linecycle, linenumber int)) { g.beam = beam }

// videoBeam retourne la position courante du balayage, ou (0,0) si non câblée
// (gate-array testé en isolation, sans moteur).
func (g *GateArray) videoBeam() (linecycle, linenumber int) {
	if g.beam == nil {
		return 0, 0
	}
	return g.beam()
}

// iniln reproduit Iniln() (dcto8demulation.c) : 0x20 dans la fenêtre active de ligne
// (cycles 11..51), 0 sinon (synchro horizontale).
func (g *GateArray) iniln() int {
	c, _ := g.videoBeam()
	if c < 11 || c > 51 {
		return 0
	}
	return 0x20
}

// initn reproduit Initn() (dcto8demulation.c) : 0x80 dans la zone active de trame
// (lignes 56..255, avec les bords de ligne sur les lignes limites), 0 sinon (synchro
// verticale / VBL).
func (g *GateArray) initn() int {
	c, n := g.videoBeam()
	if n < 56 || n > 255 {
		return 0
	}
	if n == 56 && c < 12 {
		return 0
	}
	if n == 255 && c > 50 {
		return 0
	}
	return 0x80
}

// SoundLevel retourne le niveau courant du haut-parleur (0..0x3F), échantillonné
// par le moteur (contrat engine.Device).
func (g *GateArray) SoundLevel() uint8 { return g.sound }

// initprog reproduit Initprog() (partie mémoire) : recalcule tous les pointeurs
// de banque depuis l'état des ports. ramuser est fixe (ram - 0x2000).
func (g *GateArray) initprog() {
	for i := range g.touche {
		g.touche[i] = 0x80 // touches relâchées (réf C Initprog : touche[i] = 0x80)
	}
	// Inc J1a : reset des entrées joystick au repos. Le clavier est reset ici
	// par symétrie (transitoire côté hôte) ; le joystick doit l'être aussi —
	// sinon Initprog() (déclenché par bouton overlay, media error, ou cartouche
	// montée) laisserait des bits direction/fire appuyés visibles côté CPU via
	// 0xe7cc/0xe7cd. Codex review #171 P2 confirmé. Repos = 0xFF / 0xC0 (cf.
	// machine.NeutralJoystick, logique inversée).
	g.joysPosition = 0xFF
	g.joysAction = 0xC0
	g.carflags &= 0xec
	// Mode de décodage forcé en standard, SANS relire e7dc (réf C Initprog :
	// Decodevideo = Decode320x16, dcto8demulation.c:330). Le registre e7dc (port[0x1c])
	// n'est pas réaligné ici : après un Initprog déclenché par MountCartridge (qui
	// préserve les ports, réf C Loadmemo), le décodage repasse en 320x16 tandis que
	// e7dc garde sa valeur — état post-Initprog fidèle, que le firmware réécrit ensuite.
	g.vmode = mode320x16
	g.ramuserBase = -0x2000
	g.videopageBorder(g.port[0x1d])
	g.updateVideoRAM()
	g.updateRAMBank()
	g.updateROMBank()
}

// videopageBorder positionne la page vidéo AFFICHÉE et la couleur de bordure
// depuis e7dd (réf C : Videopage_bordercolor). Bits 6-7 = page affichée (offset
// (c&0xc0)<<8 dans ram), bits 0-3 = index palette de la bordure.
func (g *GateArray) videopageBorder(c byte) {
	g.port[0x1d] = c
	g.pagevideoBase = (int(c) & 0xc0) << 8
	g.bordercolor = int(c) & 0x0f
}

// setVideoMode sélectionne le mode de décodage depuis e7dc (réf C : TO8videomode).
func (g *GateArray) setVideoMode(c byte) {
	g.port[0x1c] = c
	switch c {
	case 0x21:
		g.vmode = mode320x4
	case 0x2a:
		g.vmode = mode640x2
	case 0x41:
		g.vmode = mode320x4special
	case 0x7b:
		g.vmode = mode160x16
	default:
		g.vmode = mode320x16
	}
}

// paletteWrite traite une écriture e7da (réf C : Palettecolor). Les 32 octets de
// x7da forment 16 couleurs (octet pair : r en bits0-3, v en bits4-7 ; octet
// impair : b en bits0-3). port[0x1b] est l'index auto-incrémenté (modulo 32). La
// couleur RGBA est recalculée à la volée au décodage (palette24 / DecodeFrame).
func (g *GateArray) paletteWrite(c byte) {
	i := int(g.port[0x1b]) & 0x1f
	g.x7da[i] = c
	g.port[0x1b] = byte((i + 1) & 0x1f)
	// La couleur RENDUE n'est latchée qu'à l'écriture du 2e octet (index impair),
	// comme la réf C Palettecolor : tant que le 2e octet n'est pas écrit, pcolor
	// garde l'ancienne valeur. Évite une couleur transitoire fausse en cas
	// d'écriture fractionnée ou d'animation de palette décodée entre les 2 octets.
	if i&1 != 0 {
		even := int(g.x7da[i&0x1e])
		g.pcolor[i>>1] = rgbaFromRVB(even&0x0f, (even>>4)&0x0f, int(c)&0x0f)
	}
}

// Reset relance la machine dans l'état de reset matériel (efface la RAM).
func (g *GateArray) Reset() { g.hardReset() }

// Initprog effectue un reset DOUX (RAM et ports conservés) : recalcule les pointeurs
// de banque depuis l'état des ports et recharge le vecteur reset du CPU. C'est le
// pendant public de softReset, pour le contrat machine.Machine (famille TO, #118).
func (g *GateArray) Initprog() { g.softReset() }

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
	// Repartir sur la banque cartouche 0 : sans cela, un (re)chargement après que
	// le guest a sélectionné une banque non nulle mapperait la nouvelle cartouche
	// sur une banque obsolète (cf. réf C Loadmemo / core MO5 loadCartridge).
	g.carflags &= 0xfc
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
		// p5 (0x20) = acknowledge réception d'un code touche : l'effacer acquitte
		// l'IRQ clavier (réf C : if((c & 0x20) == 0) keyb_irqcount = 0). Sans cela
		// la ligne IRQKeyboard resterait assertée jusqu'au timeout (~500 ms).
		if c&0x20 == 0 {
			g.keybIRQCount = 0
		}
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
	case 0xe7da:
		g.paletteWrite(c)
	case 0xe7db:
		g.port[0x1b] = c
	case 0xe7dc:
		g.setVideoMode(c)
	case 0xe7dd:
		g.videopageBorder(c)
	case 0xe7cd:
		// Registre action/musique : si le bit2 de e7cf sélectionne la musique,
		// l'octet écrit est le niveau du haut-parleur (réf C). Sinon port standard.
		if g.port[0x0f]&4 != 0 {
			g.sound = c & 0x3f
		} else {
			g.port[0x0d] = c
		}
	case 0xe7c5:
		g.port[0x05] = c
		g.timerControl() // recharge le compteur si bit0 armé (réf C Timercontrol)
	case 0xe7c6:
		g.latch6846 = (g.latch6846 & 0xff) | (int(c) << 8) // octet de poids fort
	case 0xe7c7:
		g.latch6846 = (g.latch6846 & 0xff00) | int(c) // octet de poids faible
	default:
		if a >= 0xe7c0 && a < 0xe800 {
			g.port[a&0x3f] = c
		}
	}
}

func (g *GateArray) readIO(a uint16) byte {
	switch a {
	case 0xe7c0:
		// CSR composite (6846) : si au moins une source d'IRQ est active, le bit7
		// composite est armé (réf C : port[0] ? port[0]|0x80 : 0).
		if g.port[0x00] != 0 {
			return g.port[0x00] | 0x80
		}
		return 0
	case 0xe7c3:
		// Registre d'état port C : bit7 toujours armé, bit1 = interrupteur crayon
		// optique / clic souris (penbutton). Réf C : port[0x03]|0x80|(penbutton<<1).
		v := g.port[0x03] | 0x80
		if g.penbutton {
			v |= 0x02
		}
		return v
	case 0xe7cc:
		// Registre direction joystick (Inc J1a) : si le bit2 de e7ce sélectionne le
		// joystick, retourne g.joysPosition (en logique inversée, 0=appuyé) ; sinon
		// port[0x0c] standard. Réf C dcto8demulation.c Mgetto8d :
		// (port[0x0e]&4) ? joysposition : port[0x0c].
		if g.port[0x0e]&4 != 0 {
			return g.joysPosition
		}
		return g.port[0x0c]
	case 0xe7cd:
		// Registre action/musique : si le bit2 de e7cf sélectionne le mode musique/
		// joystick, on retourne g.joysAction OR g.sound (les deux occupent le même
		// registre matériel — le canal son et les boutons fire sont OR'és par le
		// hardware). Réf C dcto8demulation.c Mgetto8d :
		// (port[0x0f]&4) ? joysaction|sound : port[0x0d].
		// Inc J1a : fix bug latent — l'ancienne implémentation retournait g.sound
		// seul, donc taper le bouton fire J1/J2 n'aurait pas atteint le CPU même
		// après câblage de SetJoystick. joysAction au repos = 0xC0, donc l'OR ne
		// change rien tant qu'aucun bouton n'est pressé (compat tests existants).
		if g.port[0x0f]&4 != 0 {
			return g.joysAction | g.sound
		}
		return g.port[0x0d]
	case 0xe7c6:
		return byte(g.timer6846 >> 11 & 0xff) // timer, octet de poids fort
	case 0xe7c7:
		return byte(g.timer6846 >> 3 & 0xff) // timer, octet de poids faible
	case 0xe7da:
		// Lecture palette : index auto-incrémenté (post-incrément non masqué au
		// stockage, masqué à l'indexation — réf C : x7da[port[0x1b]++ & 0x1f]).
		v := g.x7da[g.port[0x1b]&0x1f]
		g.port[0x1b]++
		return v
	case 0xe7df:
		v := g.port[0x1f]
		if g.keyboard.clearPendingOnASCIIRead {
			g.port[0x1e] = 0x00
		}
		return v
	case 0xe7e4:
		return g.port[0x1d] & 0xf0
	case 0xe7e5:
		return g.port[0x25] & 0x1f
	case 0xe7e6:
		return g.port[0x26] & 0x7f
	case 0xe7ca:
		// Registre de contrôle PIA (réf C Mgetto8d) : 2 hors zone affichable
		// (videolinenumber ≥ 200), 0 sinon. Synchro trame scrutée par le firmware.
		if _, n := g.videoBeam(); n < 200 {
			return 0
		}
		return 2
	case 0xe7e7:
		// bit0 (port[0x24]) + bits de synchro faisceau : Initn (trame) | Iniln (ligne).
		// Réf C Mgetto8d : (port[0x24] & 0x01) | Initn() | Iniln(). Indispensable au
		// boot : le moniteur scrute ce registre pour se caler sur le balayage (#118).
		return g.port[0x24]&0x01 | byte(g.initn()) | byte(g.iniln())
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
