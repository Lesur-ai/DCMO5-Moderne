package audio

// encode_test.go — conversion niveau MO5 → PCM (pure, headless).

import "testing"

func s16(lo, hi byte) int16 { return int16(uint16(lo) | uint16(hi)<<8) }

func TestEncodeLevel_RestIsSilence(t *testing.T) {
	if v := EncodeLevel(0, 480); v != 0 {
		t.Errorf("niveau 0 → %d, want 0 (silence)", v)
	}
}

func TestEncodeLevel_Amplitude(t *testing.T) {
	const gain = 100
	if v := EncodeLevel(0x3F, gain); v != int16(0x3F*gain) {
		t.Errorf("niveau 0x3F → %d, want %d", v, 0x3F*gain)
	}
	// Monotone et toujours positif (unipolaire).
	if EncodeLevel(10, gain) >= EncodeLevel(20, gain) {
		t.Error("amplitude doit croître avec le niveau")
	}
	if EncodeLevel(5, gain) < 0 {
		t.Error("le signal doit rester positif (unipolaire)")
	}
}

func TestEncodeLevel_Clamp(t *testing.T) {
	if v := EncodeLevel(0x3F, 100000); v != 32767 {
		t.Errorf("écrêtage = %d, want 32767", v)
	}
}

func TestPutStereoSample(t *testing.T) {
	var dst [4]byte
	PutStereoSample(dst[:], 12345)
	if l, r := s16(dst[0], dst[1]), s16(dst[2], dst[3]); l != 12345 || r != 12345 {
		t.Errorf("stéréo = L:%d R:%d, want 12345/12345 (dupliqué)", l, r)
	}
}
