// Package machine définit le contrat commun à toutes les machines Thomson émulées
// (MO5, TO8D, TO9+, …) : l'interface runtime Machine pilotée par l'hôte et l'UI, les
// types d'entrées, le descripteur MachineProfile et son registre.
//
// Règle de dépendance (anti-cycle) : les paquets de machines concrètes importent
// `machine` ; `machine` n'importe AUCUNE machine concrète ni `internal/core`.
//
// Ref : DESIGN/MACHINE_PROFILES.md (§4) — contrat issu de l'audit TO8D/TO9+ et de la
// revue de conception Codex (entrées idempotentes, FrameSize fixe par machine).
package machine

import (
	"github.com/Lesur-ai/dcmo5/internal/cpu6809"
	"github.com/Lesur-ai/dcmo5/internal/media"
)

// Key identifie une touche logique d'une machine. L'espace de touches dépend de la
// machine (58 pour le MO5, 84 pour la famille TO8/TO9) : il n'est donc pas figé ici.
type Key int

// JoystickInput décrit l'état instantané des deux manettes.
type JoystickInput struct {
	Position uint8 // axes des deux manettes (4 bits par manette)
	Action   uint8 // boutons d'action
}

// PointerKind distingue le crayon optique de la souris. Le MO5 n'a que le crayon ;
// la famille TO a les deux (traps 0x4B crayon, 0x4E/0x52 souris).
type PointerKind int

const (
	PointerPen   PointerKind = iota // crayon optique
	PointerMouse                    // souris (famille TO)
)

// PointerInput unifie crayon et souris (revue Codex : SetPen était MO5-centré).
// X/Y sont exprimés dans le repère écran actif de la machine ; X peut atteindre 640
// en mode 80 colonnes sur la famille TO.
type PointerInput struct {
	Kind   PointerKind
	X, Y   int
	Button bool // bouton crayon / clic souris
}

// Machine est le contrat runtime piloté par l'hôte (internal/emu.Host) et l'UI,
// indépendamment du modèle Thomson émulé.
//
// Sémantique des entrées (revue Codex, bloquant) : SetKey/SetJoystick/SetPointer
// publient un ÉTAT idempotent, réappliqué à chaque tick par l'hôte. La machine
// détecte elle-même les TRANSITIONS d'appui — indispensable au clavier TO8D qui émet
// scancode + IRQ sur front (sinon rafale d'IRQ). Les lignes d'interruption sont
// internes au moteur : le contrat n'expose donc pas d'IRQ().
//
// Vidéo : FrameSize est CONSTANT pour une instance de machine (336×216 pour le MO5,
// 672×216 pour la famille TO). Les modes vidéo sont des résolutions de décodage dans
// cette frame logique, pas un redimensionnement runtime. L'hôte dimensionne ses
// tampons au moment du New() de la machine.
type Machine interface {
	// Exécution
	Step(cycles int) int // avance d'au plus cycles, retourne les cycles consommés
	Reset()              // reset matériel (efface la RAM)
	Initprog()           // reset doux (RAM conservée)

	// Entrées (état idempotent ; transitions détectées par la machine)
	SetKey(k Key, pressed bool)
	SetJoystick(j JoystickInput)
	SetPointer(p PointerInput)

	// Vidéo (FrameSize fixe par instance)
	FrameSize() (w, h int)
	FramebufferInto(dst []uint32) // rend dans dst (len ≥ w*h)

	// Audio
	AudioSampleRate() int
	DrainAudio(dst []uint8) int

	// Médias à chaud
	MountTape(media.Tape)
	EjectTape()
	MountDisk(media.Disk)
	EjectDisk()
	MountCartridge(media.Cartridge)
	EjectCartridge()
	MountPrinter(media.PrinterSink)
	EjectPrinter()

	// Observabilité
	CPUSnapshot() cpu6809.Snapshot
}
