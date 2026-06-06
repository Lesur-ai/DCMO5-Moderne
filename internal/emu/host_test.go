package emu

// host_test.go — orchestrateur temps réel. Tests : production audio
// déterministe (via tick), absence de data race (goroutine + accès concurrents),
// pause.

import (
	"sync"
	"testing"
	"time"

	"github.com/Lesur-ai/dcmo5/internal/audio"
	"github.com/Lesur-ai/dcmo5/internal/core"
	"github.com/Lesur-ai/dcmo5/internal/spec"
)

func s16le(lo, hi byte) int16 { return int16(uint16(lo) | uint16(hi)<<8) }

// nopMachine : machine dont la ROM ne fait que des NOP (avance déterministe).
func nopMachine(t *testing.T) *core.Machine {
	t.Helper()
	rom := make([]byte, 0x4000)
	for i := range rom {
		rom[i] = 0x12 // NOP
	}
	rom[0x3FFE] = 0xC0
	rom[0x3FFF] = 0x00
	m, err := core.NewMachine(core.Options{ROMSys: rom})
	if err != nil {
		t.Fatalf("NewMachine: %v", err)
	}
	m.Reset()
	return m
}

// TestHost_TickProducesAudio vérifie que tick avance l'émulation et pousse dans
// la ring le niveau sonore courant (déterministe, sans goroutine).
func TestHost_TickProducesAudio(t *testing.T) {
	const gain = 480
	m := nopMachine(t)
	h := New(m, gain)
	m.Write8(0xA7CD, 0x3F) // niveau max ; la ROM NOP ne le modifie pas

	h.tick(spec.CPUClockHz / 100) // ~10 ms d'émulation
	if h.stream.BufferedSamples() == 0 {
		t.Fatal("aucun échantillon produit par tick")
	}
	p := make([]byte, audio.BytesPerSample)
	h.stream.Read(p)
	if v := s16le(p[0], p[1]); v != int16(0x3F*gain) {
		t.Errorf("échantillon = %d, want %d (0x3F×gain)", v, 0x3F*gain)
	}
}

// TestHost_TickAppliesInput vérifie que l'instantané d'entrée est appliqué.
func TestHost_TickAppliesInput(t *testing.T) {
	m := nopMachine(t)
	h := New(m, 1)
	var in InputState
	in.Keys[0x20] = true // ESPACE pressé
	h.SetInput(in)
	h.tick(64)
	// La matrice clavier se lit via 0xA7C1 : col = (port[1]&0xFE)>>1, et le port
	// renvoie port[1] | touche[col] (bit 0x80 = relâchée). Pour interroger la
	// touche d'index 0x20, on sélectionne col=0x20 en écrivant port[1]=0x20<<1.
	m.Write8(0xA7C1, 0x20<<1)
	got := m.Read8(0xA7C1)
	if got&0x80 != 0 {
		t.Errorf("touche ESPACE non pressée après SetInput+tick (port=0x%02X)", got)
	}
}

// TestHost_StopIdempotent : Stop doit pouvoir être appelé plusieurs fois sans
// paniquer (fermeture de canal en double).
func TestHost_StopIdempotent(t *testing.T) {
	h := New(nopMachine(t), 1)
	h.Start()
	h.Stop()
	h.Stop() // ne doit pas paniquer
	// Stop sans Start préalable non plus.
	h2 := New(nopMachine(t), 1)
	h2.Stop()
}

// TestHost_Paused : en pause, l'état est rapporté correctement.
func TestHost_Paused(t *testing.T) {
	h := New(nopMachine(t), 1)
	if h.Paused() {
		t.Fatal("ne doit pas démarrer en pause")
	}
	h.SetPaused(true)
	if !h.Paused() {
		t.Error("SetPaused(true) non pris en compte")
	}
}

// TestHost_ConcurrentAccessNoRace lance la goroutine d'émulation et sollicite
// toutes les surfaces partagées en parallèle. À exécuter avec -race.
func TestHost_ConcurrentAccessNoRace(t *testing.T) {
	h := New(nopMachine(t), 480)
	h.Start()
	defer h.Stop()

	stop := make(chan struct{})
	var wg sync.WaitGroup

	worker := func(f func()) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					f()
				}
			}
		}()
	}

	worker(func() { h.SetInput(InputState{PenX: 1}) })
	pcm := make([]byte, 512)
	worker(func() { h.AudioStream().Read(pcm) })
	fb := make([]uint32, spec.FrameWidth*spec.FrameHeight)
	worker(func() { h.Framebuffer(fb) })
	worker(func() { h.Reset(); time.Sleep(5 * time.Millisecond) })

	time.Sleep(100 * time.Millisecond)
	close(stop)
	wg.Wait()
}
