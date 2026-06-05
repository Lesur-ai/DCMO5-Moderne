// Package impl fournit les implémentations fichiers des interfaces media.
package impl

import (
	"errors"
	"fmt"
	"io"
	"os"
)

// FileTape implémente media.Tape sur un fichier .k7.
type FileTape struct {
	f        *os.File
	readOnly bool
	pos      int64
	size     int64
}

// OpenTape ouvre un fichier cassette .k7. readOnly=true → écriture interdite.
func OpenTape(path string, readOnly bool) (*FileTape, error) {
	flag := os.O_RDWR
	if readOnly {
		flag = os.O_RDONLY
	}
	f, err := os.OpenFile(path, flag, 0)
	if err != nil {
		return nil, fmt.Errorf("tape: %w", err)
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("tape: stat: %w", err)
	}
	return &FileTape{f: f, readOnly: readOnly, size: info.Size()}, nil
}

// NewTape crée une cassette vierge dans le fichier path.
func NewTape(path string) (*FileTape, error) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return nil, fmt.Errorf("tape: create: %w", err)
	}
	return &FileTape{f: f, readOnly: false}, nil
}

// Close ferme le fichier cassette.
func (t *FileTape) Close() error { return t.f.Close() }

func (t *FileTape) ReadByte() (byte, error) {
	var buf [1]byte
	_, err := t.f.ReadAt(buf[:], t.pos)
	if errors.Is(err, io.EOF) {
		return 0, io.EOF
	}
	if err != nil {
		return 0, err
	}
	t.pos++
	return buf[0], nil
}

func (t *FileTape) WriteByte(b byte) error {
	if t.readOnly {
		return fmt.Errorf("tape: lecture seule")
	}
	_, err := t.f.WriteAt([]byte{b}, t.pos)
	if err != nil {
		return err
	}
	t.pos++
	if t.pos > t.size {
		t.size = t.pos
	}
	return nil
}

func (t *FileTape) Rewind() error {
	t.pos = 0
	return nil
}

func (t *FileTape) Position() int64 { return t.pos }
