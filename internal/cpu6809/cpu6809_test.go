package cpu6809_test

import (
	"testing"

	"github.com/Lesur-ai/dcmo5/internal/cpu6809"
)

// stubBus est un bus mémoire minimal pour les tests.
type stubBus struct {
	mem [0x10000]uint8
}

func (b *stubBus) Read8(addr uint16) uint8     { return b.mem[addr] }
func (b *stubBus) Write8(addr uint16, v uint8) { b.mem[addr] = v }

func TestCPUImplementsBus(t *testing.T) {
	// Vérifie que stubBus satisfait l'interface cpu6809.Bus.
	var _ cpu6809.Bus = (*stubBus)(nil)
}

func TestCPUReset(t *testing.T) {
	bus := &stubBus{}
	// Placer le vecteur de reset à 0xFFFE/0xFFFF = 0x1234
	bus.mem[0xFFFE] = 0x12
	bus.mem[0xFFFF] = 0x34

	cpu := cpu6809.New(bus)
	cpu.Reset()

	snap := cpu.Snapshot()
	if snap.PC != 0x1234 {
		t.Errorf("PC après Reset = 0x%04X, want 0x1234", snap.PC)
	}
	// CC doit avoir IRQ (0x10) et FIRQ (0x40) masqués au reset.
	if snap.CC&0x10 == 0 {
		t.Errorf("CC.I (IRQ mask) non positionné après Reset : CC = 0x%02X", snap.CC)
	}
	if snap.CC&0x40 == 0 {
		t.Errorf("CC.F (FIRQ mask) non positionné après Reset : CC = 0x%02X", snap.CC)
	}
}

func TestCPUSnapshotIndependent(t *testing.T) {
	bus := &stubBus{}
	cpu := cpu6809.New(bus)
	cpu.Reset()
	s1 := cpu.Snapshot()
	s2 := cpu.Snapshot()
	if s1 != s2 {
		t.Error("deux Snapshots successifs sans Step doivent être identiques")
	}
}
