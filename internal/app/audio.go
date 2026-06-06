// Fichier : audio.go — sortie audio Ebitengine du son MO5.
// Le cœur produit des niveaux (core.DrainAudio) ; ce fichier les convertit en
// PCM (internal/audio.Stream) et les joue via Ebitengine.
package app

import (
	"time"

	dcaudio "github.com/Lesur-ai/dcmo5/internal/audio"
	"github.com/Lesur-ai/dcmo5/internal/spec"
	ebaudio "github.com/hajimehoshi/ebiten/v2/audio"
)

const (
	// defaultAudioGain amplifie l'écart au niveau central (registre 6 bits →
	// PCM s16). 31 (amplitude max) × gain reste sous 32767. Réglage du volume.
	defaultAudioGain = 600
	// audioBufferDuration : taille du tampon du lecteur. Court pour limiter la
	// latence (sinon le pré-remplissage de silence d'Oto retarde le son), mais
	// assez grand pour éviter les coupures entre deux frames (60 Hz ≈ 16 ms).
	audioBufferDuration = 50 * time.Millisecond
)

// DisableAudio coupe la sortie audio (à appeler avant Run). Utile sur une
// machine sans backend audio fonctionnel, ou via le flag CLI --no-audio.
func (a *App) DisableAudio() { a.audioDisabled = true }

// initAudio met en place le contexte audio, le flux PCM et le lecteur. En cas
// d'échec (pas de périphérique audio), l'émulation continue sans son. Appelé
// depuis Run (après que main a pu désactiver l'audio).
func (a *App) initAudio() {
	if a.audioDisabled || a.audioStream != nil {
		return
	}
	stream := dcaudio.NewStream(defaultAudioGain, spec.AudioSampleRate/2)
	a.audioBuf = make([]uint8, 2048)

	ctx := ebaudio.NewContext(spec.AudioSampleRate)
	player, err := ctx.NewPlayer(stream)
	if err != nil {
		return // pas de son, mais l'émulation tourne
	}
	// Tampon court : réduit la latence du son interactif.
	player.SetBufferSize(audioBufferDuration)
	a.audioStream = stream
	a.audioPlayer = player
	a.audioPlayer.Play()
}

// pumpAudio draine les échantillons produits par le cœur et les pousse dans le
// flux PCM. Appelé une fois par frame, après l'avance de l'émulation.
func (a *App) pumpAudio() {
	if a.audioStream == nil {
		return
	}
	for {
		n := a.machine.DrainAudio(a.audioBuf)
		if n == 0 {
			break
		}
		a.audioStream.Write(a.audioBuf[:n])
		if n < len(a.audioBuf) {
			break
		}
	}
}
