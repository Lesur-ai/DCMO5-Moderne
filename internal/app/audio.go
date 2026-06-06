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
	// defaultAudioGain convertit le niveau (0..63) en amplitude PCM s16.
	// 63 × gain reste sous 32767 (pas d'écrêtage). Réglage du volume.
	defaultAudioGain = 480
	// audioBufferDuration : tampon du lecteur. ~120 ms est le minimum stable sur
	// les machines testées : en dessous, le backend se vide périodiquement et
	// glitche (« tac » au repos, son haché pendant les bips). La latence qui en
	// résulte sera réduite par une refonte « audio-driven » (cf. issue).
	audioBufferDuration = 120 * time.Millisecond
)

// Cadence audio (dynamic rate control). On vise une petite réserve constante
// d'échantillons dans le flux : assez pour ne pas se vider entre deux frames
// (sinon « tac » périodique), assez peu pour que le son des frappes reste
// synchrone (faible latence).
const (
	audioSamplesPerFrame = spec.AudioSampleRate / 60 // production nominale/frame
	audioTargetSamples   = audioSamplesPerFrame * 7  // réserve cible (~117 ms, alignée sur le tampon lecteur)
	audioMinFrameSamples = audioSamplesPerFrame / 2  // plancher (garde la vidéo fluide)
	audioMaxFrameSamples = audioSamplesPerFrame * 4  // plafond modéré (évite le « brouillé » du varispeed)
)

// audioPacedCycles retourne le nombre de cycles CPU à exécuter cette frame pour
// maintenir le tampon audio autour de audioTargetSamples. Asservit la vitesse
// d'émulation sur l'horloge audio (plus stable que le vsync).
func (a *App) audioPacedCycles() int {
	backlog := a.audioStream.BufferedSamples()
	// Échantillons à produire = combler l'écart à la cible + une frame nominale.
	want := audioTargetSamples - backlog + audioSamplesPerFrame
	if want < audioMinFrameSamples {
		want = audioMinFrameSamples
	} else if want > audioMaxFrameSamples {
		want = audioMaxFrameSamples
	}
	return want * spec.CPUClockHz / spec.AudioSampleRate
}

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
