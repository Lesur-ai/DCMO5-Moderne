// Fichier : audio.go — sortie audio Ebitengine du son MO5.
// Le cœur produit des niveaux (core.DrainAudio) ; ce fichier les convertit en
// PCM (internal/audio.Stream) et les joue via Ebitengine.
package app

import (
	dcaudio "github.com/Lesur-ai/dcmo5/internal/audio"
	"github.com/Lesur-ai/dcmo5/internal/spec"
	ebaudio "github.com/hajimehoshi/ebiten/v2/audio"
)

// defaultAudioGain amplifie l'écart au niveau central (registre 6 bits → PCM
// s16). 31 (amplitude max) × gain reste sous 32767. Réglage du volume.
const defaultAudioGain = 600

// initAudio met en place le contexte audio, le flux PCM et le lecteur. En cas
// d'échec (pas de périphérique audio), l'émulation continue sans son.
func (a *App) initAudio() {
	if a.audioStream != nil {
		return
	}
	a.audioStream = dcaudio.NewStream(defaultAudioGain, spec.AudioSampleRate/2)
	a.audioBuf = make([]uint8, 2048)

	ctx := ebaudio.NewContext(spec.AudioSampleRate)
	player, err := ctx.NewPlayer(a.audioStream)
	if err != nil {
		a.audioStream = nil // pas de son, mais l'émulation tourne
		return
	}
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
