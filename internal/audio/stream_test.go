package audio

// stream_test.go — conversion niveau→PCM et FIFO (purs, headless).
// Non complaisants : on vérifie les octets PCM produits, le silence, le bornage.

import "testing"

// s16 décode un échantillon signé 16 bits little-endian.
func s16(lo, hi byte) int16 {
	return int16(uint16(lo) | uint16(hi)<<8)
}

func TestStream_CenterLevelIsSilence(t *testing.T) {
	s := NewStream(100, 1000)
	s.Write([]uint8{centerLevel}) // niveau central → 0
	if s.Buffered() != BytesPerSample {
		t.Fatalf("Buffered = %d, want %d", s.Buffered(), BytesPerSample)
	}
	p := make([]byte, BytesPerSample)
	s.Read(p)
	if v := s16(p[0], p[1]); v != 0 {
		t.Errorf("niveau central → %d, want 0 (silence)", v)
	}
}

func TestStream_LevelToAmplitude(t *testing.T) {
	const gain = 100
	s := NewStream(gain, 1000)
	s.Write([]uint8{0x3F}) // max → (63-32)*gain = 3100
	p := make([]byte, BytesPerSample)
	s.Read(p)
	wantL := int16((0x3F - centerLevel) * gain)
	if v := s16(p[0], p[1]); v != wantL {
		t.Errorf("niveau 0x3F → %d, want %d", v, wantL)
	}
	// Stéréo : canal droit identique au gauche.
	if l, r := s16(p[0], p[1]), s16(p[2], p[3]); l != r {
		t.Errorf("stéréo non identique: L=%d R=%d", l, r)
	}
}

func TestStream_NegativeBelowCenter(t *testing.T) {
	const gain = 100
	s := NewStream(gain, 1000)
	s.Write([]uint8{0}) // 0 → (0-32)*100 = -3200
	p := make([]byte, BytesPerSample)
	s.Read(p)
	if v := s16(p[0], p[1]); v != -3200 {
		t.Errorf("niveau 0 → %d, want -3200", v)
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
			t.Fatalf("octet %d = 0x%02X, want 0 (silence)", i, b)
		}
	}
}

func TestStream_FIFOOrder(t *testing.T) {
	const gain = 1
	s := NewStream(gain, 1000)
	s.Write([]uint8{centerLevel + 10, centerLevel + 20}) // +10, +20
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
	// gain élevé → doit être clampé à [-32768, 32767], pas déborder.
	s := NewStream(100000, 10)
	s.Write([]uint8{0x3F, 0x00}) // +31*100000 et -32*100000
	p := make([]byte, 2*BytesPerSample)
	s.Read(p)
	if v := s16(p[0], p[1]); v != 32767 {
		t.Errorf("clamp haut = %d, want 32767", v)
	}
	if v := s16(p[4], p[5]); v != -32768 {
		t.Errorf("clamp bas = %d, want -32768", v)
	}
}
