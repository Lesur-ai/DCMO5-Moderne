// Package media définit les interfaces des périphériques de stockage MO5.
// Les implémentations fichiers sont dans ce package ; le cœur les reçoit
// déjà construites par la couche application.
// Ce package ne connaît pas les chemins OS.
package media

import "errors"

// ErrWriteProtected est retournée par une opération d'écriture sur un support en
// lecture seule / protégé. Le cœur la distingue d'une erreur d'E/S pour produire
// le bon code d'erreur MO5 (72 = protection, vs 53 = E/S). Ref C: Diskerror(72).
var ErrWriteProtected = errors.New("media: support protégé en écriture")

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
