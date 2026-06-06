// Package core représente la machine Thomson MO5 complète.
// Il ne dépend d'aucune bibliothèque graphique, audio ni de chemins fichiers.
package core

import (
	"fmt"

	"github.com/Lesur-ai/dcmo5/internal/cpu6809"
	"github.com/Lesur-ai/dcmo5/internal/media"
	"github.com/Lesur-ai/dcmo5/internal/spec"
)

// Key identifie une touche du clavier MO5 (index dans [0, spec.KeyMax)).
type Key int

// JoystickInput décrit l'état instantané des deux manettes.
type JoystickInput struct {
	Position uint8 // axes des deux manettes (4 bits par manette)
	Action   uint8 // boutons d'action
}

// Options configure la machine au démarrage.
type Options struct {
	ROMSys          []byte            // ROM système 16 Ko (nil = ROM absente)
	Tape            media.Tape        // cassette montée, ou nil
	Disk            media.Disk        // disquette montée, ou nil
	Cartridge       media.Cartridge   // cartouche montée, ou nil
	Printer         media.PrinterSink // imprimante, ou nil
	AudioSampleRate int               // taux d'échantillonnage audio (0 = spec.AudioSampleRate)
}

// Machine représente le Thomson MO5 complet.
type Machine struct {
	cpu  *cpu6809.CPU
	opts Options

	// Mémoire physique
	ram  [spec.RAMTotalSize]uint8 // 48 Ko RAM (vidéo + utilisateur)
	rom  [0x4000]uint8            // 16 Ko ROM système (0xC000–0xFFFF)
	car  [0x10000]uint8           // 4 banques × 16 Ko cartouche
	port [spec.PortSize]uint8     // 64 octets ports E/S

	// État mémoire banked
	cartype  int   // 0=simple 1=MEMO5 switch 2=OS-9
	carflags uint8 // bits0-1=banque bits2=cart-active bit3=write-en bits4=OS9bank

	// Entrées
	touche       [spec.KeyMax]uint8 // 0x00=pressée 0x80=relâchée
	joysPosition uint8              // axes manettes
	joysAction   uint8              // boutons d'action
	xpen, ypen   int
	penbutton    bool

	// Lecteur cassette bit-level (ref: dcmo5devices.c k7bit/k7octet)
	k7bit   uint8 // masque du bit en cours (0x80→0x01) ; 0 = recharger un octet
	k7octet uint8 // octet cassette courant en cours de lecture bit à bit

	// Son (ref: dcmo5main.c). sound = niveau courant du haut-parleur (0..0x3F),
	// mis à jour par les ports 0xA7C1/0xA7CD. Échantillonné dans Step() à
	// spec.AudioSampleRate via un accumulateur de cycles, dans samples.
	sound           uint8   // niveau sonore courant (6 bits)
	sampleAccum     int64   // accumulateur cycles×SampleRate pour l'échantillonnage
	samples         []uint8 // tampon d'échantillons audio (niveau 0..0x3F)
	audioSampleRate int     // taux d'échantillonnage effectif

	// Timing vidéo (ref: dcmo5emulation.c Run())
	// 64 cycles par ligne, 312 lignes par trame (50 Hz)
	videolinecycle  int // cycles dans la ligne courante [0,63]
	videolinenumber int // numéro de ligne courante [0,311]
}

// NewMachine crée une machine avec les options fournies.
func NewMachine(opts Options) (*Machine, error) {
	if len(opts.ROMSys) != 0 && len(opts.ROMSys) != 0x4000 {
		return nil, fmt.Errorf("core: ROMSys doit faire exactement 0x4000 octets, reçu %d", len(opts.ROMSys))
	}
	m := &Machine{opts: opts}
	m.audioSampleRate = opts.AudioSampleRate
	if m.audioSampleRate <= 0 {
		m.audioSampleRate = spec.AudioSampleRate
	}
	if len(opts.ROMSys) == 0x4000 {
		copy(m.rom[:], opts.ROMSys)
	}
	m.hardReset()
	m.loadCartridge() // charge opts.Cartridge dans car[] si présente
	m.cpu = cpu6809.New(m)
	return m, nil
}

// hardReset initialise la RAM, les ports et l'état interne.
// Ref: dcmo5emulation.c Hardreset()
func (m *Machine) hardReset() {
	for i := range m.ram {
		// Pattern d'init : alternance 0x00/0xFF selon bit 7 de l'index
		if i&0x80 != 0 {
			m.ram[i] = 0xFF
		} else {
			m.ram[i] = 0x00
		}
	}
	for i := range m.port {
		m.port[i] = 0
	}
	for i := range m.car {
		m.car[i] = 0
	}
	for i := range m.touche {
		m.touche[i] = 0x80 // touches relâchées
	}
	m.joysPosition = 0xFF // manettes au centre
	m.joysAction = 0xC0   // boutons relâchés
	m.carflags = 0
	m.cartype = 0
	m.xpen, m.ypen = 0, 0
	m.penbutton = false
	m.videolinecycle = 0
	m.videolinenumber = 0
	// État audio : repartir silencieux, sans échantillons périmés ni reliquat
	// d'accumulateur (sinon DrainAudio rejouerait du son d'avant le reset).
	m.sound = 0
	m.sampleAccum = 0
	m.samples = m.samples[:0]
	m.k7bit = 0
	m.k7octet = 0
	m.mo5VideoRAM()
}

// ── Sélection de banques ──────────────────────────────────────────────────────

// mo5VideoRAM actualise ramVideoOffset selon port[0]&1.
// Retourne l'offset dans ram[] pour l'adresse 0x0000.
// - bit0=0 : RAM vidéo couleurs à 0x0000 (offset 0)
// - bit0=1 : RAM vidéo couleurs à 0x2000 (offset 0x2000)
func (m *Machine) mo5VideoRAM() {
	// pas d'offset explicite nécessaire : on encode dans Read8/Write8
}

// videoBase retourne l'offset de la page vidéo active dans ram[].
func (m *Machine) videoBase() uint16 {
	if m.port[0]&1 != 0 {
		return 0x2000
	}
	return 0x0000
}

// romBankBase retourne le pointeur de base de la ROM banque active.
// Ref: dcmo5emulation.c MO5rombank()
func (m *Machine) romBankBase() uint32 {
	if m.carflags&4 == 0 {
		// pas de cartouche : on lit dans rom[]
		return 0 // indicateur "utiliser ROM sys" — géré dans Read8
	}
	// cartouche active : base dans car[]
	base := uint32((m.carflags & 0x03)) << 14
	if m.cartype == 2 && m.carflags&0x10 != 0 {
		base += 0x10000
	}
	return base
}

// ── Bus mémoire MO5 ─────────────────────────────────────────────────────────

// Read8 implémente cpu6809.Bus — lecture d'un octet sur le bus MO5.
// Ref: dcmo5emulation.c MgetMO5()
func (m *Machine) Read8(addr uint16) uint8 {
	switch addr >> 12 {
	case 0x0, 0x1: // RAM vidéo (couleurs ou formes selon page active)
		return m.ram[m.videoBase()+addr]
	case 0x2, 0x3, 0x4, 0x5, 0x6, 0x7, 0x8, 0x9:
		// RAM utilisateur (CPU 0x2000-0x9FFF) → ram[addr+0x2000] = ram[0x4000-0xBFFF]
		// ram[0x2000-0x3FFF] est réservé à la page vidéo 1 (formes), pas aliasée ici.
		return m.ram[addr+0x2000]
	case 0xA:
		return m.readPort(addr)
	case 0xB:
		m.switchMemo5Bank(addr)
		return m.readROMBank(addr)
	case 0xC, 0xD, 0xE:
		return m.readROMBank(addr)
	case 0xF:
		return m.rom[addr-0xC000]
	default:
		return m.ram[addr+0x2000]
	}
}

// Write8 implémente cpu6809.Bus — écriture d'un octet sur le bus MO5.
// Ref: dcmo5emulation.c MputMO5()
func (m *Machine) Write8(addr uint16, v uint8) {
	switch addr >> 12 {
	case 0x0, 0x1:
		m.ram[m.videoBase()+addr] = v
	case 0x2, 0x3, 0x4, 0x5, 0x6, 0x7, 0x8, 0x9:
		m.ram[addr+0x2000] = v
	case 0xA:
		m.writePort(addr, v)
	case 0xB, 0xC, 0xD, 0xE:
		// Écriture cartouche : write-enable (bit3) + cart active (bit2) + cart simple (type 0)
		if m.carflags&8 != 0 && m.carflags&4 != 0 && m.cartype == 0 {
			base := m.romBankBase()
			m.car[base+(uint32(addr)-0xB000)] = v
		}
	case 0xF:
		// ROM sys read-only : écriture ignorée
	default:
		m.ram[addr+0x2000] = v
	}
}

// ── Ports E/S ────────────────────────────────────────────────────────────────

// readPort lit un port d'E/S MO5.
// Ref: dcmo5emulation.c MgetMO5() case 0xa
func (m *Machine) readPort(addr uint16) uint8 {
	switch addr {
	case 0xA7C0:
		penBit := uint8(0)
		if m.penbutton {
			penBit = 0x20
		}
		return m.port[0] | 0x80 | penBit
	case 0xA7C1:
		col := (m.port[1] & 0xFE) >> 1
		if int(col) >= len(m.touche) {
			col = 0
		}
		return m.port[1] | m.touche[col]
	case 0xA7C2:
		return m.port[2]
	case 0xA7C3:
		// bit7 = ~Initn() : 1 hors zone active (lignes 56-255), 0 dans zone active
		return m.port[3] | uint8(^m.initn()&0xFF)
	case 0xA7CB:
		return (m.carflags & 0x3F) | ((m.carflags & 0x80) >> 1) | ((m.carflags & 0x40) << 1)
	case 0xA7CC:
		if m.port[0x0E]&4 != 0 {
			return m.joysPosition
		}
		return m.port[0x0C]
	case 0xA7CD:
		// Ref C : (port[0x0F]&4) ? joysaction | sound : port[0x0d].
		// Le niveau son courant est reflété dans la lecture (registre musique).
		if m.port[0x0F]&4 != 0 {
			return m.joysAction | m.sound
		}
		return m.port[0x0D]
	case 0xA7CE:
		return 4
	case 0xA7D8:
		// état disquette : ~Initn() (ref C)
		return uint8(^m.initn() & 0xFF)
	case 0xA7E1:
		return 0xFF
	case 0xA7E6:
		// Iniln() << 1 : bit de synchro ligne (ref C)
		return uint8(m.iniln() << 1)
	case 0xA7E7:
		// Initn() : bit de synchro trame (ref C)
		return uint8(m.initn())
	default:
		if addr < 0xA7C0 {
			return 0 // CD90-640 ROM (hors périmètre v1)
		}
		if addr < 0xA800 {
			return m.port[addr&0x3F]
		}
		return 0
	}
}

// writePort écrit dans un port d'E/S MO5.
// Ref: dcmo5emulation.c MputMO5() case 0xa
func (m *Machine) writePort(addr uint16, v uint8) {
	switch addr {
	case 0xA7C0:
		m.port[0] = v & 0x5F
		m.mo5VideoRAM()
	case 0xA7C1:
		m.port[1] = v & 0x7F
		m.sound = (v & 1) << 5 // bit haut-parleur → niveau 0 ou 32
	case 0xA7C2:
		m.port[2] = v & 0x3F
	case 0xA7C3:
		m.port[3] = v & 0x3F
	case 0xA7CB:
		m.carflags = v
	case 0xA7CC:
		m.port[0x0C] = v
	case 0xA7CD:
		m.port[0x0D] = v
		m.sound = v & spec.AudioLevelMax // registre niveau musique/son (6 bits)
	case 0xA7CE:
		m.port[0x0E] = v
	case 0xA7CF:
		m.port[0x0F] = v
	default:
		if addr >= 0xA7C0 && addr < 0xA800 {
			m.port[addr&0x3F] = v
		}
	}
}

// ── ROM banque ────────────────────────────────────────────────────────────────

func (m *Machine) readROMBank(addr uint16) uint8 {
	if m.carflags&4 == 0 {
		// Pas de cartouche : rombank = mo5rom - 0xC000 (ref C MO5rombank).
		// 0xC000-0xEFFF lisent dans m.rom[], 0xB000-0xBFFF = hors ROM → 0.
		if addr >= 0xC000 {
			return m.rom[addr-0xC000]
		}
		return 0
	}
	base := m.romBankBase()
	offset := uint32(addr) - 0xB000
	idx := base + offset
	if int(idx) < len(m.car) {
		return m.car[idx]
	}
	return 0
}

// switchMemo5Bank gère la commutation de banque MEMO5.
// Ref: dcmo5emulation.c Switchmemo5bank()
func (m *Machine) switchMemo5Bank(addr uint16) {
	if m.cartype != 1 {
		return
	}
	if addr&0xFFFC != 0xBFFC {
		return
	}
	m.carflags = (m.carflags & 0xFC) | (uint8(addr) & 3)
}

// ── Interface publique ────────────────────────────────────────────────────────

// Reset réinitialise la machine.
func (m *Machine) Reset() {
	m.hardReset()
	m.loadCartridge()
	m.cpu.Reset()
}

const (
	cyclesPerLine = 64  // cycles par ligne horizontale MO5
	linesPerFrame = 312 // lignes par trame (50 Hz)
)

// Step avance l'émulation d'au plus n cycles et retourne les cycles consommés.
// Reproduit fidèlement la boucle dcmo5emulation.c Run() :
//   - opcode illégal → entreesortie(-code) + 64 cycles (I/O)
//   - tous les 64 cycles → fin de ligne (videolinecycle, videolinenumber++)
//   - toutes les 312 lignes → IRQ 50 Hz (trame complète)
func (m *Machine) Step(cycles int) int {
	if cycles <= 0 {
		return 0
	}
	consumed := 0
	for consumed < cycles {
		c := m.cpu.Step()
		if c < 0 {
			m.entreesortie(-c)
			c = 64
		} else if c == 0 {
			c = 2
		}
		consumed += c

		// Échantillonnage audio : produit m.audioSampleRate échantillons par
		// spec.CPUClockHz cycles, en capturant le niveau sonore courant.
		m.sampleAccum += int64(c) * int64(m.audioSampleRate)
		for m.sampleAccum >= int64(spec.CPUClockHz) {
			m.sampleAccum -= int64(spec.CPUClockHz)
			m.appendSample(m.sound)
		}

		// Timing vidéo (ref: dcmo5emulation.c Run())
		m.videolinecycle += c
		for m.videolinecycle >= cyclesPerLine {
			m.videolinecycle -= cyclesPerLine
			m.videolinenumber++
			if m.videolinenumber >= linesPerFrame {
				m.videolinenumber = 0
				// IRQ de fin de trame (50 Hz) — non masquable par le code ROM
				m.cpu.IRQ()
			}
		}

		if consumed >= cycles {
			break
		}
	}
	return consumed
}

// maxAudioBacklog borne le tampon d'échantillons à ~0,5 s, indépendamment du
// taux : si l'application ne draine pas (fenêtre inactive, pause), on évite une
// croissance mémoire illimitée en abandonnant les échantillons les plus anciens.
func (m *Machine) maxAudioBacklog() int { return m.audioSampleRate / 2 }

// appendSample ajoute un échantillon au tampon audio en respectant le plafond.
func (m *Machine) appendSample(level uint8) {
	if len(m.samples) >= m.maxAudioBacklog() {
		// Tampon saturé : on jette le plus ancien (glissement) pour rester borné.
		copy(m.samples, m.samples[1:])
		m.samples[len(m.samples)-1] = level
		return
	}
	m.samples = append(m.samples, level)
}

// DrainAudio copie les échantillons disponibles dans dst et vide le tampon
// interne. Retourne le nombre d'échantillons écrits (≤ len(dst)). Les niveaux
// sont sur 6 bits (0..spec.AudioLevelMax) ; la conversion en PCM est à la charge
// de la couche audio. Conçu pour être appelé une fois par frame par l'app.
func (m *Machine) DrainAudio(dst []uint8) int {
	n := copy(dst, m.samples)
	if n >= len(m.samples) {
		m.samples = m.samples[:0]
	} else {
		// Conserver le reliquat non copié au début du tampon.
		rest := copy(m.samples, m.samples[n:])
		m.samples = m.samples[:rest]
	}
	return n
}

// AudioBacklog retourne le nombre d'échantillons en attente (observabilité).
func (m *Machine) AudioBacklog() int { return len(m.samples) }

// AudioSampleRate retourne le taux d'échantillonnage audio effectif.
func (m *Machine) AudioSampleRate() int { return m.audioSampleRate }

// SetKey met à jour l'état d'une touche MO5.
func (m *Machine) SetKey(key Key, pressed bool) {
	if int(key) >= 0 && int(key) < len(m.touche) {
		if pressed {
			m.touche[key] = 0x00
		} else {
			m.touche[key] = 0x80
		}
	}
}

// SetJoystick met à jour l'état des manettes.
func (m *Machine) SetJoystick(input JoystickInput) {
	m.joysPosition = input.Position
	m.joysAction = input.Action
}

// SetPen met à jour la position et l'état du crayon optique.
func (m *Machine) SetPen(x, y int, pressed bool) {
	m.xpen = x
	m.ypen = y
	m.penbutton = pressed
}

// CPUSnapshot retourne une copie de l'état courant du CPU (registres, cycles).
// Utile pour l'observabilité (tests, futur affichage d'état machine).
func (m *Machine) CPUSnapshot() cpu6809.Snapshot {
	return m.cpu.Snapshot()
}

// PhysicalRAMChecksum retourne le hash FNV-32 de la RAM physique complète
// (les deux pages vidéo + RAM user), indépendamment de la page active.
// Utilisé par la fidelity suite pour détecter les régressions sur toute la RAM.
func (m *Machine) PhysicalRAMChecksum() uint32 {
	const fnvOffset32 = 2166136261
	const fnvPrime32 = 16777619
	h := uint32(fnvOffset32)
	for _, b := range m.ram {
		h ^= uint32(b)
		h *= fnvPrime32
	}
	return h
}
