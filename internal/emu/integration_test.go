package emu

// integration_test.go — Inc J0 du support joystick : harness d'intégration CPU
// 6809 pour les tests bout-en-bout. Permet de charger un petit programme Thomson
// dans la ROM système d'une machine MO5 et de l'exécuter quelques cycles, puis
// d'observer la RAM ou les registres pour valider qu'un état hôte (joystick,
// clavier, …) se propage VRAIMENT jusqu'au CPU émulé.
//
// Sans ce harness, les tests joystick ultérieurs (J2a, J6) ne pourraient
// observer que les structures internes (`core.joysAction`, `gatearray.touche`)
// — ce qui contourne le chemin réel CPU → bus → port et accepte un test de
// complaisance.

import (
	"testing"

	"github.com/Lesur-ai/dcmo5/internal/core"
)

// testProgram représente un petit programme 6809 à exécuter dans le test :
// suite d'octets à placer en ROM système (mappée à 0xC000 sur MO5) à partir de
// l'offset romOffset, vecteur reset positionné automatiquement sur 0xC000+romOffset.
type testProgram struct {
	romOffset int    // décalage dans la ROM 16K (0x0000..0x3FFF)
	code      []byte // octets 6809 (opcodes + operands)
}

// loadMO5Program charge un programme dans une nouvelle machine MO5 (ROM 16K
// remplie de NOP 6809 (0x12) par défaut, vecteur reset → entry point du
// programme). Retourne la machine prête à `Step()`. Pratique pour valider en
// quelques instructions un effet de bord côté CPU (écriture en RAM, lecture
// d'un port, etc.).
//
// La RAM utilisateur MO5 est mappée à 0x2000-0x5FFF (16 Ko, cf. core/mo5hw.go).
// La ROM système est mappée à 0xC000-0xFFFF, et le vecteur reset CPU est lu en
// 0xFFFE/0xFFFF (= ROM 0x3FFE/0x3FFF en interne core, cf. host_test.go:21-35).
func loadMO5Program(t *testing.T, prog testProgram) *core.Machine {
	t.Helper()
	if prog.romOffset < 0 || prog.romOffset+len(prog.code) > 0x4000 {
		t.Fatalf("programme hors ROM 16K : offset=%#x len=%d", prog.romOffset, len(prog.code))
	}
	rom := make([]byte, 0x4000)
	for i := range rom {
		rom[i] = 0x12 // NOP 6809 (cf. host_test.go nopMachine)
	}
	copy(rom[prog.romOffset:], prog.code)
	// Vecteur reset = entry point du programme. Adresse CPU = 0xC000 + romOffset.
	entry := uint16(0xC000) + uint16(prog.romOffset)
	rom[0x3FFE] = byte(entry >> 8)
	rom[0x3FFF] = byte(entry & 0xFF)
	m, err := core.NewMachine(core.Options{ROMSys: rom})
	if err != nil {
		t.Fatalf("NewMachine: %v", err)
	}
	m.Reset()
	return m
}

// runMO5 fait avancer l'émulation de cycles cycles (mode synchrone, sans
// goroutine) et retourne l'octet à l'adresse readback. Utile pour observer
// l'état final RAM après quelques instructions.
func runMO5(t *testing.T, m *core.Machine, cycles int, readback uint16) uint8 {
	t.Helper()
	m.Step(cycles)
	return m.Read8(readback)
}

// TestHarness_LoadAndRun (J0.T1) : valide que le harness fonctionne avant
// d'être utilisé par les tests joystick. Programme minimal LDA #$42 / STA $2000
// / BRA * : charge 0x42 dans l'accumulateur A, l'écrit en RAM[$2000], puis
// boucle indéfiniment sur lui-même. Après 32 cycles (largement suffisant pour
// les 3 instructions ≈ 8-9 cycles), RAM[$2000] DOIT valoir 0x42 — sinon le
// chemin Reset → vecteur → CPU → bus → Write8 est cassé et tous les tests
// joystick à venir seront muets (toujours verts, ce qui est pire).
func TestHarness_LoadAndRun(t *testing.T) {
	// LDA #$42        : 86 42        (2 octets, charger 0x42 dans A)
	// STA $2000       : B7 20 00     (3 octets, mode étendu : écrire A en $2000)
	// BRA *           : 20 FE        (2 octets, branch relative offset -2 → boucle)
	prog := testProgram{
		romOffset: 0x0000,
		code:      []byte{0x86, 0x42, 0xB7, 0x20, 0x00, 0x20, 0xFE},
	}
	m := loadMO5Program(t, prog)
	got := runMO5(t, m, 32, 0x2000)
	if got != 0x42 {
		t.Fatalf("RAM[$2000] = %#x après LDA #$42 / STA $2000 / BRA *, want 0x42 — harness CPU 6809 cassé", got)
	}
}
