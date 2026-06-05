// Package media définit les interfaces des périphériques de stockage MO5.
// Les implémentations fichiers sont dans ce package ; le cœur les reçoit
// déjà construites par la couche application.
// Ce package ne connaît pas les chemins OS.
package media

// Tape représente une cassette .k7.
type Tape interface {
	ReadByte() (byte, error)
	WriteByte(byte) error
	Rewind() error
	Position() int64
}

// Disk représente une disquette .fd (CD90-640).
type Disk interface {
	ReadSector(unit, track, sector int) ([256]byte, error)
	WriteSector(unit, track, sector int, data [256]byte) error
	FormatUnit(unit int) error
}

// Cartridge représente une cartouche MEMO5 .rom.
type Cartridge interface {
	Bytes() []byte
}

// PrinterSink reçoit les octets envoyés à l'imprimante parallèle.
type PrinterSink interface {
	WriteByte(byte) error
}
