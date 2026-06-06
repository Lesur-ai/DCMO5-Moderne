package audio

// stream_test.go — conversion niveau→PCM et FIFO (purs, headless).
// Non complaisants : on vérifie les octets PCM produits, le silence, le bornage.
// Mapping unipolaire : level 0 = silence, amplitude = level*gain.

import "testing"

// s16 décode un échantillon signé 16 bits little-endian.
func s16(lo, hi byte) int16 {
	return int16(uint16(lo) | uint16(hi)<<8)
}

func TestStream_RestLevelIsSilence(t *testing.T) {
	s := NewStream(100, 1000)
	s.Write([]uint8{0}) // niveau de repos → silence
	if s.Buffered() != BytesPerSample {
		t.Fatalf("Buffered = %d, want %d", s.Buffered(), BytesPerSample)
	}
	p := make([]byte, BytesPerSample)
	s.Read(p)
	if v := s16(p[0], p[1]); v != 0 {
		t.Errorf("niveau 0 → %d, want 0 (silence)", v)
	}
}

func TestStream_LevelToAmplitude(t *testing.T) {
	const gain = 100
	s := NewStream(gain, 1000)
	s.Write([]uint8{0x3F}) // max → 63*gain = 6300
	p := make([]byte, BytesPerSample)
	s.Read(p)
	wantL := int16(0x3F * gain)
	if v := s16(p[0], p[1]); v != wantL {
		t.Errorf("niveau 0x3F → %d, want %d", v, wantL)
	}
	// Stéréo : canal droit identique au gauche.
	if l, r := s16(p[0], p[1]), s16(p[2], p[3]); l != r {
		t.Errorf("stéréo non identique: L=%d R=%d", l, r)
	}
}

func TestStream_AmplitudeMonotonic(t *testing.T) {
	const gain = 100
	s := NewStream(gain, 1000)
	s.Write([]uint8{10, 20}) // amplitudes croissantes, toutes positives
	p := make([]byte, 2*BytesPerSample)
	s.Read(p)
	a := s16(p[0], p[1])
	b := s16(p[4], p[5])
	if a != 1000 || b != 2000 {
		t.Errorf("amplitudes = %d,%d, want 1000,2000", a, b)
	}
	if a < 0 || b < 0 {
		t.Errorf("le signal doit rester positif (unipolaire): %d,%d", a, b)
	}
}

func TestStream_ReadEmptyIsSilenceNoEOF(t *testing.T) {
	s := NewStream(100, 1000)
	p := make([]byte, 16)
	for i := range p {
		p[i] = 0xAB // pollution pour vérifier l'écrasement par le silence
	}
	n, err := s.Read(p)
	if err != nil {
		t.Fatalf("Read vide retourne une erreur: %v (ne doit jamais EOF)", err)
	}
	if n != len(p) {
		t.Fatalf("Read vide n=%d, want %d (flux continu)", n, len(p))
	}
	for i, b := range p {
		if b != 0 {
			t.Fatalf("octet %d = 0x%02X, want 0 (silence, jamais écrit)", i, b)
		}
	}
}

// TestStream_HoldsLastSampleOnUnderrun vérifie qu'en sous-alimentation, Read
// répète le dernier échantillon (anti-clic) au lieu d'un silence brutal.
func TestStream_HoldsLastSampleOnUnderrun(t *testing.T) {
	const gain = 1
	s := NewStream(gain, 1000)
	s.Write([]uint8{25}) // dernier échantillon = 25
	p := make([]byte, 3*BytesPerSample)
	s.Read(p)
	for k := 0; k < 3; k++ {
		if v := s16(p[k*BytesPerSample], p[k*BytesPerSample+1]); v != 25 {
			t.Errorf("échantillon %d = %d, want 25 (maintien du dernier, pas silence)", k, v)
		}
	}
}

func TestStream_FIFOOrder(t *testing.T) {
	const gain = 1
	s := NewStream(gain, 1000)
	s.Write([]uint8{10, 20})
	p := make([]byte, 2*BytesPerSample)
	s.Read(p)
	if v := s16(p[0], p[1]); v != 10 {
		t.Errorf("1er échantillon = %d, want 10", v)
	}
	// 2e échantillon : octets 4-7 (chaque échantillon stéréo fait 4 octets).
	if v := s16(p[4], p[5]); v != 20 {
		t.Errorf("2e échantillon = %d, want 20", v)
	}
}

func TestStream_Bounded(t *testing.T) {
	const maxSamples = 8
	s := NewStream(100, maxSamples)
	levels := make([]uint8, 1000) // bien plus que la capacité
	s.Write(levels)
	if s.Buffered() > maxSamples*BytesPerSample {
		t.Errorf("tampon non borné: %d octets, max %d", s.Buffered(), maxSamples*BytesPerSample)
	}
}

func TestStream_ClampsToInt16(t *testing.T) {
	// gain élevé → doit être clampé à 32767, sans déborder ni passer négatif.
	s := NewStream(100000, 10)
	s.Write([]uint8{0x3F}) // 63*100000 → clamp 32767
	p := make([]byte, BytesPerSample)
	s.Read(p)
	if v := s16(p[0], p[1]); v != 32767 {
		t.Errorf("clamp haut = %d, want 32767", v)
	}
}
