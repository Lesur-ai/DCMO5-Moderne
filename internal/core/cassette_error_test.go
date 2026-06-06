package core_test

// Tests de la sémantique d'erreur cassette alignée réf C (Initprog + Erreur n),
// exposée de façon observable via Options.OnError.

import (
	"errors"
	"testing"

	"github.com/Lesur-ai/dcmo5/internal/core"
)

// eofTape : cassette dont la lecture échoue immédiatement (fin de bande) et qui
// enregistre les rembobinages, pour vérifier le comportement EOF.
type eofTape struct{ rewinds int }

func (t *eofTape) ReadByte() (byte, error) { return 0, errors.New("EOF") }
func (t *eofTape) WriteByte(byte) error    { return nil }
func (t *eofTape) Rewind() error           { t.rewinds++; return nil }
func (t *eofTape) Position() int64         { return 0 }

func TestCassetteError_NoTape_Code11(t *testing.T) {
	var got int
	m, _ := core.NewMachine(core.Options{OnError: func(code int) { got = code }})
	m.Reset()
	m.Entreesortie(0x42) // READOCTETK7 sans cassette
	if got != 11 {
		t.Errorf("cassette absente : code erreur = %d, want 11", got)
	}
}

func TestCassetteError_EOF_Code12_AndRewind(t *testing.T) {
	var got int
	tape := &eofTape{}
	m, _ := core.NewMachine(core.Options{Tape: tape, OnError: func(code int) { got = code }})
	m.Reset()
	m.Entreesortie(0x42) // READOCTETK7 sur bande vide → EOF
	if got != 12 {
		t.Errorf("EOF cassette : code erreur = %d, want 12", got)
	}
	if tape.rewinds != 1 {
		t.Errorf("EOF cassette : rembobinages = %d, want 1", tape.rewinds)
	}
}

func TestCassetteError_NoSink_NoPanic(t *testing.T) {
	// Sans OnError configuré, l'erreur ne doit pas paniquer (sink nil-safe).
	m, _ := core.NewMachine(core.Options{})
	m.Reset()
	m.Entreesortie(0x42) // ne doit pas paniquer
}

// roTape : cassette dont l'écriture échoue (protégée / lecture seule).
type roTape struct{}

func (roTape) ReadByte() (byte, error) { return 0, nil }
func (roTape) WriteByte(byte) error    { return errors.New("lecture seule") }
func (roTape) Rewind() error           { return nil }
func (roTape) Position() int64         { return 0 }

func TestCassetteWriteError_NoTape_Code11(t *testing.T) {
	var got int
	m, _ := core.NewMachine(core.Options{OnError: func(code int) { got = code }})
	m.Reset()
	m.Entreesortie(0x45) // WRITEOCTETK7 sans cassette
	if got != 11 {
		t.Errorf("écriture sans cassette : code = %d, want 11", got)
	}
}

func TestCassetteWriteError_Protected_Code13(t *testing.T) {
	var got int
	m, _ := core.NewMachine(core.Options{Tape: roTape{}, OnError: func(code int) { got = code }})
	m.Reset()
	m.Entreesortie(0x45) // WRITEOCTETK7 sur cassette protégée
	if got != 13 {
		t.Errorf("écriture protégée : code = %d, want 13", got)
	}
}
