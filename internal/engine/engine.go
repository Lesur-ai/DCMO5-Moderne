// Package engine fournit la boucle d'émulation COMMUNE à toutes les machines
// Thomson : exécution CPU 6809, comptage de cycles, échantillonnage audio, cadence
// vidéo ligne/trame et IRQ de fin de trame. La partie spécifique à une machine
// (carte mémoire, traps d'E/S, timing périphériques, niveau audio, décodage vidéo)
// est fournie par un Device injecté.
//
// La boucle Step réplique fidèlement celle de internal/core (MO5 v1) : c'est le
// socle sur lequel le MO5 sera rebranché au lot suivant (sans régression), puis la
// famille gate-array (TO8D…). Ce paquet ne dépend ni de core ni d'une machine
// concrète : il est testable en isolation avec un Device synthétique.
//
// Ref : DESIGN/MACHINE_PROFILES.md §5 (moteur partagé + Device).
package engine

import (
	"github.com/Lesur-ai/dcmo5/internal/cpu6809"
	"github.com/Lesur-ai/dcmo5/internal/machine"
	"github.com/Lesur-ai/dcmo5/internal/spec"
)

// Timing vidéo Thomson (commun MO5 / famille TO) : 64 cycles par ligne, 312 lignes
// par trame → IRQ 50 Hz. Ref: dcmo5emulation.c / dcto8demulation.c Run().
const (
	cyclesPerLine = spec.VideoCyclesPerLine
	linesPerFrame = spec.VideoLinesPerFrame
)

// Device est la partie d'une machine que le moteur pilote : bus mémoire, dispatch
// des traps d'E/S, timing des périphériques par instruction, niveau audio courant et
// rendu vidéo. Une machine concrète (MO5, gate-array) implémente Device.
type Device interface {
	cpu6809.Bus // Read8/Write8 = carte mémoire de la machine

	// Trap dispatche un appel d'E/S (opcode illégal MO5/Thomson, code = -opcode).
	Trap(code int)

	// OnInstructionCycles est appelé après chaque instruction (c cycles consommés),
	// pour le timing des périphériques propres à la machine (timer 6846, IRQ clavier
	// de la famille TO). La machine asserte/relâche ses lignes via irq. No-op pour le
	// MO5 (sa seule source d'IRQ — la trame — est gérée par le moteur).
	OnInstructionCycles(c int, irq *machine.IRQLines)

	// SoundLevel retourne le niveau de haut-parleur courant (0..0x3F), échantillonné
	// par le moteur à la fréquence audio.
	SoundLevel() uint8

	// FrameSize retourne la taille (fixe) du framebuffer logique de la machine.
	FrameSize() (w, h int)

	// DecodeFrame rend le framebuffer courant dans dst (len ≥ w*h).
	DecodeFrame(dst []uint32)
}

// frameIRQSuppressor permet à un Device de désactiver l'IRQ de fin de trame 50 Hz
// générée par le moteur. Le MO5 (modèle par défaut) en a besoin : sa VBL est délivrée
// par cette IRQ de trame (dcmo5emulation.c). La famille gate-array (TO8D/TO9+) n'a PAS
// d'IRQ de trame — son interruption périodique vient du timer 6846 (cf.
// gatearray.OnInstructionCycles, dcto8demulation.c Run() : aucun dc6809_irq de trame).
// Un Device qui n'implémente pas cette interface conserve l'IRQ de trame (défaut MO5).
type frameIRQSuppressor interface {
	SuppressFrameIRQ() bool
}

type videoLineRenderer interface {
	RenderVideoLine(videolinenumber int)
}

// Engine exécute une machine via son Device. Il possède le CPU, l'accumulateur
// d'échantillonnage audio, les compteurs de balayage vidéo et les lignes d'IRQ.
type Engine struct {
	cpu *cpu6809.CPU
	dev Device
	irq machine.IRQLines

	audioSampleRate int
	sampleAccum     int64
	samples         []uint8

	videolinecycle  int
	videolinenumber int

	frameIRQ bool // false si le Device supprime l'IRQ de trame (famille gate-array)
}

// New crée un moteur pilotant dev. audioSampleRate ≤ 0 retombe sur le défaut spec.
func New(dev Device, audioSampleRate int) *Engine {
	if audioSampleRate <= 0 {
		audioSampleRate = spec.AudioSampleRate
	}
	e := &Engine{dev: dev, audioSampleRate: audioSampleRate, frameIRQ: true}
	if s, ok := dev.(frameIRQSuppressor); ok && s.SuppressFrameIRQ() {
		e.frameIRQ = false
	}
	e.cpu = cpu6809.New(dev)
	return e
}

// CPU expose le processeur (accès registres pour les handlers d'E/S de la machine).
func (e *Engine) CPU() *cpu6809.CPU { return e.cpu }

// IRQ expose les lignes d'interruption (pour les tests et l'observabilité).
func (e *Engine) IRQ() *machine.IRQLines { return &e.irq }

// VideoBeam expose la position courante du balayage vidéo (cycle dans la ligne,
// numéro de ligne). Le Device en a besoin pour ses registres de synchronisation
// faisceau (MO5 : ports 0xA7C3/D8/E6/E7 via Initn()/Iniln()). L'état reflète la
// fin de la dernière instruction exécutée, comme dcmo5emulation.c.
func (e *Engine) VideoBeam() (linecycle, linenumber int) {
	return e.videolinecycle, e.videolinenumber
}

// AudioSampleRate retourne le taux d'échantillonnage effectif.
func (e *Engine) AudioSampleRate() int { return e.audioSampleRate }

// Reset réinitialise le CPU (vecteur reset) puis tout le timing moteur (audio,
// compteurs vidéo, lignes d'IRQ). Le contenu mémoire/état de la machine reste à
// la charge du Device.
func (e *Engine) Reset() {
	e.cpu.Reset()
	e.ResetTiming()
}

// ResetTiming réinitialise le timing moteur SANS toucher au CPU : compteurs
// vidéo, état audio et lignes d'IRQ. Le Device l'utilise pour un reset matériel
// qui pilote le CPU séparément (ordre mémoire → vecteur reste à sa charge).
func (e *Engine) ResetTiming() {
	e.videolinecycle = 0
	e.videolinenumber = 0
	e.ResetAudio()
	e.irq.Reset()
}

// ResetAudio vide le tampon d'échantillons et l'accumulateur, sans toucher au
// CPU ni au balayage vidéo. Le Device l'utilise pour un reset « doux » (MO5
// Initprog) qui coupe le son sans réamorcer la trame en cours.
func (e *Engine) ResetAudio() {
	e.sampleAccum = 0
	e.samples = e.samples[:0]
}

// Step avance l'émulation d'au plus cycles et retourne les cycles consommés.
// Réplique exactement dcmo5emulation.c Run() / core.Machine.Step :
//   - opcode illégal (CPU.Step < 0) → Device.Trap(-code) + 64 cycles ;
//   - échantillonnage audio cadencé (audioSampleRate par CPUClockHz cycles) ;
//   - timing périphériques de la machine via Device.OnInstructionCycles ;
//   - cadence ligne (64 cy) / trame (312 lignes) → IRQ 50 Hz de fin de trame.
func (e *Engine) Step(cycles int) int {
	if cycles <= 0 {
		return 0
	}
	consumed := 0
	for consumed < cycles {
		c := e.cpu.Step()
		if c < 0 {
			e.dev.Trap(-c)
			c = 64
		} else if c == 0 {
			c = 2
		}
		consumed += c

		// Échantillonnage audio : audioSampleRate échantillons par CPUClockHz cycles.
		e.sampleAccum += int64(c) * int64(e.audioSampleRate)
		for e.sampleAccum >= int64(spec.CPUClockHz) {
			e.sampleAccum -= int64(spec.CPUClockHz)
			e.appendSample(e.dev.SoundLevel())
		}

		// Timing périphériques de la machine (6846, IRQ clavier TO ; MO5 = no-op).
		e.dev.OnInstructionCycles(c, &e.irq)

		// Livraison des lignes d'IRQ NIVEAU-déclenchées asserties par la machine
		// (timer/clavier de la famille TO). cpu.IRQ() honore le masque I : si I est
		// masqué l'IRQ est ignorée, mais la ligne RESTE assertée (le niveau persiste),
		// donc elle est reprise dès que le code démasque I — c'est précisément le cas
		// masqué-puis-démasqué que ce modèle doit gérer. Quand l'IRQ est prise, le CPU
		// masque I, évitant la ré-entrée tant que le handler n'a pas relâché la source
		// (puis RTI). MO5 n'asserte aucune ligne ici (sa trame est livrée plus bas) →
		// no-op, fidélité préservée.
		if e.irq.Pending() {
			e.cpu.IRQ()
		}

		// Cadence vidéo ligne/trame.
		e.videolinecycle += c
		for e.videolinecycle >= cyclesPerLine {
			e.videolinecycle -= cyclesPerLine
			if r, ok := e.dev.(videoLineRenderer); ok {
				r.RenderVideoLine(e.videolinenumber)
			}
			e.videolinenumber++
			if e.videolinenumber >= linesPerFrame {
				e.videolinenumber = 0
				if e.frameIRQ {
					e.cpu.IRQ() // IRQ 50 Hz de fin de trame (MO5 ; supprimée pour la famille gate-array)
				}
			}
		}

		if consumed >= cycles {
			break
		}
	}
	return consumed
}

// maxAudioBacklog borne le tampon d'échantillons à ~0,5 s (cf. core.Machine).
func (e *Engine) maxAudioBacklog() int { return e.audioSampleRate / 2 }

// appendSample ajoute un échantillon en respectant le plafond (glissement si saturé).
func (e *Engine) appendSample(level uint8) {
	if len(e.samples) >= e.maxAudioBacklog() {
		copy(e.samples, e.samples[1:])
		e.samples[len(e.samples)-1] = level
		return
	}
	e.samples = append(e.samples, level)
}

// DrainAudio copie les échantillons disponibles dans dst et vide le tampon interne.
// Retourne le nombre d'échantillons écrits (≤ len(dst)).
func (e *Engine) DrainAudio(dst []uint8) int {
	n := copy(dst, e.samples)
	if n >= len(e.samples) {
		e.samples = e.samples[:0]
	} else {
		rest := copy(e.samples, e.samples[n:])
		e.samples = e.samples[:rest]
	}
	return n
}

// AudioBacklog retourne le nombre d'échantillons en attente (observabilité).
func (e *Engine) AudioBacklog() int { return len(e.samples) }

// FrameSize retourne la taille du framebuffer logique de la machine.
func (e *Engine) FrameSize() (w, h int) { return e.dev.FrameSize() }

// FramebufferInto rend le framebuffer courant dans dst via le Device.
func (e *Engine) FramebufferInto(dst []uint32) { e.dev.DecodeFrame(dst) }
