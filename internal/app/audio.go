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

// initAudio installe le lecteur sur la ring du Host courant. Échec non fatal :
// l'émulation tourne sans son (le Host produit quand même, la ring déborde).
//
// Le contexte ebaudio est créé UNE SEULE FOIS et mémorisé (a.audioCtx) : ebiten
// n'autorise qu'un contexte par process (un 2e ebaudio.NewContext panique). Le Player,
// lui, est lié à la ring d'UN Host précis (AudioReader capture le pointeur du Host) :
// au changement de machine on le détruit (teardownAudio) puis on le recrée ici sur le
// NOUVEAU Host — d'où le re-appel d'initAudio après teardownAudio (audioPlayer == nil).
func (a *App) initAudio() {
	if a.audioDisabled || a.audioPlayer != nil {
		return
	}
	if a.audioCtx == nil {
		a.audioCtx = ebaudio.NewContext(spec.AudioSampleRate)
	}
	player, err := a.audioCtx.NewPlayer(a.host.AudioReader())
	if err != nil {
		return
	}
	player.SetBufferSize(audioBufferDuration)
	a.audioPlayer = player
	a.audioPlayer.Play()
}

// teardownAudio détruit le lecteur audio courant (lié à la ring de l'ancien Host) pour
// que initAudio en recrée un sur le nouveau Host. À appeler au changement de machine
// APRÈS host.Stop() (le lecteur ne doit plus lire pendant le démontage) et AVANT
// initAudio (qui est no-op tant que audioPlayer != nil). Le CONTEXTE (a.audioCtx) est
// conservé. No-op si l'audio est désactivé ou déjà absent.
func (a *App) teardownAudio() {
	if a.audioPlayer == nil {
		return
	}
	a.audioPlayer.Pause()
	a.audioPlayer.Close()
	a.audioPlayer = nil
}
