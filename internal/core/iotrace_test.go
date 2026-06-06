package core_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Lesur-ai/dcmo5/internal/core"
	"github.com/Lesur-ai/dcmo5/internal/media/impl"
)

// TestIOTrace_DisabledByDefault garantit le caractère opt-in : sans
// EnableIOTrace, aucun compteur n'est tenu (coût nul, comportement inchangé).
func TestIOTrace_DisabledByDefault(t *testing.T) {
	m, _ := core.NewMachine(core.Options{})
	m.Reset()
	m.Entreesortie(0x4B) // crayon : doit s'exécuter sans tracer
	if got := m.IOTraceCounts(); len(got) != 0 {
		t.Errorf("trace désactivée par défaut: compteurs = %v, want vide", got)
	}
}

// TestIOTrace_CountsAndFormat vérifie, pour chaque famille de trap, que le
// compteur est incrémenté et que la ligne de journal porte les paramètres
// d'entrée pertinents (assertions observables sur la sortie réelle).
func TestIOTrace_CountsAndFormat(t *testing.T) {
	var buf bytes.Buffer

	// Cassette montée pour que le trap 0x42 traverse la lecture réelle.
	tapePath := t.TempDir() + "/trace.k7"
	tape, err := impl.NewTape(tapePath)
	if err != nil {
		t.Fatalf("NewTape: %v", err)
	}
	if err := tape.WriteByte(0x55); err != nil {
		t.Fatalf("WriteByte: %v", err)
	}
	if err := tape.Rewind(); err != nil {
		t.Fatalf("Rewind: %v", err)
	}
	tape.Close()
	tape2, err := impl.OpenTape(tapePath, true)
	if err != nil {
		t.Fatalf("OpenTape: %v", err)
	}

	m, _ := core.NewMachine(core.Options{Tape: tape2})
	m.Reset()
	m.EnableIOTrace(&buf)

	// Crayon dans la zone active pour figer des coordonnées connues.
	m.SetPen(120, 80, true)
	m.Entreesortie(0x4B) // READPENXY
	m.Entreesortie(0x42) // READOCTETK7
	m.Entreesortie(0x42) // READOCTETK7 (2e occurrence)

	counts := m.IOTraceCounts()
	if counts[0x4B] != 1 {
		t.Errorf("compteur READPENXY = %d, want 1", counts[0x4B])
	}
	if counts[0x42] != 2 {
		t.Errorf("compteur READOCTETK7 = %d, want 2", counts[0x42])
	}

	// La cassette a réellement été lue : le 1er READOCTETK7 place l'octet en RAM.
	if v := m.Read8(0x2045); v != 0x55 {
		t.Errorf("lecture cassette réelle: RAM[0x2045] = 0x%02X, want 0x55", v)
	}

	out := buf.String()
	for _, want := range []string{
		"io=4B READPENXY xpen=120 ypen=80",
		"io=42 READOCTETK7",
		"k7bit=", "k7octet=", "A=", "tape=true",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("journal trace ne contient pas %q\n--- trace ---\n%s", want, out)
		}
	}
	// Deux occurrences du trap cassette → deux lignes READOCTETK7.
	if n := strings.Count(out, "READOCTETK7"); n != 2 {
		t.Errorf("nombre de lignes READOCTETK7 = %d, want 2", n)
	}
}

// TestIOTrace_DiskParams vérifie que la trace disque capture les paramètres
// lus en RAM (unité/piste/secteur/destination) tels que vus par la ROM.
func TestIOTrace_DiskParams(t *testing.T) {
	var buf bytes.Buffer
	path := t.TempDir() + "/trace.fd"
	disk, err := impl.NewDisk(path)
	if err != nil {
		t.Fatalf("NewDisk: %v", err)
	}
	disk.Close()
	disk2, err := impl.OpenDisk(path, false)
	if err != nil {
		t.Fatalf("OpenDisk: %v", err)
	}

	m, _ := core.NewMachine(core.Options{Disk: disk2})
	m.Reset()
	m.EnableIOTrace(&buf)

	m.Write8(0x2049, 1)    // unité 1
	m.Write8(0x204B, 5)    // piste 5
	m.Write8(0x204C, 3)    // secteur 3
	m.Write8(0x204F, 0x61) // dest hi
	m.Write8(0x2050, 0x00) // dest lo → 0x6100
	m.Entreesortie(0x14)   // READSECTOR

	if c := m.IOTraceCounts()[0x14]; c != 1 {
		t.Fatalf("compteur READSECTOR = %d, want 1", c)
	}
	out := buf.String()
	if !strings.Contains(out, "io=14 READSECTOR u=1 trkHi=0 trk=5 sec=3 dest=6100") {
		t.Errorf("trace disque incorrecte\n--- trace ---\n%s", out)
	}
}

// TestIOTrace_DisableResets garantit qu'un EnableIOTrace(nil) coupe la trace
// et que les compteurs repartent de zéro après réactivation.
func TestIOTrace_DisableResets(t *testing.T) {
	var buf bytes.Buffer
	m, _ := core.NewMachine(core.Options{})
	m.Reset()

	m.EnableIOTrace(&buf)
	m.Entreesortie(0x4B)
	if m.IOTraceCounts()[0x4B] != 1 {
		t.Fatalf("compteur attendu 1 après 1 trap")
	}

	m.EnableIOTrace(nil) // désactivation
	m.Entreesortie(0x4B)
	if got := m.IOTraceCounts(); len(got) != 0 {
		t.Errorf("après désactivation: compteurs = %v, want vide", got)
	}

	m.EnableIOTrace(&buf) // réactivation → compteurs remis à zéro
	if got := m.IOTraceCounts()[0x4B]; got != 0 {
		t.Errorf("après réactivation: compteur = %d, want 0", got)
	}
}
