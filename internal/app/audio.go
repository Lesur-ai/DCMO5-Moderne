// Fichier : audio.go — sortie audio Ebitengine.
//
// Le lecteur consomme la ring du Host (emu) : il ne touche jamais le cœur.
// Le contexte est créé au taux natif (48000 Hz, cf. spec) pour éviter le
// rééchantillonnage du backend.
package app

import (
	"time"

	"github.com/Lesur-ai/dcmo5/internal/spec"
	ebaudio "github.com/hajimehoshi/ebiten/v2/audio"
)

const (
	// defaultAudioGain convertit le niveau MO5 (0..63) en amplitude PCM s16.
	// 63 × gain reste sous 32767 (pas d'écrêtage).
	defaultAudioGain = 480
	// audioBufferDuration : tampon du lecteur. ≥ bloc backend (à 48000, le bloc
	// AudioQueue par défaut ≈ 32 ms) pour éviter les queues de zéros, tout en
	// restant faible pour la latence.
	audioBufferDuration = 50 * time.Millisecond
)

// DisableAudio coupe la sortie audio (à appeler avant Run).
func (a *App) DisableAudio() { a.audioDisabled = true }

// initAudio installe le lecteur sur la ring du Host. Échec non fatal :
// l'émulation tourne sans son (le Host produit quand même, la ring déborde).
func (a *App) initAudio() {
	if a.audioDisabled || a.audioPlayer != nil {
		return
	}
	ctx := ebaudio.NewContext(spec.AudioSampleRate)
	player, err := ctx.NewPlayer(a.host.AudioReader())
	if err != nil {
		return
	}
	player.SetBufferSize(audioBufferDuration)
	a.audioPlayer = player
	a.audioPlayer.Play()
}
