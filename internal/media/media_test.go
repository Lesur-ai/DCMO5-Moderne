package media_test

import (
	"testing"

	"github.com/Lesur-ai/dcmo5/internal/media"
)

// Les tests vérifient que les interfaces sont définissables et satisfiables.
// Les implémentations concrètes seront testées en P5.

type stubTape struct{}

func (s *stubTape) ReadByte() (byte, error) { return 0, nil }
func (s *stubTape) WriteByte(byte) error    { return nil }
func (s *stubTape) Rewind() error           { return nil }
func (s *stubTape) Position() int64         { return 0 }

type stubDisk struct{}

func (s *stubDisk) ReadSector(_, _, _ int) ([256]byte, error)  { return [256]byte{}, nil }
func (s *stubDisk) WriteSector(_, _, _ int, _ [256]byte) error { return nil }
func (s *stubDisk) FormatUnit(_ int) error                     { return nil }

type stubCartridge struct{}

func (s *stubCartridge) Bytes() []byte { return nil }

type stubPrinter struct{}

func (s *stubPrinter) WriteByte(_ byte) error { return nil }

func TestInterfacesSatisfied(t *testing.T) {
	var _ media.Tape = (*stubTape)(nil)
	var _ media.Disk = (*stubDisk)(nil)
	var _ media.Cartridge = (*stubCartridge)(nil)
	var _ media.PrinterSink = (*stubPrinter)(nil)
}
