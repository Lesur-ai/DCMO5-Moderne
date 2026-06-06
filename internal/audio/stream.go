package audio

import "sync"

// Stream est une file FIFO thread-safe (un producteur, un consommateur) entre la
// goroutine d'émulation (Write des niveaux MO5) et le thread audio (Read du PCM).
// En sous-alimentation, Read MAINTIENT le dernier échantillon (anti-clic) et ne
// retourne jamais io.EOF. Le tampon est borné : au-delà, les plus anciens
// échantillons sont abandonnés pour préserver la latence.
type Stream struct {
	mu   sync.Mutex
	buf  []byte
	gain int
	max  int     // capacité max en octets (0 = illimité)
	last [4]byte // dernier échantillon stéréo écrit (maintenu si vide)
}

// NewStream crée un flux. gain règle le volume ; maxSamples borne le tampon.
func NewStream(gain, maxSamples int) *Stream {
	return &Stream{gain: gain, max: maxSamples * BytesPerSample}
}

// Write encode des niveaux MO5 (0..63) en PCM s16 stéréo et les met en file.
func (s *Stream) Write(levels []uint8) {
	if len(levels) == 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, lv := range levels {
		v := EncodeLevel(lv, s.gain)
		lo, hi := byte(v), byte(v>>8)
		s.buf = append(s.buf, lo, hi, lo, hi)
		s.last = [4]byte{lo, hi, lo, hi}
	}
	if s.max > 0 && len(s.buf) > s.max {
		drop := len(s.buf) - s.max
		rest := copy(s.buf, s.buf[drop:])
		s.buf = s.buf[:rest]
	}
}

// Read fournit du PCM au backend ; complète par maintien du dernier échantillon.
func (s *Stream) Read(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := copy(p, s.buf)
	rest := copy(s.buf, s.buf[n:])
	s.buf = s.buf[:rest]
	// Compléter octet par octet en répétant le dernier échantillon. n est un
	// multiple de BytesPerSample (la file ne contient que des frames complètes,
	// et n vaut soit tout p, soit toute la file), donc i%BytesPerSample garde la
	// phase L/R correcte même si len(p) n'est pas un multiple de la frame.
	for i := n; i < len(p); i++ {
		p[i] = s.last[i%BytesPerSample]
	}
	return len(p), nil
}

// Silence vide la file et réinitialise le maintien : Read renverra du vrai
// silence (et non le dernier échantillon) jusqu'à la prochaine écriture. Utilisé
// à l'entrée en pause pour couper net le son sans laisser un ton figé.
func (s *Stream) Silence() {
	s.mu.Lock()
	s.buf = s.buf[:0]
	s.last = [4]byte{}
	s.mu.Unlock()
}

// Buffered retourne le nombre d'octets PCM en attente (observabilité).
func (s *Stream) Buffered() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.buf)
}

// BufferedSamples retourne le nombre d'échantillons (paires L/R) en attente.
func (s *Stream) BufferedSamples() int { return s.Buffered() / BytesPerSample }
