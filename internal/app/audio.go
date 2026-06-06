// Fichier : audio.go — sortie audio « audio-driven » du son MO5.
//
// Le thread audio pilote l'émulation : pour fournir N échantillons au backend,
// audioReader.Read fait avancer la machine juste ce qu'il faut et encode le son
// produit. L'émulation est ainsi cadencée par l'horloge audio (régulière),
// supprimant le gros tampon — donc la latence — sans réintroduire d'underrun.
//
// La machine étant touchée par deux threads (audio via Read, jeu via
// Update/Draw), tout accès passe par App.mu.
package app

import (
	"time"

	dcaudio "github.com/Lesur-ai/dcmo5/internal/audio"
	"github.com/Lesur-ai/dcmo5/internal/spec"
	ebaudio "github.com/hajimehoshi/ebiten/v2/audio"
)

const (
	// defaultAudioGain convertit le niveau (0..63) en amplitude PCM s16.
	// 63 × gain reste sous 32767 (pas d'écrêtage). Réglage du volume.
	defaultAudioGain = 480
	// audioBufferDuration : tampon du lecteur. En audio-driven, Read fournit
	// exactement ce qui est demandé (jamais de sous-alimentation), donc un
	// tampon court suffit → faible latence sans « tac ».
	audioBufferDuration = 40 * time.Millisecond
	// audioReadGuard borne le nombre d'itérations Step par Read (sécurité).
	audioReadGuard = 4096
)

// DisableAudio coupe la sortie audio (à appeler avant Run). En mode désactivé,
// l'émulation est pilotée par les frames (Update), pas par le thread audio.
func (a *App) DisableAudio() { a.audioDisabled = true }

// initAudio installe le lecteur audio-driven. Échec non fatal (émulation
// pilotée par frames). Appelé depuis Run (après un éventuel DisableAudio).
func (a *App) initAudio() {
	if a.audioDisabled || a.audioActive {
		return
	}
	ctx := ebaudio.NewContext(spec.AudioSampleRate)
	reader := &audioReader{app: a}
	player, err := ctx.NewPlayer(reader)
	if err != nil {
		return // pas de son ; Update pilotera l'émulation
	}
	player.SetBufferSize(audioBufferDuration)
	a.audioPlayer = player
	a.audioActive = true
	player.Play()
}

// audioReader implémente io.Reader pour Ebitengine : il pilote l'émulation à la
// demande du backend audio.
type audioReader struct {
	app    *App
	levels []uint8 // tampon de niveaux réutilisé entre les Read
	last   int16   // dernier échantillon encodé (maintien en pause)
}

// Read fournit du PCM s16 stéréo. Il fait tourner l'émulation (sous verrou) pour
// produire exactement le nombre d'échantillons demandés, puis les encode.
func (r *audioReader) Read(p []byte) (int, error) {
	need := len(p) / dcaudio.BytesPerSample
	if need == 0 {
		return 0, nil
	}
	if cap(r.levels) < need {
		r.levels = make([]uint8, need)
	}
	lv := r.levels[:need]
	got := 0

	a := r.app
	// En pause / menu ouvert, on n'avance pas l'émulation : silence.
	// emuPaused est atomique (écrit par le thread jeu, lu ici).
	if !a.emuPaused.Load() {
		a.mu.Lock()
		guard := 0
		for got < need {
			want := need - got
			cycles := want*spec.CPUClockHz/spec.AudioSampleRate + 64
			a.machine.Step(cycles)
			got += a.machine.DrainAudio(lv[got:])
			if guard++; guard >= audioReadGuard {
				break // sécurité (ne devrait jamais arriver)
			}
		}
		a.mu.Unlock()
	}

	for i := 0; i < need; i++ {
		var s int16
		if i < got {
			s = dcaudio.EncodeLevel(lv[i], defaultAudioGain)
			r.last = s
		}
		// En pause (got==0) ou complément : silence (0). r.last sert d'anti-clic
		// uniquement si on voulait maintenir ; ici le repos MO5 est déjà 0.
		dcaudio.PutStereoSample(p[i*dcaudio.BytesPerSample:], s)
	}
	return len(p), nil
}
