package core_test

// sounddump_test.go — outil de diagnostic : capture le son produit par le CŒUR
// (sans Ebitengine) dans un WAV, pour analyser hors ligne le bruit « tac ».
// Activé par DCMO5_SOUND_DUMP=1 et nécessite la vraie ROM.
//
//   DCMO5_SOUND_DUMP=1 go test ./internal/core -run TestSoundDump -v

import (
	"encoding/binary"
	"os"
	"testing"

	"github.com/Lesur-ai/dcmo5/internal/spec"
)

func TestSoundDump(t *testing.T) {
	if os.Getenv("DCMO5_SOUND_DUMP") == "" {
		t.Skip("diagnostic — définir DCMO5_SOUND_DUMP=1")
	}
	rom := loadROM(t) // helper existant (rom_integration_test.go)
	m, _ := newMachineWithROM(t, rom)

	const skipSeconds = 32 // atteindre le prompt BASIC (scan clavier actif)
	const seconds = 6
	cyclesPerFrame := spec.CPUClockHz / 60
	buf := make([]uint8, 8192)
	// Avancer jusqu'au prompt sans capturer (vider le tampon).
	for f := 0; f < 60*skipSeconds; f++ {
		m.Step(cyclesPerFrame)
		for n := m.DrainAudio(buf); n == len(buf); n = m.DrainAudio(buf) {
		}
	}
	var levels []uint8
	// Capturer le régime au prompt.
	for f := 0; f < 60*seconds; f++ {
		m.Step(cyclesPerFrame)
		for {
			n := m.DrainAudio(buf)
			if n == 0 {
				break
			}
			levels = append(levels, buf[:n]...)
			if n < len(buf) {
				break
			}
		}
	}
	t.Logf("échantillons capturés: %d (~%.2fs)", len(levels), float64(len(levels))/spec.AudioSampleRate)

	// Convertir niveaux 6 bits → PCM s16 mono centré, écrire un WAV.
	pcm := make([]byte, len(levels)*2)
	for i, lv := range levels {
		v := int16(int(lv) * 480) // unipolaire (repos 0 = silence)
		binary.LittleEndian.PutUint16(pcm[i*2:], uint16(v))
	}
	path := "/tmp/dcmo5_core_sound.wav"
	if err := writeWAV(path, pcm, spec.AudioSampleRate); err != nil {
		t.Fatalf("écriture WAV: %v", err)
	}
	t.Logf("WAV écrit: %s", path)

	// Statistiques : transitions + valeurs (repos = silence si niveau constant 0).
	transitions := 0
	min, max := levels[0], levels[0]
	for i := 1; i < len(levels); i++ {
		if levels[i] != levels[i-1] {
			transitions++
		}
		if levels[i] < min {
			min = levels[i]
		}
		if levels[i] > max {
			max = levels[i]
		}
	}
	t.Logf("transitions: %d (%.1f/s) ; niveau premier=%d min=%d max=%d", transitions, float64(transitions)/seconds, levels[0], min, max)
}

// writeWAV écrit un fichier WAV PCM 16 bits mono.
func writeWAV(path string, pcm []byte, sampleRate int) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	var hdr [44]byte
	copy(hdr[0:], "RIFF")
	binary.LittleEndian.PutUint32(hdr[4:], uint32(36+len(pcm)))
	copy(hdr[8:], "WAVE")
	copy(hdr[12:], "fmt ")
	binary.LittleEndian.PutUint32(hdr[16:], 16)
	binary.LittleEndian.PutUint16(hdr[20:], 1) // PCM
	binary.LittleEndian.PutUint16(hdr[22:], 1) // mono
	binary.LittleEndian.PutUint32(hdr[24:], uint32(sampleRate))
	binary.LittleEndian.PutUint32(hdr[28:], uint32(sampleRate*2))
	binary.LittleEndian.PutUint16(hdr[32:], 2)  // block align
	binary.LittleEndian.PutUint16(hdr[34:], 16) // bits
	copy(hdr[36:], "data")
	binary.LittleEndian.PutUint32(hdr[40:], uint32(len(pcm)))
	if _, err := f.Write(hdr[:]); err != nil {
		return err
	}
	_, err = f.Write(pcm)
	return err
}
