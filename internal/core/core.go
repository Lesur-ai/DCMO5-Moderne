// Package core représente la machine Thomson MO5 complète.
// Il ne dépend d'aucune bibliothèque graphique, audio ni de chemins fichiers.
package core

import (
	"fmt"

	"github.com/Lesur-ai/dcmo5/internal/cpu6809"
	"github.com/Lesur-ai/dcmo5/internal/engine"
	"github.com/Lesur-ai/dcmo5/internal/machine"
	"github.com/Lesur-ai/dcmo5/internal/media"
	"github.com/Lesur-ai/dcmo5/internal/spec"
)

// Key identifie une touche du clavier MO5 (index dans [0, KeyCount)).
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

	// PatchSystemROM aligne en mémoire les routines d'E/S des vraies ROM MO5 sur
	// le modèle trap de l'émulateur, comme les ROM patchées de dcmo5 v11
	// (cf. rompatch.go) : ROM système (cassette, crayon, imprimante) ET, si
	// présente, ROM contrôleur de disquette CD90-640 (lire/écrire/formater +
	// amorçage DOS). Sans ce patch, ces E/S bouclent sur du bit-bang matériel non
	// émulé. Les fichiers ROM ne sont jamais modifiés. Sans effet sur une ROM non
	// reconnue.
	PatchSystemROM bool

	// OnError notifie la couche hôte d'une erreur d'E/S MO5 (codes BASIC : 11 =
	// cassette absente, 12 = fin de bande/EOF, 13 = protection/écriture).
	// Équivaut à la boîte de dialogue Erreur(n) de la réf C (dcmo5main.c).
	// nil = aucune notification. Le cœur reste sans dépendance UI.
	OnError func(code int)

	// DiskControllerROM est la ROM du contrôleur de disquette CD90-640, mappée en
	// lecture sur 0xA000..0xA7BF (le code DOS appelé par BASIC). Sans elle, cette
	// zone renvoie 0 et la disquette est inexploitable. ≤ 0x7C0 octets ; nil =
	// pas de contrôleur. Ref C: dcmo5emulation.c MgetMO5() — cd90640rom[a&0x7ff].
	DiskControllerROM []byte
}

// Machine représente le Thomson MO5 complet.
type Machine struct {
	eng  *engine.Engine // boucle d'émulation partagée (CPU, audio, trame, IRQ)
	cpu  *cpu6809.CPU   // = eng.CPU() ; raccourci pour les handlers d'E/S (io.go)
	opts Options

	// Mémoire physique
	ram  [RAMTotalSize]uint8 // 48 Ko RAM (vidéo + utilisateur)
	rom  [0x4000]uint8       // 16 Ko ROM système (0xC000–0xFFFF)
	car  [0x10000]uint8      // 4 banques × 16 Ko cartouche
	port [PortSize]uint8     // 64 octets ports E/S

	// État mémoire banked
	cartype  int   // 0=simple 1=MEMO5 switch 2=OS-9
	carflags uint8 // bits0-1=banque bits2=cart-active bit3=write-en bits4=OS9bank

	// Entrées
	touche       [KeyCount]uint8 // 0x00=pressée 0x80=relâchée
	joysPosition uint8           // axes manettes
	joysAction   uint8           // boutons d'action
	xpen, ypen   int
	penbutton    bool

	// Lecteur cassette bit-level (ref: dcmo5devices.c k7bit/k7octet)
	k7bit   uint8 // masque du bit en cours (0x80→0x01) ; 0 = recharger un octet
	k7octet uint8 // octet cassette courant en cours de lecture bit à bit

	// Son (ref: dcmo5main.c). sound = niveau courant du haut-parleur (0..0x3F),
	// mis à jour par les ports 0xA7C1/0xA7CD. Exposé au moteur via SoundLevel(),
	// qui l'échantillonne à audioSampleRate. Le tampon d'échantillons et le
	// balayage vidéo (trame 50 Hz) sont détenus par le moteur (internal/engine).
	sound           uint8 // niveau sonore courant (6 bits)
	audioSampleRate int   // taux d'échantillonnage effectif (transmis au moteur)

	// Instrumentation E/S optionnelle (nil = désactivée, coût nul). Voir iotrace.go.
	ioTrace *ioTrace

	// ROM contrôleur de disquette CD90-640 (0xA000..0xA7BF). diskRomLen == 0 si
	// aucun contrôleur monté. Voir Options.DiskControllerROM.
	diskRom    [0x800]uint8
	diskRomLen int
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
		if opts.PatchSystemROM {
			m.applySystemRomPatches() // alignement trap en mémoire (cf. rompatch.go)
		}
	}
	if n := len(opts.DiskControllerROM); n > 0 {
		if n > len(m.diskRom) {
			n = len(m.diskRom)
		}
		copy(m.diskRom[:n], opts.DiskControllerROM[:n])
		m.diskRomLen = n
		if opts.PatchSystemROM {
			m.applyDiskControllerPatches() // alignement trap du DOS (cf. rompatch.go)
		}
	}
	// Le moteur partagé crée le CPU sur le bus de cette machine. Comme
	// cpu6809.New (et la v1 du cœur), il NE réinitialise PAS le CPU : le vecteur
	// reset n'est chargé que par Reset()/Initprog(), après loadCartridge — ordre
	// d'amorçage préservé à l'identique.
	m.eng = engine.New(m, m.audioSampleRate)
	m.cpu = m.eng.CPU()
	m.hardReset()
	m.loadCartridge() // charge opts.Cartridge dans car[] si présente
	return m, nil
}

// resetRAM réamorce la RAM au motif d'init 0x00/0xFF (alternance selon le bit 7 de
// l'index). Motif partagé par Hardreset() et Loadmemo() de la réf C. Extrait pour
// que MountCartridge puisse réamorcer la RAM SANS passer par hardReset (qui efface
// aussi ports/trame/crayon). Pas de paramètre d'étendue : la RAM MO5 fait exactement
// RAMTotalSize (0xC000) = l'étendue Loadmemo (« i < 0xc000 »), contrairement au TO8D
// dont la RAM de 512 Ko impose resetRAM(0xc000) (cf. #134).
func (m *Machine) resetRAM() {
	for i := range m.ram {
		if i&0x80 != 0 {
			m.ram[i] = 0xFF
		} else {
			m.ram[i] = 0x00
		}
	}
}

// hardReset initialise la RAM, les ports et l'état interne.
// Ref: dcmo5emulation.c Hardreset()
func (m *Machine) hardReset() {
	m.resetRAM()
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
	// Son coupé ; le tampon d'échantillons et le balayage vidéo (trame 50 Hz)
	// appartiennent au moteur : on les réamorce via ResetTiming(), qui ne touche
	// PAS le CPU (le vecteur reset reste piloté par Reset()/Initprog()). Sans ce
	// flush, DrainAudio rejouerait du son d'avant le reset.
	m.sound = 0
	m.eng.ResetTiming()
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
			// ROM du contrôleur de disquette CD90-640 (0xA000..0xA7BF).
			// Ref C: dcmo5emulation.c — cd90640rom[a & 0x7ff].
			if m.diskRomLen > 0 {
				return m.diskRom[addr&0x7FF]
			}
			return 0
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
		m.sound = v & AudioLevelMax // registre niveau musique/son (6 bits)
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

// Reset réinitialise la machine (reset matériel : efface la RAM).
func (m *Machine) Reset() {
	m.hardReset()
	m.loadCartridge()
	m.cpu.Reset()
}

// Initprog relance le programme (reset « doux ») SANS effacer la RAM : touches
// relâchées, manettes au centre, banque cartouche remise à 0, son coupé, puis
// rechargement du vecteur reset. Ref: dcmo5emulation.c Initprog().
func (m *Machine) Initprog() {
	for i := range m.touche {
		m.touche[i] = 0x80
	}
	m.joysPosition = 0xFF
	m.joysAction = 0xC0
	m.carflags &= 0xEC // efface bits 0,1 (banque) et 4 (OS-9), garde cart-enabled
	m.sound = 0
	m.eng.ResetAudio() // coupe le son sans réamorcer la trame en cours (reset doux)
	m.k7bit = 0
	m.k7octet = 0
	m.cpu.Reset() // CC = 0x10, PC = vecteur reset
}

// Step avance l'émulation d'au plus cycles cycles et retourne les cycles
// consommés. Le MO5 délègue sa boucle au moteur partagé (internal/engine), qui
// exécute le CPU, échantillonne l'audio (via SoundLevel), cadence le balayage
// vidéo et délivre l'IRQ de fin de trame (50 Hz) — exactement la boucle
// dcmo5emulation.c Run() de la v1, désormais factorisée. Le dispatch d'E/S sur
// opcode illégal revient ici via Trap().
func (m *Machine) Step(cycles int) int { return m.eng.Step(cycles) }

// ── Contrat engine.Device : la partie MO5 pilotée par le moteur ───────────────

// Compile-time : le MO5 satisfait le contrat Device du moteur.
var _ engine.Device = (*Machine)(nil)

// Trap dispatche un appel d'E/S MO5 (opcode illégal, code = -opcode). Le moteur
// l'invoque quand cpu.Step() retourne un coût négatif.
func (m *Machine) Trap(code int) { m.entreesortie(code) }

// OnInstructionCycles : le MO5 n'a aucun périphérique à cadencer par instruction
// (sa seule source d'IRQ — la fin de trame — est gérée par le moteur). No-op,
// fidélité préservée. La famille TO (timer 6846, IRQ clavier) l'utilisera.
func (m *Machine) OnInstructionCycles(int, *machine.IRQLines) {}

// SoundLevel retourne le niveau courant du haut-parleur (0..0x3F) ; le moteur
// l'échantillonne à la fréquence audio.
func (m *Machine) SoundLevel() uint8 { return m.sound }

// FrameSize retourne la taille (fixe) du framebuffer logique MO5.
func (m *Machine) FrameSize() (w, h int) { return FrameWidth, FrameHeight }

// DecodeFrame rend le framebuffer courant dans dst (contrat Device → délègue au
// rendu MO5 de video.go).
func (m *Machine) DecodeFrame(dst []uint32) { m.FramebufferInto(dst) }

// DrainAudio copie les échantillons disponibles dans dst et vide le tampon du
// moteur. Retourne le nombre d'échantillons écrits (≤ len(dst)). Les niveaux
// sont sur 6 bits (0..AudioLevelMax) ; la conversion en PCM est à la charge
// de la couche audio. Conçu pour être appelé une fois par frame par l'app.
func (m *Machine) DrainAudio(dst []uint8) int { return m.eng.DrainAudio(dst) }

// AudioBacklog retourne le nombre d'échantillons en attente (observabilité).
func (m *Machine) AudioBacklog() int { return m.eng.AudioBacklog() }

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
