// Fichier : io.go — branchement médias et entrées/sorties MO5.
// Ref: dcmo5devices.c Entreesortie(), Readsector(), Readoctetk7(), etc.
package core

import (
	"errors"

	"github.com/Lesur-ai/dcmo5/internal/media"
)

// ── Initialisation cartouche ──────────────────────────────────────────────────

// loadCartridge charge opts.Cartridge dans car[] et configure cartype/carflags.
// Ref: dcmo5devices.c Loadmemo()
func (m *Machine) loadCartridge() {
	if m.opts.Cartridge == nil {
		return
	}
	data := m.opts.Cartridge.Bytes()
	if len(data) == 0 {
		return
	}
	// Repartir d'un espace cartouche vierge : un remontage ne doit pas laisser
	// de résidus d'une cartouche précédente plus grande.
	for i := range m.car {
		m.car[i] = 0
	}

	// Copier dans car[] (max 64 Ko)
	n := len(data)
	if n > len(m.car) {
		n = len(m.car)
	}
	copy(m.car[:n], data[:n])

	// cartype : 0 = simple (≤ 16 Ko), 1 = MEMO5 bank switch (> 16 Ko)
	m.cartype = 0
	if len(data) > 0x4000 {
		m.cartype = 1
	}
	m.carflags = 4 // cart active, write disabled, banque 0
}

// ── Montage / éjection des médias à chaud ─────────────────────────────────────
//
// Ces méthodes permettent de changer de support après la création de la machine
// (cassette, disquette, cartouche), socle du menu de pilotage. La fermeture des
// fichiers sous-jacents reste à la charge de l'appelant (couche application) :
// le cœur ne connaît que les interfaces media, pas les fichiers OS.

// MountTape insère une cassette et réamorce l'état de lecture bit-level.
func (m *Machine) MountTape(t media.Tape) {
	m.opts.Tape = t
	m.k7bit = 0
	m.k7octet = 0
}

// EjectTape retire la cassette courante.
func (m *Machine) EjectTape() {
	m.opts.Tape = nil
	m.k7bit = 0
	m.k7octet = 0
}

// MountDisk insère une disquette.
func (m *Machine) MountDisk(d media.Disk) {
	m.opts.Disk = d
}

// EjectDisk retire la disquette courante.
func (m *Machine) EjectDisk() {
	m.opts.Disk = nil
}

// MountCartridge insère une cartouche en reproduisant la sémantique de la réf C
// Loadmemo() (dcmo5devices.c:221, chemin de chargement réussi) : RAZ RAM +
// chargement cartouche + Initprog() — et NON un Hardreset(). Concrètement :
//   - resetRAM()      : réamorce la RAM (motif 0x00/0xFF) ;
//   - loadCartridge() : car[] + cartype + carflags=4 (banc cartouche actif) ;
//   - Initprog()      : reset doux (touches/manettes/banque/son) suivi de cpu.Reset()
//     — équivalent fidèle d'Initprog()→Reset6809() de la réf C.
//
// Passer par Reset()/hardReset serait infidèle : hardReset remet TOUS les ports
// d'E/S à 0, réamorce le balayage vidéo (ResetTiming) et efface l'état du crayon —
// que Loadmemo PRÉSERVE. C'est le sibling MO5 du correctif gate-array TO8D #134
// (annoncé dans #132). La RAM, le routage cartouche et le reset CPU restent garantis.
func (m *Machine) MountCartridge(c media.Cartridge) {
	m.opts.Cartridge = c
	m.resetRAM()
	m.loadCartridge()
	m.Initprog()
}

// EjectCartridge retire la cartouche, désactive le banc cartouche et relance sur
// la ROM système. Ref C Loadmemo(name="") : carflags = 0 puis Initprog().
func (m *Machine) EjectCartridge() {
	m.opts.Cartridge = nil
	m.carflags = 0
	m.cartype = 0
	m.cpu.Reset()
}

// SetPrinter branche (p non nil) ou débranche (nil) la sortie imprimante à chaud.
// Le trap d'impression (0x51) écrit dans ce sink quand il est présent. Permet le
// montage/éjection imprimante via le contrat machine.Machine sans reconstruire la
// machine. Ref: dcmo5devices.c Imprime() — fputc(B, fprn).
func (m *Machine) SetPrinter(p media.PrinterSink) {
	m.opts.Printer = p
}

// ── Dispatch I/O (Entreesortie) ───────────────────────────────────────────────

// Entreesortie dispatche les appels I/O du CPU vers les périphériques montés.
// Appelé depuis Step() quand cpu.Step() retourne cycles < 0.
// Exporté pour les tests d'intégration.
// Ref: dcmo5emulation.c Entreesortie()
func (m *Machine) Entreesortie(io int) {
	m.entreesortie(io)
}

func (m *Machine) entreesortie(io int) {
	if m.ioTrace != nil {
		// Figer l'état d'entrée (params RAM, registres) avant le dispatch.
		m.ioTrace.record(io, m)
	}
	switch io {
	case 0x14:
		m.readSector()
	case 0x15:
		m.writeSector()
	case 0x18:
		m.formatDisk()
	case 0x41:
		m.readBitK7()
	case 0x42:
		m.readOctetK7()
	case 0x45:
		m.writeOctetK7()
	case 0x4B:
		m.readPenXY()
	case 0x51:
		m.imprime()
	}
}

// ── Cassette .k7 ─────────────────────────────────────────────────────────────

// readOctetK7 lit un octet de la cassette, le place dans A et 0x2045 et
// réamorce l'état bit-level. Sémantique d'erreur alignée sur la réf C :
//   - cassette absente : Initprog() + Erreur 11 ;
//   - fin de bande (EOF) : rembobinage + Initprog() + Erreur 12.
//
// Ref: dcmo5devices.c Readoctetk7() — *Ap = k7octet = byte; Mputc(0x2045, byte); k7bit = 0
func (m *Machine) readOctetK7() {
	if m.opts.Tape == nil {
		m.Initprog()
		m.signalError(11) // cassette absente
		return
	}
	b, err := m.opts.Tape.ReadByte()
	if err != nil {
		m.opts.Tape.Rewind()
		m.Initprog()
		m.signalError(12) // fin de bande / EOF
		return
	}
	m.k7octet = b
	m.k7bit = 0
	m.cpu.SetRegA(b)
	m.Write8(0x2045, b)
}

// signalError notifie la couche hôte d'une erreur d'E/S MO5 si un sink est
// configuré (équivalent de Erreur(n) côté réf C). Sans dépendance UI dans le cœur.
func (m *Machine) signalError(code int) {
	if m.opts.OnError != nil {
		m.opts.OnError(code)
	}
}

// IOErrorLabel donne un libellé court pour un code d'erreur d'E/S MO5
// (codes BASIC, cf. réf C). Sert aux notifications hôte.
func IOErrorLabel(code int) string {
	switch code {
	case 11:
		return "cassette absente"
	case 12:
		return "fin de bande"
	case 13:
		return "écriture protégée / échec"
	default:
		return "erreur E/S"
	}
}

// readBitK7 lit un bit de la cassette : A=0xFF si le bit vaut 1, 0x00 sinon, et
// décale l'octet courant dans 0x2045. Recharge un octet quand tous les bits
// sont consommés. Ref: dcmo5devices.c Readbitk7()
func (m *Machine) readBitK7() {
	if m.opts.Tape == nil {
		return
	}
	octet := int(m.Read8(0x2045)) << 1
	if m.k7bit == 0 {
		m.readOctetK7() // recharge m.k7octet ; remet k7bit à 0
		m.k7bit = 0x80
	}
	if m.k7octet&m.k7bit != 0 {
		octet |= 0x01
		m.cpu.SetRegA(0xFF)
	} else {
		m.cpu.SetRegA(0x00)
	}
	m.Write8(0x2045, uint8(octet))
	m.k7bit >>= 1
}

// writeOctetK7 écrit le registre A sur la cassette, puis remet 0x2045 à 0.
// Sémantique d'erreur alignée sur la réf C :
//   - cassette absente : Initprog() + Erreur 11 ;
//   - échec/protection en écriture : Initprog() + Erreur 13.
//
// Ref: dcmo5devices.c Writeoctetk7() — fputc(*Ap, fk7); Mputc(0x2045, 0)
func (m *Machine) writeOctetK7() {
	if m.opts.Tape == nil {
		m.Initprog()
		m.signalError(11) // cassette absente
		return
	}
	if err := m.opts.Tape.WriteByte(m.cpu.RegA()); err != nil {
		m.Initprog()
		m.signalError(13) // protection / échec d'écriture
		return
	}
	m.Write8(0x2045, 0)
}

// ── Disquette .fd ─────────────────────────────────────────────────────────────

// diskError reproduit Diskerror(n) de la réf C : écrit le code d'erreur en
// 0x204E (n-1) et positionne le carry. Codes : 53 = E/S (paramètres/secteur
// invalide), 71 = lecteur non prêt (pas de disquette), 72 = protection écriture.
// Ref: dcmo5devices.c Diskerror() — Mputc(0x204e, n-1) ; CC |= 0x01.
func (m *Machine) diskError(n int) {
	m.Write8(0x204E, uint8(n-1))
	m.cpu.SetRegCC(m.cpu.RegCC() | 0x01)
}

// readSector lit un secteur disque et le copie en RAM.
// Ref: dcmo5devices.c Readsector() — u(0x2049)≤3, 0x204a==0, p(0x204b)≤79,
// s(0x204c)∈[1,16] ; sinon Diskerror(53). Pas de disquette → Diskerror(71).
func (m *Machine) readSector() {
	if m.opts.Disk == nil {
		m.diskError(71)
		return
	}
	unit := int(m.Read8(0x2049))
	if m.Read8(0x204A) != 0 || unit > 3 {
		m.diskError(53)
		return
	}
	track := int(m.Read8(0x204B))
	sector := int(m.Read8(0x204C))
	if track > 79 || sector == 0 || sector > 16 {
		m.diskError(53)
		return
	}
	dest := uint16(m.Read8(0x204F))<<8 | uint16(m.Read8(0x2050))

	buf, err := m.opts.Disk.ReadSector(unit, track, sector)
	if err != nil {
		m.diskError(53) // secteur hors capacité du fichier ou E/S
		return
	}
	for i, b := range buf {
		m.Write8(dest+uint16(i), b)
	}
}

// writeSector écrit un secteur disque depuis la RAM.
// Ref: dcmo5devices.c Writesector() — mêmes contrôles ; échec d'écriture
// (lecture seule / hors capacité) → Diskerror(72/53).
func (m *Machine) writeSector() {
	if m.opts.Disk == nil {
		m.diskError(71)
		return
	}
	unit := int(m.Read8(0x2049))
	if m.Read8(0x204A) != 0 || unit > 3 {
		m.diskError(53)
		return
	}
	track := int(m.Read8(0x204B))
	sector := int(m.Read8(0x204C))
	if track > 79 || sector == 0 || sector > 16 {
		m.diskError(53)
		return
	}
	src := uint16(m.Read8(0x204F))<<8 | uint16(m.Read8(0x2050))

	var buf [256]byte
	for i := range buf {
		buf[i] = m.Read8(src + uint16(i))
	}
	if err := m.opts.Disk.WriteSector(unit, track, sector, buf); err != nil {
		m.diskError(diskErrCodeFor(err))
	}
}

// diskErrCodeFor distingue la protection en écriture (72) d'une erreur d'E/S
// générique (53), conformément à la réf C. Ref: dcmo5devices.c Writesector().
func diskErrCodeFor(err error) int {
	if errors.Is(err, media.ErrWriteProtected) {
		return 72
	}
	return 53
}

// formatDisk formate une unité disque.
// Ref: dcmo5devices.c Formatdisk() — pas de disquette → Diskerror(71) ;
// unité > 3 → retour silencieux (comme la réf C).
func (m *Machine) formatDisk() {
	if m.opts.Disk == nil {
		m.diskError(71)
		return
	}
	unit := int(m.Read8(0x2049))
	if unit > 3 {
		return
	}
	if err := m.opts.Disk.FormatUnit(unit); err != nil {
		m.diskError(diskErrCodeFor(err))
	}
}

// ── Crayon optique ────────────────────────────────────────────────────────────

// readPenXY écrit les coordonnées du crayon dans la pile CPU (S+6, S+8).
// Ref: dcmo5devices.c Readpenxy() — Mputw(S+6, xpen); Mputw(S+8, ypen); CC &= 0xfe
func (m *Machine) readPenXY() {
	if m.xpen < 0 || m.xpen >= ActiveWidth || m.ypen < 0 || m.ypen >= ActiveHeight {
		m.cpu.SetRegCC(m.cpu.RegCC() | 0x01) // set carry = erreur (hors zone active)
		return
	}
	s := m.cpu.RegS()
	m.Write8(s+6, uint8(m.xpen>>8))
	m.Write8(s+7, uint8(m.xpen))
	m.Write8(s+8, uint8(m.ypen>>8))
	m.Write8(s+9, uint8(m.ypen))
	m.cpu.SetRegCC(m.cpu.RegCC() & 0xfe) // clear carry = succès
}

// ── Imprimante ────────────────────────────────────────────────────────────────

// imprime envoie le registre B à l'imprimante et efface le carry.
// Ref: dcmo5devices.c Imprime() — fputc(*Bp, fprn); CC &= 0xfe
func (m *Machine) imprime() {
	if m.opts.Printer == nil {
		return
	}
	m.opts.Printer.WriteByte(m.cpu.RegB())
	m.cpu.SetRegCC(m.cpu.RegCC() & 0xfe)
}
