package core_test

// Tests du trap crayon optique (0x4B → readPenXY) : on vérifie la chaîne
// complète conversion curseur → écran MO5 → coordonnées écrites dans la pile,
// ainsi que la sémantique du carry (détection / hors-zone).

import (
	"testing"

	"github.com/Lesur-ai/dcmo5/internal/core"
)

// penTrapROM fabrique une ROM 16 Ko qui place S puis exécute le trap crayon :
//
//	LDS #0x6000   (10 CE 60 00)
//	0x4B          (opcode illégal → readPenXY)
//	NOP…          (12)
//
// Le trap écrit xpen en S+6/S+7 et ypen en S+8/S+9 (big-endian), et positionne
// le carry (CC bit0) : 0 = détection (succès), 1 = hors zone (erreur).
func penTrapROM() []byte {
	rom := make([]byte, 0x4000)
	rom[0x0000] = 0x10
	rom[0x0001] = 0xCE
	rom[0x0002] = 0x60
	rom[0x0003] = 0x00 // LDS #0x6000
	rom[0x0004] = 0x4B // trap crayon
	for i := 5; i < len(rom)-2; i++ {
		rom[i] = 0x12 // NOP
	}
	rom[0x3FFE] = 0xC0
	rom[0x3FFF] = 0x00 // vecteur reset → 0xC000
	return rom
}

func runPenTrap(t *testing.T, cursorX, cursorY int, pressed bool) (*core.Machine, int, int) {
	t.Helper()
	m, err := core.NewMachine(core.Options{ROMSys: penTrapROM()})
	if err != nil {
		t.Fatalf("NewMachine: %v", err)
	}
	m.Reset()
	// Conversion identique à celle de la couche app (repère framebuffer → MO5).
	px, py := core.PenFromFramebuffer(cursorX, cursorY)
	m.SetPen(px, py, pressed)
	m.Step(100) // LDS + trap + NOPs
	return m, px, py
}

func TestPenTrap_InZone_WritesCoordsAndClearsCarry(t *testing.T) {
	// Curseur au milieu de l'écran : framebuffer (8+160, 8+100) → MO5 (160,100).
	m, px, py := runPenTrap(t, core.BorderWidth+160, core.BorderWidth+100, true)
	if px != 160 || py != 100 {
		t.Fatalf("conversion: (%d,%d), want (160,100)", px, py)
	}
	gotX := uint16(m.Read8(0x6006))<<8 | uint16(m.Read8(0x6007))
	gotY := uint16(m.Read8(0x6008))<<8 | uint16(m.Read8(0x6009))
	if gotX != 160 || gotY != 100 {
		t.Errorf("coords pile = (%d,%d), want (160,100)", gotX, gotY)
	}
	if cc := m.CPUSnapshot().CC; cc&0x01 != 0 {
		t.Errorf("carry = %d, want 0 (détection en zone active)", cc&0x01)
	}
}

func TestPenTrap_CornersActiveZone(t *testing.T) {
	// Les quatre coins exacts de la zone active doivent être détectés (carry=0)
	// avec les coordonnées MO5 attendues.
	corners := []struct{ cx, cy, wantX, wantY int }{
		{core.BorderWidth, core.BorderWidth, 0, 0},
		{core.BorderWidth + core.ActiveWidth - 1, core.BorderWidth, core.ActiveWidth - 1, 0},
		{core.BorderWidth, core.BorderWidth + core.ActiveHeight - 1, 0, core.ActiveHeight - 1},
		{core.BorderWidth + core.ActiveWidth - 1, core.BorderWidth + core.ActiveHeight - 1,
			core.ActiveWidth - 1, core.ActiveHeight - 1},
	}
	for _, c := range corners {
		m, _, _ := runPenTrap(t, c.cx, c.cy, true)
		gotX := uint16(m.Read8(0x6006))<<8 | uint16(m.Read8(0x6007))
		gotY := uint16(m.Read8(0x6008))<<8 | uint16(m.Read8(0x6009))
		if int(gotX) != c.wantX || int(gotY) != c.wantY {
			t.Errorf("coin (%d,%d): coords=(%d,%d), want (%d,%d)", c.cx, c.cy, gotX, gotY, c.wantX, c.wantY)
		}
		if cc := m.CPUSnapshot().CC; cc&0x01 != 0 {
			t.Errorf("coin (%d,%d): carry=1, want 0 (zone active)", c.cx, c.cy)
		}
	}
}

func TestPenTrap_OutOfZone_SetsCarry(t *testing.T) {
	// Curseur dans la bordure ou hors écran → hors zone active → carry=1.
	outside := []struct{ cx, cy int }{
		{0, 0},                      // coin fenêtre (bordure)
		{core.BorderWidth - 1, 100}, // bordure gauche (x=7)
		{100, core.BorderWidth - 1}, // bordure haute (y=7)
		{core.BorderWidth + core.ActiveWidth, 100},  // juste après bord droit
		{100, core.BorderWidth + core.ActiveHeight}, // juste après bord bas
	}
	for _, c := range outside {
		m, _, _ := runPenTrap(t, c.cx, c.cy, true)
		if cc := m.CPUSnapshot().CC; cc&0x01 == 0 {
			t.Errorf("curseur (%d,%d) hors zone : carry=0, want 1 (pas de détection)", c.cx, c.cy)
		}
	}
}
