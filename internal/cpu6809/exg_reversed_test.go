package cpu6809_test

import (
	"testing"

	"github.com/Lesur-ai/dcmo5/internal/cpu6809"
)

// Régression : EXG est symétrique. Les encodages « inversés » des paires 16 bits
// (ex. EXG Y,X = postbyte 0x21) doivent échanger les registres comme leur forme
// canonique (0x12). Auparavant ces postbytes tombaient dans le default et ne
// faisaient RIEN (cause du plantage du loader de 5emeaxe : EXG Y,X ignoré).

func TestEXG_YX_reversed(t *testing.T) {
	bus := &stubBus{}
	bus.set16(0xFFFE, 0x1000)
	bus.mem[0x1000] = 0x8E // LDX #$1111
	bus.set16(0x1001, 0x1111)
	bus.mem[0x1003] = 0x10 // LDY #$2222 (préfixe page 1)
	bus.mem[0x1004] = 0x8E
	bus.set16(0x1005, 0x2222)
	bus.mem[0x1007] = 0x1E // EXG
	bus.mem[0x1008] = 0x21 // Y,X (forme inversée de 0x12)
	cpu := cpu6809.New(bus)
	cpu.Reset()
	cpu.Step() // LDX
	cpu.Step() // LDY
	cpu.Step() // EXG Y,X
	s := cpu.Snapshot()
	if s.X != 0x2222 || s.Y != 0x1111 {
		t.Errorf("EXG Y,X : X=0x%04X Y=0x%04X, want X=0x2222 Y=0x1111", s.X, s.Y)
	}
}

func TestEXG_UX_reversed(t *testing.T) {
	bus := &stubBus{}
	bus.set16(0xFFFE, 0x1000)
	bus.mem[0x1000] = 0x8E // LDX #$AAAA
	bus.set16(0x1001, 0xAAAA)
	bus.mem[0x1003] = 0xCE // LDU #$5555
	bus.set16(0x1004, 0x5555)
	bus.mem[0x1006] = 0x1E // EXG
	bus.mem[0x1007] = 0x31 // U,X (inversé de 0x13)
	cpu := cpu6809.New(bus)
	cpu.Reset()
	cpu.Step() // LDX
	cpu.Step() // LDU
	cpu.Step() // EXG U,X
	s := cpu.Snapshot()
	if s.X != 0x5555 || s.U != 0xAAAA {
		t.Errorf("EXG U,X : X=0x%04X U=0x%04X, want X=0x5555 U=0xAAAA", s.X, s.U)
	}
}

func TestEXG_XD_reversed(t *testing.T) {
	// EXG X,D = 0x10 (inversé de 0x01 D,X) → échange D et X.
	bus := &stubBus{}
	bus.set16(0xFFFE, 0x1000)
	bus.mem[0x1000] = 0xCC // LDD #$3333
	bus.set16(0x1001, 0x3333)
	bus.mem[0x1003] = 0x8E // LDX #$4444
	bus.set16(0x1004, 0x4444)
	bus.mem[0x1006] = 0x1E // EXG
	bus.mem[0x1007] = 0x10 // X,D
	cpu := cpu6809.New(bus)
	cpu.Reset()
	cpu.Step() // LDD
	cpu.Step() // LDX
	cpu.Step() // EXG X,D
	s := cpu.Snapshot()
	d := uint16(s.A)<<8 | uint16(s.B)
	if s.X != 0x3333 || d != 0x4444 {
		t.Errorf("EXG X,D : X=0x%04X D=0x%04X, want X=0x3333 D=0x4444", s.X, d)
	}
}

func TestEXG_XY_canonical_unchanged(t *testing.T) {
	// La forme canonique 0x12 doit continuer à fonctionner.
	bus := &stubBus{}
	bus.set16(0xFFFE, 0x1000)
	bus.mem[0x1000] = 0x8E // LDX #$0001
	bus.set16(0x1001, 0x0001)
	bus.mem[0x1003] = 0x10 // LDY #$0002
	bus.mem[0x1004] = 0x8E
	bus.set16(0x1005, 0x0002)
	bus.mem[0x1007] = 0x1E // EXG X,Y
	bus.mem[0x1008] = 0x12
	cpu := cpu6809.New(bus)
	cpu.Reset()
	cpu.Step()
	cpu.Step()
	cpu.Step()
	s := cpu.Snapshot()
	if s.X != 0x0002 || s.Y != 0x0001 {
		t.Errorf("EXG X,Y : X=0x%04X Y=0x%04X, want X=0x0002 Y=0x0001", s.X, s.Y)
	}
}
