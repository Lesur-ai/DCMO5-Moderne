// Fichier : io.go — traps d'E/S (Entreesortie), montage des médias et pointeur du
// gate-array. Réf : dcto8ddevices.c (Readsector/Writesector/Formatdisk,
// Readoctetk7/Writeoctetk7, Readpenxy, Readmousebutton, Imprime) et
// dcto8demulation.c Entreesortie(). Les paramètres disque/cassette sont en RAM
// (0x6049–0x6050 pour le disque TO8D, 0x2045 pour la cassette) ; les handlers
// lisent/écrivent aussi les registres CPU via le CPU attaché (AttachCPU).
package gatearray

import (
	"errors"

	"github.com/Lesur-ai/dcmo5/internal/engine"
	"github.com/Lesur-ai/dcmo5/internal/media"
)

// Le gate-array satisfait le contrat Device du moteur (mémoire, trap, timing,
// son, vidéo). Vérification à la compilation.
var _ engine.Device = (*GateArray)(nil)

// ── Montage des médias ────────────────────────────────────────────────────────

// MountTape insère une cassette et réamorce la lecture bit à bit.
func (g *GateArray) MountTape(t media.Tape) { g.tape = t; g.k7bit = 0; g.k7octet = 0 }

// EjectTape retire la cassette.
func (g *GateArray) EjectTape() { g.tape = nil; g.k7bit = 0; g.k7octet = 0 }

// MountDisk insère une disquette.
func (g *GateArray) MountDisk(d media.Disk) { g.disk = d }

// EjectDisk retire la disquette.
func (g *GateArray) EjectDisk() { g.disk = nil }

// MountPrinter / EjectPrinter branchent ou débranchent la sortie imprimante.
func (g *GateArray) MountPrinter(p media.PrinterSink) { g.printer = p }
func (g *GateArray) EjectPrinter()                    { g.printer = nil }

// MountCartridge insère une cartouche en reproduisant la sémantique de la réf C
// Loadmemo() (dcto8ddevices.c:219, chemin de chargement réussi) : RAZ partielle de
// la RAM + chargement cartouche + Initprog() — et NON un Hardreset(). Concrètement :
//   - resetRAM(0xc000) : réamorce SEULEMENT les premiers 0xc000 octets (réf C
//     Loadmemo : i < 0xc000) ; les banques RAM hautes (≥ 0xc000) sont préservées ;
//   - LoadCartridge()  : car[] + cartype + banc cartouche 0 ;
//   - softReset()      : Initprog() (recalcul banques + touches relâchées) suivi
//     de cpu.Reset() — équivalent fidèle d'Initprog()→Reset6809().
//
// Un Hardreset() serait infidèle : il réamorcerait TOUTE la RAM (banques hautes
// comprises), remettrait à zéro TOUS les ports et forcerait capslock=true (que
// Loadmemo ne touche pas), tout en n'effectuant PAS le reset CPU que Loadmemo
// réalise via Initprog. Le montage doux préserve donc les banques RAM hautes,
// l'état des ports et de capslock, et relance le CPU sur le vecteur reset.
func (g *GateArray) MountCartridge(c media.Cartridge) {
	var data []byte
	if c != nil {
		data = c.Bytes()
	}
	g.resetRAM(0xc000)
	g.LoadCartridge(data)
	g.softReset()
}

// EjectCartridge retire la cartouche et relance la machine sur la ROM système, en
// alignant la sémantique sur la réf C Loadmemo(name="") (action UI « [decharger] » :
// dcto8dinterface.c:873 → dcto8ddevices.c:219). Une éjection ne se contente PAS de
// recalculer le banc : elle RELANCE le CPU. Les deux variantes plateforme de la réf
// (Hardreset côté Windows quand fopen(répertoire) échoue ; chemin Initprog côté
// Linux/macOS où fopen réussit) relancent le 6809 via Reset6809. On reproduit cet
// invariant avec softReset() = initprog() + cpu.Reset(), exactement comme
// MountCartridge (#134) et comme le sibling cœur MO5 (internal/core/io.go).
//
// On reproduit l'INVARIANT de relance (recalcul des banques + reset CPU), pas les
// effets RAM platform-dependent de Loadmemo("") (wipe total côté Windows, réamorçage
// RAM basse côté Linux/macOS) : comme le sibling MO5, l'éjection ne touche pas à la
// RAM. Seule la relance manquait — c'est le périmètre du fix.
//
// On vide car[] AVANT le reset : contrairement au cœur MO5 (dont la lecture teste
// carflags bit2), le routage ROM du gate-array lit car[] selon e7c3 bit2 SANS tester
// le bit cart-enabled — sans vidage, l'ancienne cartouche resterait visible après
// éjection.
func (g *GateArray) EjectCartridge() {
	for i := range g.car {
		g.car[i] = 0
	}
	g.carflags = 0
	g.cartype = 0
	g.softReset() // réf C Loadmemo("") : Initprog()+Reset6809() — recalcul banques + reset CPU
}

// SetPointer met à jour le pointeur (crayon optique / souris). Les coordonnées
// sont dans le repère ÉCRAN TO8D (x ∈ [0,639], y ∈ [0,199]) ; la conversion
// depuis le repère framebuffer est faite par l'adaptateur machine (#118), comme
// pour le MO5.
func (g *GateArray) SetPointer(x, y int, button bool) {
	g.xpen = x
	g.ypen = y
	g.penbutton = button
}

// ── Traps d'E/S (Entreesortie) ────────────────────────────────────────────────

// Trap dispatche un appel d'E/S de la famille TO (opcode illégal, code = -opcode).
// Contrat engine.Device. Réf C : Entreesortie().
func (g *GateArray) Trap(code int) {
	switch code {
	case 0x14:
		g.readSector()
	case 0x15:
		g.writeSector()
	case 0x18:
		g.formatDisk()
	case 0x42:
		g.readOctetK7()
	case 0x45:
		g.writeOctetK7()
	case 0x4b:
		g.readPenXY(0) // crayon optique
	case 0x4e:
		g.readPenXY(1) // souris
	case 0x51:
		g.imprime()
	case 0x52:
		g.readMouseButton()
	}
}

// write16 écrit un mot big-endian sur le bus (réf C : Mputw).
func (g *GateArray) write16(a, v uint16) {
	g.Write8(a, byte(v>>8))
	g.Write8(a+1, byte(v))
}

// softReset reproduit l'effet d'Initprog() déclenché par une erreur média : reset
// doux des banques/entrées + rechargement du vecteur reset du CPU.
func (g *GateArray) softReset() {
	g.initprog()
	if g.cpu != nil {
		g.cpu.Reset()
	}
}

// ── Disquette .fd (params en 0x6049–0x6050, réf C Readsector/Writesector) ──────

// diskError écrit le code d'erreur en 0x604e (n-1) et positionne le carry.
func (g *GateArray) diskError(n int) {
	g.Write8(0x604e, byte(n-1))
	g.cpu.SetRegCC(g.cpu.RegCC() | 0x01)
}

// diskParams lit et valide (unité, piste, secteur) ; ok=false si déjà en erreur.
func (g *GateArray) diskParams() (unit, track, sector int, ok bool) {
	unit = int(g.Read8(0x6049))
	if unit > 3 {
		g.diskError(53)
		return
	}
	if g.Read8(0x604a) != 0 {
		g.diskError(53)
		return
	}
	track = int(g.Read8(0x604b))
	if track > 79 {
		g.diskError(53)
		return
	}
	sector = int(g.Read8(0x604c))
	if sector == 0 || sector > 16 {
		g.diskError(53)
		return
	}
	return unit, track, sector, true
}

func (g *GateArray) readSector() {
	if g.disk == nil {
		g.diskError(71)
		return
	}
	unit, track, sector, ok := g.diskParams()
	if !ok {
		return
	}
	buf, err := g.disk.ReadSector(unit, track, sector)
	if err != nil {
		g.diskError(53)
		return
	}
	dst := uint16(g.Read8(0x604f))<<8 | uint16(g.Read8(0x6050))
	for i, b := range buf {
		g.Write8(dst+uint16(i), b)
	}
}

func (g *GateArray) writeSector() {
	if g.disk == nil {
		g.diskError(71)
		return
	}
	unit, track, sector, ok := g.diskParams()
	if !ok {
		return
	}
	src := uint16(g.Read8(0x604f))<<8 | uint16(g.Read8(0x6050))
	var buf [256]byte
	for i := range buf {
		buf[i] = g.Read8(src + uint16(i))
	}
	if err := g.disk.WriteSector(unit, track, sector, buf); err != nil {
		g.diskError(diskErrCode(err))
	}
}

func (g *GateArray) formatDisk() {
	if g.disk == nil {
		g.diskError(71)
		return
	}
	unit := int(g.Read8(0x6049))
	if unit > 3 {
		return
	}
	if err := g.disk.FormatUnit(unit); err != nil {
		g.diskError(diskErrCode(err))
	}
}

// diskErrCode distingue la protection en écriture (72) de l'E/S générique (53).
func diskErrCode(err error) int {
	if errors.Is(err, media.ErrWriteProtected) {
		return 72
	}
	return 53
}

// ── Cassette .k7 (octet en 0x2045, réf C Readoctetk7/Writeoctetk7) ─────────────

func (g *GateArray) readOctetK7() {
	if g.tape == nil {
		g.softReset() // réf C : Initprog() + Erreur(11)
		return
	}
	b, err := g.tape.ReadByte()
	if err != nil {
		g.tape.Rewind()
		g.softReset() // réf C : Initprog() + Erreur(12) (fin de bande)
		return
	}
	g.k7octet = b
	g.k7bit = 0
	g.cpu.SetRegA(b)
	g.Write8(0x2045, b)
}

func (g *GateArray) writeOctetK7() {
	if g.tape == nil {
		g.softReset() // réf C : Initprog() + Erreur(11)
		return
	}
	if err := g.tape.WriteByte(g.cpu.RegA()); err != nil {
		g.softReset() // réf C : Initprog() + Erreur(13)
		return
	}
	g.Write8(0x2045, 0)
}

// ── Crayon optique / souris (réf C Readpenxy, Readmousebutton) ─────────────────

func (g *GateArray) readPenXY(device int) {
	// Hors zone active (x ∈ [0,639], y ∈ [0,199]) → carry = pas de détection.
	if g.xpen < 0 || g.xpen >= 640 || g.ypen < 0 || g.ypen >= 200 {
		g.cpu.SetRegCC(g.cpu.RegCC() | 0x01)
		return
	}
	// Mode 80 colonnes (e7dc == 0x2a) : pleine résolution X ; sinon X divisé par 2.
	k := uint(1)
	if g.port[0x1c] == 0x2a {
		k = 0
	}
	x := uint16(g.xpen >> k)
	if device > 0 { // souris : coordonnées aussi écrites en RAM moniteur
		g.write16(0x60d8, x)
		g.write16(0x60d6, uint16(g.ypen))
	}
	g.cpu.SetRegX(x)
	g.cpu.SetRegY(uint16(g.ypen))
	g.cpu.SetRegCC(g.cpu.RegCC() & 0xfe) // succès
}

func (g *GateArray) readMouseButton() {
	// A = 3 par défaut ; bouton pressé → A = 0 et carry/bits positionnés (réf C).
	if g.penbutton {
		g.cpu.SetRegA(0)
		g.cpu.SetRegCC(g.cpu.RegCC() | 0x05)
	} else {
		g.cpu.SetRegA(3)
	}
}

// ── Imprimante (réf C Imprime) ─────────────────────────────────────────────────

func (g *GateArray) imprime() {
	if g.printer == nil {
		return
	}
	_ = g.printer.WriteByte(g.cpu.RegB())
	g.cpu.SetRegCC(g.cpu.RegCC() & 0xfe)
}
