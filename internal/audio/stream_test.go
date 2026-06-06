package audio

// stream_test.go — ring FIFO PCM (pure, headless). Non complaisant.

import "testing"

func TestStream_WriteReadFIFO(t *testing.T) {
	s := NewStream(1, 1000)
	s.Write([]uint8{10, 20})
	if s.BufferedSamples() != 2 {
		t.Fatalf("BufferedSamples = %d, want 2", s.BufferedSamples())
	}
	p := make([]byte, 2*BytesPerSample)
	s.Read(p)
	if v := s16(p[0], p[1]); v != 10 {
		t.Errorf("1er = %d, want 10", v)
	}
	if v := s16(p[4], p[5]); v != 20 {
		t.Errorf("2e = %d, want 20", v)
	}
}

func TestStream_HoldsLastOnUnderrun(t *testing.T) {
	s := NewStream(1, 1000)
	s.Write([]uint8{25})
	p := make([]byte, 3*BytesPerSample)
	s.Read(p)
	for k := 0; k < 3; k++ {
		if v := s16(p[k*BytesPerSample], p[k*BytesPerSample+1]); v != 25 {
			t.Errorf("échantillon %d = %d, want 25 (maintien)", k, v)
		}
	}
}

func TestStream_EmptyIsSilenceNoEOF(t *testing.T) {
	s := NewStream(1, 1000)
	p := make([]byte, 8)
	for i := range p {
		p[i] = 0xAB
	}
	n, err := s.Read(p)
	if err != nil || n != len(p) {
		t.Fatalf("Read vide: n=%d err=%v, want %d/nil", n, err, len(p))
	}
	for i, b := range p {
		if b != 0 {
			t.Fatalf("octet %d = 0x%02X, want 0 (silence initial)", i, b)
		}
	}
}

// TestStream_PartialReadNoStaleBytes vérifie qu'une lecture de taille non
// multiple de la frame ne laisse aucun octet « stale » (tout est rempli).
func TestStream_PartialReadNoStaleBytes(t *testing.T) {
	s := NewStream(1, 1000)
	s.Write([]uint8{25})                // last = échantillon 25
	p := make([]byte, BytesPerSample+2) // 6 octets (non multiple de 4)
	for i := range p {
		p[i] = 0xCC
	}
	s.Read(p)
	for i, b := range p {
		if b == 0xCC {
			t.Fatalf("octet %d resté stale (0xCC) après Read partiel", i)
		}
	}
}

// TestStream_HoldAcrossAlignedReads vérifie qu'en sous-alimentation, des
// lectures successives alignées sur la frame restituent bien le dernier
// échantillon (L et R corrects), sans corruption.
func TestStream_HoldAcrossAlignedReads(t *testing.T) {
	s := NewStream(1, 1000)
	s.Write([]uint8{25})                 // last = [25,0,25,0]
	s.Read(make([]byte, BytesPerSample)) // consomme l'échantillon réel ; ring vide

	for k := 0; k < 3; k++ {
		p := make([]byte, BytesPerSample)
		s.Read(p)
		if l, r := s16(p[0], p[1]), s16(p[2], p[3]); l != 25 || r != 25 {
			t.Errorf("read %d en underrun: L=%d R=%d, want 25/25 (maintien)", k, l, r)
		}
	}
}

func TestStream_Bounded(t *testing.T) {
	const maxSamples = 8
	s := NewStream(1, maxSamples)
	s.Write(make([]uint8, 1000))
	if s.Buffered() > maxSamples*BytesPerSample {
		t.Errorf("non borné: %d octets", s.Buffered())
	}
}
