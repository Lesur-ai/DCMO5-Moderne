// Package audio convertit les niveaux sonores du MO5 (6 bits) en flux PCM
// consommable par une couche audio (Ebitengine). La logique est pure et
// thread-safe : elle ne dépend d'aucune bibliothèque graphique/audio et est
// donc entièrement testable headless.
//
// Format produit : PCM signé 16 bits little-endian, 2 canaux (stéréo), comme
// l'exige Ebitengine. Le signal MO5 étant mono, les deux canaux sont identiques.
package audio

import "sync"

// BytesPerSample : 2 octets (s16) × 2 canaux (stéréo).
const BytesPerSample = 4

// Le niveau de repos du MO5 est sound=0 (ref dcmo5main.c : stream = sound+128,
// soit le silence en U8 centré). La conversion est donc unipolaire : level=0
// produit le silence (0 en PCM signé), évitant tout offset continu au repos.

// Stream est une file FIFO thread-safe de PCM. L'émulation y écrit des niveaux
// (Write, thread du jeu) ; la couche audio y lit du PCM (Read, thread audio).
type Stream struct {
	mu   sync.Mutex
	buf  []byte
	gain int
	max  int     // capacité maximale en octets (0 = illimité)
	last [4]byte // dernier échantillon stéréo écrit (maintenu en sous-alimentation)
}

// NewStream crée un flux. gain amplifie l'écart au niveau central ; maxSamples
// borne le tampon (échantillons) pour éviter toute croissance mémoire si la
// lecture prend du retard.
func NewStream(gain, maxSamples int) *Stream {
	return &Stream{gain: gain, max: maxSamples * BytesPerSample}
}

// Write convertit des niveaux 6 bits en PCM s16le stéréo et les met en file.
func (s *Stream) Write(levels []uint8) {
	if len(levels) == 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, lv := range levels {
		v := int(lv) * s.gain // unipolaire : repos (0) = silence
		if v > 32767 {
			v = 32767
		}
		lo, hi := byte(v), byte(v>>8)
		s.buf = append(s.buf, lo, hi, lo, hi) // L puis R, identiques
		s.last = [4]byte{lo, hi, lo, hi}      // mémoriser pour le maintien
	}
	// Borne : si le tampon dépasse, abandonner les plus anciens (multiple de 4).
	if s.max > 0 && len(s.buf) > s.max {
		drop := len(s.buf) - s.max
		rest := copy(s.buf, s.buf[drop:])
		s.buf = s.buf[:rest]
	}
}

// Read fournit du PCM à la couche audio. Le flux doit être continu : si le
// tampon est insuffisant, on MAINTIENT le dernier échantillon (au lieu d'un
// silence brutal qui produirait un clic à chaque sous-alimentation), et on ne
// retourne jamais io.EOF (sinon le lecteur s'arrêterait définitivement).
func (s *Stream) Read(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := copy(p, s.buf)
	rest := copy(s.buf, s.buf[n:]) // compacter le reliquat au début
	s.buf = s.buf[:rest]
	// Compléter par maintien du dernier échantillon (anti-clic). p et n sont
	// alignés sur BytesPerSample (le lecteur lit du PCM s16 stéréo).
	for i := n; i+BytesPerSample <= len(p); i += BytesPerSample {
		copy(p[i:i+BytesPerSample], s.last[:])
	}
	return len(p), nil
}

// Buffered retourne le nombre d'octets PCM en attente (observabilité/tests).
func (s *Stream) Buffered() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.buf)
}

// BufferedSamples retourne le nombre d'échantillons (paires L/R) en attente.
// Sert à asservir la cadence d'émulation sur la consommation audio.
func (s *Stream) BufferedSamples() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.buf) / BytesPerSample
}
