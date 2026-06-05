// Fichier : io.go — branchement médias et entrées/sorties MO5.
// Ref: dcmo5devices.c Entreesortie(), Readsector(), Readoctetk7(), etc.
package core

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

// ── Dispatch I/O (Entreesortie) ───────────────────────────────────────────────

// Entreesortie dispatche les appels I/O du CPU vers les périphériques montés.
// Appelé depuis Step() quand cpu.Step() retourne cycles < 0.
// Exporté pour les tests d'intégration.
// Ref: dcmo5emulation.c Entreesortie()
func (m *Machine) Entreesortie(io int) {
	m.entreesortie(io)
}

func (m *Machine) entreesortie(io int) {
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

// readOctetK7 lit un octet de la cassette dans A et 0x2045.
// Ref: dcmo5devices.c Readoctetk7()
func (m *Machine) readOctetK7() {
	if m.opts.Tape == nil {
		return
	}
	b, err := m.opts.Tape.ReadByte()
	if err != nil {
		// Fin de bande : on repart du début (comme Initprog dans la ref)
		m.opts.Tape.Rewind()
		return
	}
	// Écrire l'octet en A (via bus à 0x2045 = RAM user)
	m.Write8(0x2045, b)
}

// readBitK7 lit un bit de la cassette.
// Ref: dcmo5devices.c Readbitk7()
func (m *Machine) readBitK7() {
	// Implémentation simplifiée : délégue à readOctetK7 au niveau octet.
	// Le protocole bit-à-bit de la K7 MO5 sera affiné si nécessaire.
	m.readOctetK7()
}

// writeOctetK7 écrit l'octet de A sur la cassette.
// Ref: dcmo5devices.c Writeoctetk7()
func (m *Machine) writeOctetK7() {
	if m.opts.Tape == nil {
		return
	}
	// A est à l'adresse RAM user 0x4045 (CPU 0x2045 → phys 0x4045)
	a := m.Read8(0x2045)
	m.opts.Tape.WriteByte(a)
}

// ── Disquette .fd ─────────────────────────────────────────────────────────────

// readSector lit un secteur disque et le copie en RAM.
// Ref: dcmo5devices.c Readsector()
func (m *Machine) readSector() {
	if m.opts.Disk == nil {
		return
	}
	unit := int(m.Read8(0x2049))   // face (0-3)
	track := int(m.Read8(0x204B))  // piste (0-79)
	sector := int(m.Read8(0x204C)) // secteur (1-16, 1-based)
	destHi := m.Read8(0x204F)
	destLo := m.Read8(0x2050)
	dest := uint16(destHi)<<8 | uint16(destLo)

	if unit > 3 || track > 79 || sector == 0 || sector > 16 {
		return // erreur 53 : paramètres invalides
	}

	buf, err := m.opts.Disk.ReadSector(unit, track, sector)
	if err != nil {
		return
	}
	for i, b := range buf {
		m.Write8(dest+uint16(i), b)
	}
}

// writeSector écrit un secteur disque depuis la RAM.
// Ref: dcmo5devices.c Writesector()
func (m *Machine) writeSector() {
	if m.opts.Disk == nil {
		return
	}
	unit := int(m.Read8(0x2049))
	track := int(m.Read8(0x204B))
	sector := int(m.Read8(0x204C))
	srcHi := m.Read8(0x204F)
	srcLo := m.Read8(0x2050)
	src := uint16(srcHi)<<8 | uint16(srcLo)

	if unit > 3 || track > 79 || sector == 0 || sector > 16 {
		return
	}

	var buf [256]byte
	for i := range buf {
		buf[i] = m.Read8(src + uint16(i))
	}
	m.opts.Disk.WriteSector(unit, track, sector, buf)
}

// formatDisk formate une face disque.
// Ref: dcmo5devices.c Formatdisk()
func (m *Machine) formatDisk() {
	if m.opts.Disk == nil {
		return
	}
	unit := int(m.Read8(0x2049))
	if unit > 3 {
		return
	}
	m.opts.Disk.FormatUnit(unit)
}

// ── Crayon optique ────────────────────────────────────────────────────────────

// readPenXY écrit les coordonnées du crayon sur la pile MO5.
// Ref: dcmo5devices.c Readpenxy()
func (m *Machine) readPenXY() {
	if m.xpen < 0 || m.xpen >= 320 || m.ypen < 0 || m.ypen >= 200 {
		// Hors écran : positionner C dans CC (erreur)
		// On accède au CPU via le bus — simplification : on ne modifie pas CC ici
		return
	}
	// La ref C écrit xpen/ypen dans la pile via S+6 et S+8.
	// On ne peut pas accéder directement au registre S depuis le bus.
	// Implémentation minimale : écrire dans des adresses conventionnelles.
	m.Write8(0x2047, uint8(m.xpen>>8))
	m.Write8(0x2048, uint8(m.xpen))
	m.Write8(0x2049, uint8(m.ypen>>8))
	m.Write8(0x204A, uint8(m.ypen))
}

// ── Imprimante ────────────────────────────────────────────────────────────────

// imprime envoie l'octet de B à l'imprimante.
// Ref: dcmo5devices.c Imprime() — fputc(*Bp, fprn)
func (m *Machine) imprime() {
	if m.opts.Printer == nil {
		return
	}
	// B est accessible via le bus à l'adresse 0x2046 dans la ROM MO5.
	// Simplification : on lit depuis la page user RAM où le ROM stub place B.
	b := m.Read8(0x2046)
	m.opts.Printer.WriteByte(b)
}
