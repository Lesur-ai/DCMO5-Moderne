// Package audio convertit les niveaux sonores du MO5 (6 bits) en PCM.
// Logique pure (sans bibliothèque audio), entièrement testable headless.
//
// En architecture « audio-driven », c'est le thread audio qui pilote
// l'émulation et encode les échantillons à la demande ; ce package fournit la
// conversion niveau → PCM, le tampon FIFO n'est plus nécessaire.
package audio

// BytesPerSample : 2 octets (s16) × 2 canaux (stéréo), format attendu par
// Ebitengine.
const BytesPerSample = 4

// Le niveau de repos du MO5 est sound=0 (ref dcmo5main.c : stream = sound+128,
// soit le silence en U8 centré). La conversion est donc unipolaire : level=0
// produit le silence (0 en PCM signé), évitant tout offset continu au repos.

// EncodeLevel convertit un niveau MO5 (0..63) en amplitude PCM signée 16 bits,
// avec écrêtage de sécurité. level=0 → 0 (silence).
func EncodeLevel(level uint8, gain int) int16 {
	v := int(level) * gain
	if v > 32767 {
		v = 32767
	}
	return int16(v)
}

// PutStereoSample écrit l'échantillon (s16le, dupliqué L/R) dans dst[:4].
func PutStereoSample(dst []byte, sample int16) {
	lo, hi := byte(sample), byte(sample>>8)
	dst[0], dst[1], dst[2], dst[3] = lo, hi, lo, hi
}
