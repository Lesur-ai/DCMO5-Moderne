package core_test

import (
	"fmt"
	"testing"
)

func TestROM_VideoRAM_Content(t *testing.T) {
	rom := loadROM(t)
	m, _ := newMachineWithROM(t, rom)
	m.Step(3_000_000)

	t.Log("=== RAM vidéo couleurs (8 premières lignes) ===")
	for line := 0; line < 8; line++ {
		row := make([]byte, 40)
		for col := 0; col < 40; col++ {
			row[col] = m.Read8(uint16(line*40 + col))
		}
		t.Logf("Ligne %d: %X", line, row)
	}

	t.Log("=== RAM vidéo formes (via page 1) ===")
	m.Write8(0xA7C0, 0x01)
	for line := 0; line < 4; line++ {
		row := make([]byte, 40)
		allZero := true
		for col := 0; col < 40; col++ {
			row[col] = m.Read8(uint16(line*40 + col))
			if row[col] != 0 {
				allZero = false
			}
		}
		t.Logf("Formes ligne %d: %X (allZero=%v)", line, row, allZero)
	}
	m.Write8(0xA7C0, 0x00)

	// Vérifier si la RAM user contient des données
	t.Log("=== RAM user (0x4000-0x40FF) ===")
	row := make([]byte, 40)
	for i := 0; i < 40; i++ {
		row[i] = m.Read8(uint16(0x4000 + i))
	}
	t.Logf("RAM user 0x4000: %X", row)

	// Chercher des patterns non-triviaux dans les 2 Ko de RAM vidéo couleurs
	nonZero := 0
	nonFF := 0
	for addr := uint16(0); addr < 0x2000; addr++ {
		b := m.Read8(addr)
		if b != 0 {
			nonZero++
		}
		if b != 0xFF {
			nonFF++
		}
	}
	t.Logf("RAM vidéo couleurs: %d octets non-zéro, %d octets non-FF sur 8192", nonZero, nonFF)
	_ = fmt.Sprintf // éviter import inutilisé
}
