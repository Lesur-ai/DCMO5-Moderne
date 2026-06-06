package impl_test

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/Lesur-ai/dcmo5/internal/media/impl"
	"github.com/Lesur-ai/dcmo5/internal/spec"
)

// ── Tape ─────────────────────────────────────────────────────────────────────

func TestTape_WriteReadRewind(t *testing.T) {
	f := filepath.Join(t.TempDir(), "test.k7")
	tape, err := impl.NewTape(f)
	if err != nil {
		t.Fatalf("NewTape: %v", err)
	}
	defer tape.Close()

	data := []byte{0x55, 0xAA, 0x12}
	for _, b := range data {
		if err := tape.WriteByte(b); err != nil {
			t.Fatalf("WriteByte: %v", err)
		}
	}
	if pos := tape.Position(); pos != 3 {
		t.Errorf("Position après 3 octets = %d, want 3", pos)
	}
	if err := tape.Rewind(); err != nil {
		t.Fatalf("Rewind: %v", err)
	}
	if pos := tape.Position(); pos != 0 {
		t.Errorf("Position après Rewind = %d, want 0", pos)
	}
	for i, want := range data {
		got, err := tape.ReadByte()
		if err != nil {
			t.Fatalf("ReadByte[%d]: %v", i, err)
		}
		if got != want {
			t.Errorf("ReadByte[%d] = 0x%02X, want 0x%02X", i, got, want)
		}
	}
}

func TestTape_EOFAtEnd(t *testing.T) {
	f := filepath.Join(t.TempDir(), "empty.k7")
	tape, err := impl.NewTape(f)
	if err != nil {
		t.Fatalf("NewTape: %v", err)
	}
	defer tape.Close()
	_, err = tape.ReadByte()
	if err != io.EOF {
		t.Errorf("ReadByte sur cassette vide: got %v, want io.EOF", err)
	}
}

func TestTape_ReadOnly(t *testing.T) {
	f := filepath.Join(t.TempDir(), "ro.k7")
	// Créer d'abord avec NewTape
	tape, _ := impl.NewTape(f)
	tape.WriteByte(0x42)
	tape.Close()

	ro, err := impl.OpenTape(f, true)
	if err != nil {
		t.Fatalf("OpenTape readonly: %v", err)
	}
	defer ro.Close()
	if err := ro.WriteByte(0x00); err == nil {
		t.Error("WriteByte sur cassette lecture seule : erreur attendue")
	}
}

// ── Disk ──────────────────────────────────────────────────────────────────────

func TestDisk_WriteReadSector(t *testing.T) {
	f := filepath.Join(t.TempDir(), "test.fd")
	disk, err := impl.NewDisk(f)
	if err != nil {
		t.Fatalf("NewDisk: %v", err)
	}
	defer disk.Close()

	var sector [256]byte
	for i := range sector {
		sector[i] = uint8(i)
	}
	// Secteurs numérotés 1-based [1..spec.FDSectors], conforme au contrôleur MO5.
	if err := disk.WriteSector(0, 5, 3, sector); err != nil {
		t.Fatalf("WriteSector: %v", err)
	}
	got, err := disk.ReadSector(0, 5, 3)
	if err != nil {
		t.Fatalf("ReadSector: %v", err)
	}
	if got != sector {
		t.Error("ReadSector != WriteSector")
	}
}

func TestDisk_SizeValidation(t *testing.T) {
	f := filepath.Join(t.TempDir(), "bad.fd")
	os.WriteFile(f, []byte{0x00, 0x01}, 0o644)
	_, err := impl.OpenDisk(f, true)
	if err == nil {
		t.Error("OpenDisk fichier mauvaise taille : erreur attendue")
	}
}

func TestDisk_OutOfBounds(t *testing.T) {
	f := filepath.Join(t.TempDir(), "test.fd")
	disk, _ := impl.NewDisk(f)
	defer disk.Close()
	if _, err := disk.ReadSector(0, 999, 1); err == nil {
		t.Error("ReadSector piste 999 : erreur attendue")
	}
	if _, err := disk.ReadSector(5, 0, 1); err == nil {
		t.Error("ReadSector face 5 : erreur attendue")
	}
	// Secteur 0 invalide (1-based)
	if _, err := disk.ReadSector(0, 0, 0); err == nil {
		t.Error("ReadSector secteur 0 : erreur attendue (1-based)")
	}
	// Secteur 17 invalide (max = spec.FDSectors = 16)
	if _, err := disk.ReadSector(0, 0, 17); err == nil {
		t.Error("ReadSector secteur 17 : erreur attendue")
	}
}

func TestDisk_FormatUnit(t *testing.T) {
	f := filepath.Join(t.TempDir(), "test.fd")
	disk, _ := impl.NewDisk(f)
	defer disk.Close()
	var full [256]byte
	for i := range full {
		full[i] = 0x00
	}
	disk.WriteSector(0, 0, 1, full) // secteur 1 (1-based)
	if err := disk.FormatUnit(0); err != nil {
		t.Fatalf("FormatUnit: %v", err)
	}
	// La réf C remplit le disque formaté avec 0xE5 (motif « vierge »), pas 0x00.
	var wantE5 [256]byte
	for i := range wantE5 {
		wantE5[i] = 0xE5
	}
	got, _ := disk.ReadSector(0, 0, 1)
	if got != wantE5 {
		t.Errorf("FormatUnit : secteur devrait être rempli de 0xE5, got [0]=0x%02X", got[0])
	}
}

func TestDisk_NewDiskSize(t *testing.T) {
	f := filepath.Join(t.TempDir(), "test.fd")
	disk, err := impl.NewDisk(f)
	if err != nil {
		t.Fatalf("NewDisk: %v", err)
	}
	disk.Close()
	info, _ := os.Stat(f)
	if info.Size() != int64(spec.FDDiskSize) {
		t.Errorf("taille .fd = %d, want %d", info.Size(), spec.FDDiskSize)
	}
}

// ── Cartridge ─────────────────────────────────────────────────────────────────

func TestCartridge_ValidSize(t *testing.T) {
	f := filepath.Join(t.TempDir(), "test.rom")
	data := make([]byte, 0x4000) // 16 Ko
	data[0] = 0xAB
	os.WriteFile(f, data, 0o644)
	cart, err := impl.OpenCartridge(f)
	if err != nil {
		t.Fatalf("OpenCartridge: %v", err)
	}
	if b := cart.Bytes(); len(b) != 0x4000 || b[0] != 0xAB {
		t.Error("Bytes() incorrect")
	}
}

func TestCartridge_InvalidSize(t *testing.T) {
	f := filepath.Join(t.TempDir(), "bad.rom")
	os.WriteFile(f, make([]byte, 100), 0o644)
	if _, err := impl.OpenCartridge(f); err == nil {
		t.Error("OpenCartridge taille invalide : erreur attendue")
	}
}

// ── PrinterSink ───────────────────────────────────────────────────────────────

func TestWriterPrinter(t *testing.T) {
	var buf bytes.Buffer
	p := impl.NewWriterPrinter(&buf)
	data := []byte("Hello MO5\n")
	for _, b := range data {
		if err := p.WriteByte(b); err != nil {
			t.Fatalf("WriteByte: %v", err)
		}
	}
	if got := buf.String(); got != "Hello MO5\n" {
		t.Errorf("PrinterSink output = %q, want %q", got, "Hello MO5\n")
	}
}

func TestFilePrinter_Append(t *testing.T) {
	f := filepath.Join(t.TempDir(), "printer.txt")
	p, err := impl.OpenPrinter(f)
	if err != nil {
		t.Fatalf("OpenPrinter: %v", err)
	}
	p.WriteByte('A')
	p.WriteByte('B')
	p.Close()
	// Rouvrir et ajouter
	p2, _ := impl.OpenPrinter(f)
	p2.WriteByte('C')
	p2.Close()
	got, _ := os.ReadFile(f)
	if string(got) != "ABC" {
		t.Errorf("FilePrinter append: got %q, want %q", got, "ABC")
	}
}
