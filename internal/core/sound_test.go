package core_test

// sound_test.go — émulation du son MO5 (P9.4). Tests observables : niveau
// sonore piloté par les ports, débit et contenu du tampon d'échantillons.

import (
	"testing"

	"github.com/Lesur-ai/dcmo5/internal/core"
	"github.com/Lesur-ai/dcmo5/internal/spec"
)

// makeNOPMachine construit une machine dont la ROM ne fait que des NOP, pour
// avancer le temps de façon déterministe sans dépendre d'une vraie ROM.
func makeNOPMachine(t *testing.T) *core.Machine {
	t.Helper()
	rom := make([]byte, 0x4000)
	for i := range rom {
		rom[i] = 0x12 // NOP
	}
	rom[0x3FFE] = 0xC0 // vecteur reset → 0xC000
	rom[0x3FFF] = 0x00
	m, err := core.NewMachine(core.Options{ROMSys: rom})
	if err != nil {
		t.Fatalf("NewMachine: %v", err)
	}
	m.Reset()
	return m
}

// TestSound_PortSetsLevel vérifie que le port 0xA7CD fixe le niveau sonore,
// observable dans les échantillons produits.
func TestSound_PortSetsLevel(t *testing.T) {
	m := makeNOPMachine(t)
	m.Write8(0xA7CD, 0x2A) // niveau 42
	m.Step(spec.CPUClockHz / 100)

	buf := make([]uint8, spec.AudioSampleRate)
	n := m.DrainAudio(buf)
	if n == 0 {
		t.Fatal("aucun échantillon produit")
	}
	for i := 0; i < n; i++ {
		if buf[i] != 0x2A {
			t.Fatalf("échantillon %d = 0x%02X, want 0x2A (niveau port)", i, buf[i])
		}
	}
}

// TestSound_PortA7C1Bit vérifie le son 1-bit via 0xA7C1 (bit 0 → niveau 32/0).
func TestSound_PortA7C1Bit(t *testing.T) {
	m := makeNOPMachine(t)
	m.Write8(0xA7C1, 0x01) // bit haut-parleur à 1 → niveau 32
	m.Step(spec.CPUClockHz / 100)
	buf := make([]uint8, spec.AudioSampleRate)
	n := m.DrainAudio(buf)
	if n == 0 || buf[0] != 32 {
		t.Fatalf("0xA7C1 bit1: échantillon[0] = %d (n=%d), want 32", buf[0], n)
	}

	m.Write8(0xA7C1, 0x00) // bit à 0 → niveau 0
	m.Step(spec.CPUClockHz / 100)
	n = m.DrainAudio(buf)
	if n == 0 || buf[0] != 0 {
		t.Fatalf("0xA7C1 bit0: échantillon[0] = %d (n=%d), want 0", buf[0], n)
	}
}

// TestSound_SampleRate vérifie qu'une seconde simulée produit ~AudioSampleRate
// échantillons (à 1 % près), preuve d'un débit correct.
func TestSound_SampleRate(t *testing.T) {
	m := makeNOPMachine(t)
	m.Write8(0xA7CD, 0x10)

	const drainChunk = 4096
	buf := make([]uint8, drainChunk)
	total := 0
	// Simuler 1 seconde CPU en draine régulièrement (comme l'app par frame).
	remaining := spec.CPUClockHz
	for remaining > 0 {
		step := spec.CPUClockHz / 60
		if step > remaining {
			step = remaining
		}
		m.Step(step)
		remaining -= step
		for {
			n := m.DrainAudio(buf)
			total += n
			if n < len(buf) {
				break
			}
		}
	}
	low, high := spec.AudioSampleRate*99/100, spec.AudioSampleRate*101/100
	if total < low || total > high {
		t.Errorf("échantillons sur 1 s = %d, want ~%d (±1%%)", total, spec.AudioSampleRate)
	}
}

// TestSound_DrainEmptiesBuffer vérifie que DrainAudio vide le tampon.
func TestSound_DrainEmptiesBuffer(t *testing.T) {
	m := makeNOPMachine(t)
	m.Write8(0xA7CD, 0x05)
	m.Step(spec.CPUClockHz / 50)
	if m.AudioBacklog() == 0 {
		t.Fatal("le tampon devrait contenir des échantillons")
	}
	buf := make([]uint8, spec.AudioSampleRate)
	m.DrainAudio(buf)
	if m.AudioBacklog() != 0 {
		t.Errorf("après DrainAudio: backlog = %d, want 0", m.AudioBacklog())
	}
}

// TestSound_BacklogBounded vérifie que sans drainage, le tampon reste borné.
func TestSound_BacklogBounded(t *testing.T) {
	m := makeNOPMachine(t)
	m.Write8(0xA7CD, 0x01)
	// Simuler 2 s sans jamais drainer.
	for i := 0; i < 120; i++ {
		m.Step(spec.CPUClockHz / 60)
	}
	if bl := m.AudioBacklog(); bl > spec.AudioSampleRate {
		t.Errorf("backlog non borné: %d échantillons (sans drainage)", bl)
	}
}
