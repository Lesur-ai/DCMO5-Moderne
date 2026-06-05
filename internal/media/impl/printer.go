package impl

import (
	"fmt"
	"io"
	"os"
)

// FilePrinter implémente media.PrinterSink vers un fichier (mode append).
type FilePrinter struct {
	f *os.File
}

// OpenPrinter ouvre (ou crée) un fichier imprimante en mode append.
func OpenPrinter(path string) (*FilePrinter, error) {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
	if err != nil {
		return nil, fmt.Errorf("printer: %w", err)
	}
	return &FilePrinter{f: f}, nil
}

// NewWriterPrinter crée un PrinterSink qui écrit dans un io.Writer quelconque.
// Utile pour les tests (bytes.Buffer) ou stdout.
func NewWriterPrinter(w io.Writer) *WriterPrinter { return &WriterPrinter{w: w} }

// WriterPrinter implémente media.PrinterSink sur un io.Writer.
type WriterPrinter struct{ w io.Writer }

func (p *WriterPrinter) WriteByte(b byte) error {
	_, err := p.w.Write([]byte{b})
	return err
}

// Close ferme le fichier imprimante.
func (p *FilePrinter) Close() error { return p.f.Close() }

func (p *FilePrinter) WriteByte(b byte) error {
	_, err := p.f.Write([]byte{b})
	return err
}
