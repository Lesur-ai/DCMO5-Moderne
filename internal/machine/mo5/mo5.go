// Package mo5 expose le Thomson MO5 (cœur internal/core) derrière le contrat
// internal/machine, SANS déplacer le cœur (lot 1 v2, #107).
//
// L'adaptation est minimale : *core.Machine fournit déjà, avec la bonne signature,
// Step/Reset/Initprog, FramebufferInto, l'audio, les médias k7/fd/cartouche et
// CPUSnapshot — ces méthodes sont promues par embedding. Seules les méthodes au type
// d'entrée différent (clavier/manette/pointeur), FrameSize et l'imprimante sont
// traduites ici.
package mo5

import (
	"fmt"
	"io"
	"os"

	"github.com/Lesur-ai/dcmo5/internal/core"
	"github.com/Lesur-ai/dcmo5/internal/keyboard"
	"github.com/Lesur-ai/dcmo5/internal/machine"
	"github.com/Lesur-ai/dcmo5/internal/media"
)

// adapter rend un *core.Machine MO5 conforme à machine.Machine.
type adapter struct {
	*core.Machine // promeut Step, Reset, Initprog, FramebufferInto, audio, médias, CPUSnapshot
}

// Vérification à la compilation que l'adaptateur satisfait le contrat.
var _ machine.Machine = (*adapter)(nil)

// SetKey traduit la touche logique machine en touche core (même espace d'indices).
func (a *adapter) SetKey(k machine.Key, pressed bool) {
	a.Machine.SetKey(core.Key(k), pressed)
}

// SetJoystick convertit l'état manettes (champs identiques).
func (a *adapter) SetJoystick(j machine.JoystickInput) {
	a.Machine.SetJoystick(core.JoystickInput{Position: j.Position, Action: j.Action})
}

// SetPointer mappe le pointeur unifié sur le crayon optique MO5. Le MO5 n'a pas de
// souris : les événements PointerMouse sont ignorés.
//
// p.X/p.Y arrivent dans le repère du framebuffer (seul repère connu de l'UI, via
// Layout). La conversion vers l'écran actif MO5 (retrait de la bordure) est faite
// ICI, côté machine, qui seule connaît sa géométrie : hors zone active, le cœur
// (readPenXY) signalera « pas de détection ».
func (a *adapter) SetPointer(p machine.PointerInput) {
	if p.Kind == machine.PointerPen {
		x, y := core.PenFromFramebuffer(p.X, p.Y)
		a.Machine.SetPen(x, y, p.Button)
	}
}

// FrameSize : le framebuffer MO5 est de taille fixe (constantes propres au MO5,
// cf. internal/core/mo5hw.go).
func (a *adapter) FrameSize() (int, int) { return core.FrameWidth, core.FrameHeight }

// KeyboardModel retourne le modèle clavier du MO5 (table caractère → touche +
// modificateurs + nombre de touches), consommé par l'hôte et l'UI.
func (a *adapter) KeyboardModel() *keyboard.Model { return keyboard.MO5Model() }

// MountPrinter / EjectPrinter : montage à chaud de la sortie imprimante (trap 0x51).
func (a *adapter) MountPrinter(p media.PrinterSink) { a.Machine.SetPrinter(p) }
func (a *adapter) EjectPrinter()                    { a.Machine.SetPrinter(nil) }

// New construit un MO5 conforme au contrat à partir d'options cœur déjà résolues
// (médias ouverts par l'appelant). Voie typée, sans accès fichier.
func New(opts core.Options) (machine.Machine, error) {
	m, err := core.NewMachine(opts)
	if err != nil {
		return nil, err
	}
	m.Reset()
	return &adapter{Machine: m}, nil
}

// Wrap habille un *core.Machine MO5 existant pour le contrat machine.Machine, sans
// le reconstruire. Utile quand l'appelant détient déjà le cœur — par exemple le CLI,
// qui configure des options non couvertes par le contrat (EnableIOTrace), ou les
// tests. Pour une construction standard depuis des options, préférer New.
func Wrap(m *core.Machine) machine.Machine {
	return &adapter{Machine: m}
}

// ── Profil MO5 (registre) ─────────────────────────────────────────────────────

// Clés des paramètres déclarés (consommées par le launcher/overlay).
const (
	ParamROM     = "rom"      // ROM système 16 Ko (requis)
	ParamDiskROM = "disk-rom" // ROM contrôleur CD90-640 (optionnel)
	ParamTape    = "tape"     // cassette .k7
	ParamDisk    = "disk"     // disquette .fd
	ParamCart    = "cart"     // cartouche .rom
)

func init() {
	machine.Register(machine.MachineProfile{
		ID:     "mo5",
		Name:   "Thomson MO5",
		Family: machine.FamilyMO,
		Params: []machine.Param{
			{Key: ParamROM, Label: "ROM système", Kind: machine.ParamFile, FileExt: []string{".rom"}, Required: true},
			{Key: ParamDiskROM, Label: "ROM contrôleur CD90-640", Kind: machine.ParamFile, FileExt: []string{".rom"}},
			{Key: ParamTape, Label: "Cassette", Kind: machine.ParamFile, FileExt: []string{".k7"}, LiveMutable: true},
			{Key: ParamDisk, Label: "Disquette", Kind: machine.ParamFile, FileExt: []string{".fd"}, LiveMutable: true},
			{Key: ParamCart, Label: "Cartouche", Kind: machine.ParamFile, FileExt: []string{".rom"}, LiveMutable: true},
		},
		New: newFromConfig,
	})
}

// IOTraceWriter résout la destination de la trace E/S (diagnostic) depuis
// l'environnement, ou nil si désactivée :
//   - DCMO5_IO_TRACE_FILE=<path> : journalise dans le fichier (ajout) ;
//   - DCMO5_IO_TRACE=<non vide>  : journalise sur stderr ;
//   - sinon                      : désactivé.
//
// Exporté pour que le CLI partage EXACTEMENT la même politique que profile.New :
// l'instrumentation n'est pas un cas spécial du CLI (option A, lot #117).
func IOTraceWriter() io.Writer {
	if path := os.Getenv("DCMO5_IO_TRACE_FILE"); path != "" {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			fmt.Fprintln(os.Stderr, "mo5: trace E/S:", err)
			return nil
		}
		return f
	}
	if os.Getenv("DCMO5_IO_TRACE") != "" {
		return os.Stderr
	}
	return nil
}

// ioErrorReporter est le puits par défaut des erreurs d'E/S MO5 (codes BASIC) pour les
// machines construites via le profil : impression sur stderr, PARITÉ avec le boot CLI
// (qui installe son propre Options.OnError). Variable de paquet pour la testabilité
// (override en test). Sans elle, les sessions lancées par le launcher perdaient toute
// remontée d'erreur E/S (cassette/disque), contrairement au CLI (#144).
var ioErrorReporter = func(code int) {
	fmt.Fprintf(os.Stderr, "mo5: erreur E/S %d (%s)\n", code, core.IOErrorLabel(code))
}

// newFromConfig résout les ROMs (chemins → octets) depuis la Config et construit le
// MO5. profile.New est AUTO-SUFFISANT (option A) : il applique lui-même
// l'instrumentation E/S (gating env via IOTraceWriter) ET la remontée d'erreurs E/S
// (OnError), avant Reset, exactement comme le CLI — le launcher générique en bénéficie
// sans cas spécial. Les médias (k7/fd/cart) sont montés à chaud par l'hôte/l'UI après
// création (MountTape/MountDisk/...), pas ici.
func newFromConfig(cfg machine.Config) (machine.Machine, error) {
	opts := core.Options{PatchSystemROM: true, OnError: ioErrorReporter}
	if p, _ := cfg[ParamROM].(string); p != "" {
		data, err := os.ReadFile(p)
		if err != nil {
			return nil, fmt.Errorf("mo5: ROM système: %w", err)
		}
		opts.ROMSys = data
	}
	if p, _ := cfg[ParamDiskROM].(string); p != "" {
		data, err := os.ReadFile(p)
		if err != nil {
			return nil, fmt.Errorf("mo5: ROM contrôleur disquette: %w", err)
		}
		opts.DiskControllerROM = data
	}
	m, err := core.NewMachine(opts)
	if err != nil {
		return nil, err
	}
	if w := IOTraceWriter(); w != nil {
		m.EnableIOTrace(w) // avant Reset : trace aussi les E/S du reset
	}
	m.Reset()
	return &adapter{Machine: m}, nil
}
