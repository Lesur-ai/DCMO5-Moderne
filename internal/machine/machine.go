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
	"github.com/Lesur-ai/dcmo5/internal/keyboard"
	"github.com/Lesur-ai/dcmo5/internal/media"
)

// Key identifie une touche logique d'une machine. L'espace de touches dépend de la
// machine (58 pour le MO5, 84 pour la famille TO8/TO9) : il n'est donc pas figé ici.
type Key int

// JoystickInput décrit l'état instantané des deux manettes Thomson.
//
// Convention bits LOGIQUE INVERSÉE (commune MO5 et TO8D, ref C dcmo5emulation.c
// Joysemul cases 0-9 byte-identique à dcto8demulation.c) : un bit à 0 = touche
// APPUYÉE, un bit à 1 = touche RELÂCHÉE. Au repos, tous les bits sont à 1 (cf.
// NeutralJoystick).
//
// Position (8 bits, 1 par direction inversée) :
//
//	bit 0 = J1 nord (0=appuyé)   bit 4 = J2 nord
//	bit 1 = J1 sud               bit 5 = J2 sud
//	bit 2 = J1 ouest             bit 6 = J2 ouest
//	bit 3 = J1 est               bit 7 = J2 est
//
// Action (8 bits, seuls 6/7 utilisés) :
//
//	bit 6 = J1 bouton fire (0=appuyé)
//	bit 7 = J2 bouton fire
//	bits 0..5 = inutilisés (toujours 1 dans NeutralJoystick — bits sonores
//	            côté MO5/TO8D, OR'és par le hardware avec le canal son)
type JoystickInput struct {
	Position uint8 // axes des deux manettes (4 bits par manette, 0=appuyé)
	Action   uint8 // boutons d'action (bits 6/7, 0=appuyé)
}

// NeutralJoystick représente l'état joystick au repos (toutes touches relâchées,
// aucun bouton appuyé). À utiliser systématiquement comme valeur initiale dans
// les InputState : la zéro-value Go d'un struct vaudrait {0x00, 0x00}, qui
// signifierait « toutes les directions appuyées + boutons enfoncés » en logique
// inversée → cassure silencieuse côté machine. Toute construction d'InputState
// doit partir de NeutralJoystick.
var NeutralJoystick = JoystickInput{Position: 0xFF, Action: 0xC0}

// PointerKind distingue le crayon optique de la souris. Le MO5 n'a que le crayon ;
// la famille TO a les deux (traps 0x4B crayon, 0x4E/0x52 souris).
type PointerKind int

const (
	PointerPen   PointerKind = iota // crayon optique
	PointerMouse                    // souris (famille TO)
)

// PointerInput unifie crayon et souris (revue Codex : SetPen était MO5-centré).
// X/Y sont exprimés dans le repère du FRAMEBUFFER de la machine (0..w-1 / 0..h-1,
// bordures incluses), seul repère connu de l'UI via FrameSize()/Layout. Chaque
// machine convertit vers son repère interne dans SetPointer : le MO5 y retranche
// sa bordure pour obtenir l'écran actif du crayon ; la famille TO mappera de même
// (X peut alors atteindre 640 en mode 80 colonnes).
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

	// Clavier : modèle data-driven (nombre de touches, table caractère → touche,
	// indices des modificateurs). L'hôte en tire la taille de l'instantané
	// d'entrées (KeyCount variable selon la machine) ; l'UI s'en sert pour la
	// saisie live et l'injecteur. Cf. internal/keyboard.Model.
	KeyboardModel() *keyboard.Model
}
