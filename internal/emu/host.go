// Package emu orchestre l'émulation en temps réel pour une interface graphique.
//
// Architecture (cf. analyse audio) : une goroutine dédiée possède la Machine et
// l'avance par quanta cadencés sur l'horloge murale. Elle publie le son dans une
// ring buffer (lue par le thread audio, simple consommateur) et un instantané du
// framebuffer (lu par le thread d'affichage). Les entrées (clavier/souris) sont
// publiées par l'UI via un instantané ; les changements de média (reset, montage)
// passent par un canal de commandes. Aucun de ces accès ne touche la Machine
// directement : seule la goroutine d'émulation le fait, ce qui évite tout verrou
// partagé sur le cœur et garde l'UI réactive.
//
// Ce package ne dépend d'aucune bibliothèque graphique : il est testable headless
// (y compris au détecteur de data races).
package emu

import (
	"io"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Lesur-ai/dcmo5/internal/audio"
	"github.com/Lesur-ai/dcmo5/internal/machine"
	"github.com/Lesur-ai/dcmo5/internal/media"
	"github.com/Lesur-ai/dcmo5/internal/spec"
)

const (
	// emuTickSleep : pause entre deux quanta de la boucle d'émulation. Court pour
	// alimenter l'audio finement ; le quantum réel dépend du temps écoulé.
	emuTickSleep = time.Millisecond
	// maxCatchupCyclesDiv borne le rattrapage après une pause/blocage : au plus
	// CPUClockHz/ce-diviseur cycles par quantum (≈100 ms), pour ne pas produire
	// une rafale d'audio/vidéo après un gel.
	maxCatchupCyclesDiv = 10
	// audioRingMaxMS : capacité de la ring audio (sécurité anti-dérive).
	audioRingMaxMS = 150
)

// InputState est l'instantané des entrées publié par l'UI et appliqué par la
// goroutine d'émulation avant chaque quantum.
type InputState struct {
	Keys    []bool // état des touches ; taille = KeyCount de la machine (variable)
	PenX    int
	PenY    int
	PenDown bool
}

type cmdKind int

const (
	cmdReset cmdKind = iota
	cmdInitprog
	cmdMountTape
	cmdEjectTape
	cmdMountDisk
	cmdEjectDisk
	cmdMountCart
	cmdEjectCart
)

type command struct {
	kind cmdKind
	tape media.Tape
	disk media.Disk
	cart media.Cartridge
}

// Host pilote une Machine en temps réel.
type Host struct {
	machine machine.Machine
	stream  *audio.Stream

	inputMu   sync.Mutex
	input     InputState // snapshot publié par l'UI (protégé par inputMu)
	inputKeys []bool     // buffer de travail de la goroutine (copie des touches)

	fbMu    sync.Mutex
	fbFront []uint32 // dernier framebuffer publié (lu par l'affichage)
	fbBack  []uint32 // tampon de rendu de la goroutine

	paused  atomic.Bool
	running atomic.Bool

	cmds chan command
	stop chan struct{}
	done chan struct{}

	drainBuf []uint8 // tampon de drainage audio réutilisé (goroutine)
}

// New crée un Host pour la machine donnée. gain règle le volume audio. Les tampons
// framebuffer sont dimensionnés selon FrameSize() de la machine (fixe par instance),
// et non plus selon des constantes MO5 : le Host est agnostique de la machine.
func New(m machine.Machine, gain int) *Host {
	sr := m.AudioSampleRate()
	fw, fh := m.FrameSize()
	keyCount := m.KeyboardModel().KeyCount
	h := &Host{
		machine:   m,
		stream:    audio.NewStream(gain, sr*audioRingMaxMS/1000),
		fbFront:   make([]uint32, fw*fh),
		fbBack:    make([]uint32, fw*fh),
		input:     InputState{Keys: make([]bool, keyCount)},
		inputKeys: make([]bool, keyCount),
		cmds:      make(chan command, 16),
		stop:      make(chan struct{}),
		done:      make(chan struct{}),
		drainBuf:  make([]uint8, sr/30+1),
	}
	m.FramebufferInto(h.fbFront) // image initiale
	return h
}

// AudioReader retourne l'io.Reader à passer au lecteur audio. Il ne touche
// jamais la Machine : en pause il renvoie du silence, sinon il lit la ring.
func (h *Host) AudioReader() io.Reader { return hostAudio{h} }

// hostAudio adapte le Host en source PCM consciente de la pause.
type hostAudio struct{ h *Host }

func (r hostAudio) Read(p []byte) (int, error) {
	if r.h.paused.Load() {
		for i := range p {
			p[i] = 0
		}
		return len(p), nil
	}
	return r.h.stream.Read(p)
}

// Start lance la goroutine d'émulation. Idempotent.
func (h *Host) Start() {
	if h.running.Swap(true) {
		return
	}
	go h.run()
}

// Stop arrête la goroutine et attend sa fin. Idempotent : un second appel (ou
// un appel sans Start préalable) ne fait rien.
func (h *Host) Stop() {
	if !h.running.Swap(false) {
		return // jamais démarré, ou déjà arrêté
	}
	close(h.stop)
	<-h.done
}

// SetInput publie l'instantané des entrées (appelé par l'UI, thread d'affichage).
func (h *Host) SetInput(in InputState) {
	h.inputMu.Lock()
	// Snapshot autonome : Keys est un slice ; stocker son header partagerait le
	// backing array avec l'appelant (data race avec la goroutine d'émulation). On
	// copie dans le buffer possédé par le Host (taille = KeyCount, pas de
	// réallocation). Les touches au-delà de l'instantané fourni sont relâchées
	// (sinon une touche pressée resterait coincée).
	h.input.PenX, h.input.PenY, h.input.PenDown = in.PenX, in.PenY, in.PenDown
	n := copy(h.input.Keys, in.Keys)
	for i := n; i < len(h.input.Keys); i++ {
		h.input.Keys[i] = false
	}
	h.inputMu.Unlock()
}

// SetPaused suspend/relance l'émulation. À l'entrée en pause, on vide la ring
// pour couper le son immédiatement (sinon le maintien anti-clic figerait le
// dernier niveau pendant toute la pause).
func (h *Host) SetPaused(p bool) {
	if p && !h.paused.Swap(true) {
		h.stream.Silence()
	} else if !p {
		h.paused.Store(false)
	}
}

// Paused indique l'état de pause.
func (h *Host) Paused() bool { return h.paused.Load() }

// Framebuffer copie le dernier framebuffer publié dans dst (thread d'affichage).
func (h *Host) Framebuffer(dst []uint32) {
	h.fbMu.Lock()
	copy(dst, h.fbFront)
	h.fbMu.Unlock()
}

// ── Commandes médias (exécutées par la goroutine propriétaire de la Machine) ──

func (h *Host) Reset()                           { h.send(command{kind: cmdReset}) }
func (h *Host) Initprog()                        { h.send(command{kind: cmdInitprog}) }
func (h *Host) MountTape(t media.Tape)           { h.send(command{kind: cmdMountTape, tape: t}) }
func (h *Host) EjectTape()                       { h.send(command{kind: cmdEjectTape}) }
func (h *Host) MountDisk(d media.Disk)           { h.send(command{kind: cmdMountDisk, disk: d}) }
func (h *Host) EjectDisk()                       { h.send(command{kind: cmdEjectDisk}) }
func (h *Host) MountCartridge(c media.Cartridge) { h.send(command{kind: cmdMountCart, cart: c}) }
func (h *Host) EjectCartridge()                  { h.send(command{kind: cmdEjectCart}) }

// send pousse une commande ; si la goroutine n'est pas démarrée, exécute en place
// (utile en test ou avant Start).
func (h *Host) send(c command) {
	if !h.running.Load() {
		h.execCommand(c)
		return
	}
	h.cmds <- c
}

// ── Boucle d'émulation (goroutine) ────────────────────────────────────────────

func (h *Host) run() {
	defer close(h.done)
	last := time.Now()
	fbAccum := 0
	overshoot := 0                   // cycles consommés en trop au quantum précédent (Step finit l'instruction/trap)
	fbPeriod := spec.CPUClockHz / 60 // publier le framebuffer ~60 fois/s
	maxCatchup := spec.CPUClockHz / maxCatchupCyclesDiv
	for {
		// Draine TOUTES les commandes en attente avant de cadencer : un lot de commandes
		// média appliquées ensemble (ex. overlay « Appliquer » : montage disquette +
		// éjection cartouche) est ainsi intégralement consommé AVANT qu'une frame ne tourne
		// — jamais d'état média partiel sur une frame (revue Codex). Les commandes sont
		// rares (pilotage utilisateur) : aucun risque de famine du stepping.
		for draining := true; draining; {
			select {
			case <-h.stop:
				return
			case c := <-h.cmds:
				h.execCommand(c)
			default:
				draining = false
			}
		}

		now := time.Now()
		elapsed := now.Sub(last)
		last = now

		if h.paused.Load() {
			overshoot = 0 // ne pas reporter de dette de temps à travers la pause
			time.Sleep(emuTickSleep)
			continue
		}

		// Cycles dus pour le temps écoulé, moins ce qui a déjà été consommé en
		// trop au quantum précédent (évite la dérive : l'émulation et l'audio
		// suivent l'horloge murale).
		cycles := int(elapsed.Nanoseconds()*int64(spec.CPUClockHz)/int64(time.Second)) - overshoot
		if cycles < 0 {
			cycles = 0
		}
		if cycles > maxCatchup {
			cycles = maxCatchup
		}
		if cycles > 0 {
			consumed := h.tick(cycles)
			overshoot = consumed - cycles // Step peut dépasser (instruction/trap entamés)
			if overshoot < 0 {
				overshoot = 0
			}
			fbAccum += consumed
			if fbAccum >= fbPeriod {
				fbAccum = 0
				h.publishFrame()
			}
		}
		time.Sleep(emuTickSleep)
	}
}

// tick applique les entrées, avance l'émulation de cycles cycles et pousse le son
// produit dans la ring. Retourne le nombre de cycles réellement consommés (Step
// peut dépasser la demande). Séparé de run() pour être testable de façon
// déterministe.
func (h *Host) tick(cycles int) int {
	in := h.snapshotInput()
	// Application des touches en DEUX passes : modificateurs d'abord, autres
	// touches ensuite. Sur TO8D le gate-array latch le scancode caractère AVEC
	// l'état modificateurs au moment du SetKey caractère (cf. gatearray/keyboard.go
	// to8key, bits 0x80 pour SHIFT et CNT inversé) : si SHIFT est posé APRÈS le
	// caractère, le latch capture l'état modificateur précédent. Sur MO5 (matrice
	// passive scannée par ROM), l'ordre est neutre — la boucle reste correcte.
	// Indices des modifs data-driven via KeyboardModel.ModifierKeys() : ajouter un
	// modificateur sur une machine future (FCT TO9+, p. ex.) le rend automatiquement
	// prioritaire ici sans toucher au host.
	model := h.machine.KeyboardModel()
	modKeys := model.ModifierKeys()
	isMod := make(map[int]struct{}, len(modKeys))
	for _, k := range modKeys {
		if k < 0 || k >= len(in.Keys) {
			continue
		}
		h.machine.SetKey(machine.Key(k), in.Keys[k])
		isMod[k] = struct{}{}
	}
	for k := 0; k < len(in.Keys); k++ {
		if _, ok := isMod[k]; ok {
			continue
		}
		h.machine.SetKey(machine.Key(k), in.Keys[k])
	}
	h.machine.SetPointer(machine.PointerInput{Kind: machine.PointerPen, X: in.PenX, Y: in.PenY, Button: in.PenDown})
	consumed := h.machine.Step(cycles)
	for {
		n := h.machine.DrainAudio(h.drainBuf)
		if n == 0 {
			break
		}
		h.stream.Write(h.drainBuf[:n])
		if n < len(h.drainBuf) {
			break
		}
	}
	return consumed
}

// publishFrame rend le framebuffer courant et l'échange avec l'instantané lu par
// l'affichage (double-buffer).
func (h *Host) publishFrame() {
	h.machine.FramebufferInto(h.fbBack)
	h.fbMu.Lock()
	h.fbFront, h.fbBack = h.fbBack, h.fbFront
	h.fbMu.Unlock()
}

func (h *Host) snapshotInput() InputState {
	h.inputMu.Lock()
	in := h.input
	copy(h.inputKeys, h.input.Keys) // copie le contenu dans le buffer de la goroutine
	h.inputMu.Unlock()
	in.Keys = h.inputKeys // la goroutine lit son propre buffer (pas le backing partagé)
	return in
}

func (h *Host) execCommand(c command) {
	switch c.kind {
	case cmdReset:
		h.machine.Reset()
	case cmdInitprog:
		h.machine.Initprog()
	case cmdMountTape:
		h.machine.MountTape(c.tape)
	case cmdEjectTape:
		h.machine.EjectTape()
	case cmdMountDisk:
		h.machine.MountDisk(c.disk)
	case cmdEjectDisk:
		h.machine.EjectDisk()
	case cmdMountCart:
		h.machine.MountCartridge(c.cart)
	case cmdEjectCart:
		h.machine.EjectCartridge()
	}
}
