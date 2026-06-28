// Package to8d expose le Thomson TO8D derrière le contrat internal/machine, en
// assemblant le Device gate-array (internal/machine/gatearray) et la boucle commune
// (internal/engine), sans rien réécrire du cœur.
//
// Le cœur d'émulation TO8D — mémoire 512 Ko + banking, vidéo 5 modes + palette
// EF9369, timer 6846 + IRQ, traps d'E/S, clavier — vit déjà dans gatearray (lots
// #112-#116). Ce paquet n'ajoute que : (1) l'enveloppe machine.Machine, (2) le
// chargement et le découpage de la ROM TO8D (rom/to8d.rom), (3) le patch « trap »
// des ROM (rompatch.go), (4) l'enregistrement du profil au registre des machines.
//
// Règle de dépendance : to8d importe machine/gatearray/engine/keyboard/media ;
// machine n'importe AUCUNE machine concrète (cf. internal/machine/machine.go).
//
// Périmètre #118-minimal : boot du moniteur visible + sélection launcher / --machine
// to8d. Le boot direct --rom (chemin v1) reste MO5 ; le joystick et l'injection de
// date au reset sont hors périmètre (cf. SetJoystick et rompatch.go).
package to8d

import (
	"fmt"
	"os"
	"time"

	"github.com/Lesur-ai/dcmo5/internal/cpu6809"
	"github.com/Lesur-ai/dcmo5/internal/engine"
	"github.com/Lesur-ai/dcmo5/internal/keyboard"
	"github.com/Lesur-ai/dcmo5/internal/machine"
	"github.com/Lesur-ai/dcmo5/internal/machine/gatearray"
	"github.com/Lesur-ai/dcmo5/internal/media"
)

// Découpage de la ROM TO8D (rom/to8d.rom) : BASIC 64 Ko EN PREMIER, moniteur 16 Ko
// ENSUITE. Ordre VÉRIFIÉ octet-à-octet contre les tableaux bruts to8dbasic[] /
// to8dmoniteur[] de la référence C (dcto8d200905) ; cf. TestSplitROMMatchesReference.
const (
	romBasicSize = 0x10000                   // BASIC interne (4 banques de 16 Ko)
	romMonSize   = 0x4000                    // moniteur système (2 banques de 8 Ko)
	romTotalSize = romBasicSize + romMonSize // 81920 octets
)

// adapter rend la paire {gate-array + moteur} conforme à machine.Machine.
//
// L'embedding de *engine.Engine promeut Step, FrameSize, FramebufferInto, DrainAudio
// et AudioSampleRate (signatures identiques au contrat). Reset est redéfini ci-dessous
// (l'embedding fournit le reset CPU, mais pas celui du gate-array). Les méthodes au
// type d'entrée propre au contrat (clavier/manette/pointeur), les médias, Initprog,
// CPUSnapshot et KeyboardModel sont fournis ici via le gate-array.
type adapter struct {
	*engine.Engine
	ga *gatearray.GateArray
}

// Vérification à la compilation que l'adaptateur satisfait le contrat.
var _ machine.Machine = (*adapter)(nil)

// Reset effectue un reset MATÉRIEL. On réamorce d'abord l'état du gate-array
// (RAM/ports/banques) afin que le bus présente le bon vecteur reset, PUIS le CPU et
// le timing moteur le relisent (engine.Reset = cpu.Reset + reset timing).
func (a *adapter) Reset() {
	a.ga.Reset()     // hardReset gate-array : banques prêtes
	a.Engine.Reset() // cpu.Reset() lit 0xFFFE via le bus, puis reset du timing
}

// Initprog effectue un reset DOUX (RAM et ports conservés) : le gate-array recalcule
// ses banques et recharge le vecteur reset du CPU.
func (a *adapter) Initprog() { a.ga.Initprog() }

// SetKey traduit la touche logique machine en scancode TO8D (même espace d'indices
// que le modèle clavier, cf. keyboard.TO8DModel). La détection de transition (front)
// est faite par le gate-array, indispensable à l'IRQ clavier.
func (a *adapter) SetKey(k machine.Key, pressed bool) { a.ga.SetKey(int(k), pressed) }

// SetJoystick propage l'état des deux manettes au gate-array (Inc J1a). Le
// gate-array publie ces valeurs sur 0xe7cc/0xe7cd quand port[0x0e/0x0f] bit2
// sélectionne le mode joystick (mux hardware). Convention bits LOGIQUE INVERSÉE
// (0=appuyé) ; repos = machine.NeutralJoystick = {0xFF, 0xC0}. La couche hôte
// (Host.tick, Inc J2a) doit construire ses InputState à partir de
// machine.NeutralJoystick pour éviter qu'une zéro-value Go ({0x00, 0x00}) ne
// soit interprétée comme « toutes directions appuyées ».
func (a *adapter) SetJoystick(j machine.JoystickInput) {
	a.ga.SetJoystick(j.Position, j.Action)
}

// SetPointer mappe le pointeur unifié (repère framebuffer) vers l'écran actif TO8D.
// Le gate-array attend des coordonnées écran (x∈[0,639], y∈[0,199]) : la conversion
// (retrait de la bordure) est déléguée à gatearray.PenFromFramebuffer, qui seule
// connaît la géométrie. Crayon ET souris partagent ce repère côté famille TO.
func (a *adapter) SetPointer(p machine.PointerInput) {
	x, y := gatearray.PenFromFramebuffer(p.X, p.Y)
	a.ga.SetPointer(x, y, p.Button)
}

// Médias à chaud : délégués au gate-array.
func (a *adapter) MountTape(t media.Tape)           { a.ga.MountTape(t) }
func (a *adapter) EjectTape()                       { a.ga.EjectTape() }
func (a *adapter) MountDisk(d media.Disk)           { a.ga.MountDisk(d) }
func (a *adapter) EjectDisk()                       { a.ga.EjectDisk() }
func (a *adapter) MountCartridge(c media.Cartridge) { a.ga.MountCartridge(c) }
func (a *adapter) EjectCartridge()                  { a.ga.EjectCartridge() }
func (a *adapter) MountPrinter(p media.PrinterSink) { a.ga.MountPrinter(p) }
func (a *adapter) EjectPrinter()                    { a.ga.EjectPrinter() }

// CPUSnapshot expose l'état CPU (observabilité/tests), via le CPU du moteur.
func (a *adapter) CPUSnapshot() cpu6809.Snapshot { return a.Engine.CPU().Snapshot() }

// KeyboardModel retourne le modèle clavier TO8D (table caractère → touche, indices
// des modificateurs, nombre de touches), consommé par l'hôte et l'UI.
func (a *adapter) KeyboardModel() *keyboard.Model { return keyboard.TO8DModel() }

// newFromROM découpe le blob TO8D (BASIC + moniteur), patche les deux ROM, injecte la
// date de boot (now) dans le BASIC puis assemble gate-array + moteur. Cœur partagé par
// le profil (fichier) et les tests. now est la date pré-remplie au boot (jj-mm-aa) :
// le chemin de production passe time.Now() ; les tests passent une date fixe pour un
// boot déterministe.
func newFromROM(blob []byte, now time.Time) (machine.Machine, error) {
	if len(blob) != romTotalSize {
		return nil, fmt.Errorf("to8d: taille ROM inattendue %d octets (attendu %d : BASIC %d + moniteur %d)",
			len(blob), romTotalSize, romBasicSize, romMonSize)
	}
	// Copies possédées (le blob de l'appelant n'est pas muté ; les patchs restent en
	// mémoire). BASIC d'abord, moniteur ensuite (ordre vérifié contre la réf C).
	romBasic := append([]byte(nil), blob[:romBasicSize]...)
	romMon := append([]byte(nil), blob[romBasicSize:]...)

	if rep := applyPatches(romMon, monitorPatches); !rep.OK {
		return nil, fmt.Errorf("to8d: ROM moniteur non reconnue (patchs « trap » inapplicables)")
	}
	if rep := applyPatches(romBasic, basicPatches); !rep.OK {
		return nil, fmt.Errorf("to8d: ROM BASIC non reconnue (patchs « trap » inapplicables)")
	}
	// Date du jour pré-remplie au boot (jj-mm-aa), comme l'émulateur de référence.
	// applyPatches a déjà validé cette même ROM BASIC : un échec ici signale une
	// incohérence de variante (slot date / routine reset inattendus).
	if !injectBootDate(romBasic, now) {
		return nil, fmt.Errorf("to8d: injection de la date au boot impossible (ROM BASIC non reconnue)")
	}

	ga := gatearray.New(romMon, romBasic)
	eng := engine.New(ga, 0) // 0 → fréquence audio par défaut (spec)
	ga.AttachCPU(eng.CPU())
	ga.AttachBeam(eng.VideoBeam) // registres de synchro faisceau e7e7/e7ca (boot)

	a := &adapter{Engine: eng, ga: ga}
	a.Reset()
	return a, nil
}

// ── Profil TO8D (registre) ─────────────────────────────────────────────────────

func init() {
	machine.Register(machine.MachineProfile{
		ID:     "to8d",
		Name:   "Thomson TO8D",
		Family: machine.FamilyTOGateArray,
		Params: []machine.Param{
			{Key: machine.KeyROM, Label: "ROM TO8D (BASIC + moniteur)", Kind: machine.ParamFile, FileExt: []string{".rom"}, Required: true},
			{Key: machine.KeyTape, Label: "Cassette", Kind: machine.ParamFile, FileExt: []string{".k7"}, LiveMutable: true},
			{Key: machine.KeyDisk, Label: "Disquette", Kind: machine.ParamFile, FileExt: []string{".fd"}, LiveMutable: true},
			{Key: machine.KeyCart, Label: "Cartouche", Kind: machine.ParamFile, FileExt: []string{".rom"}, LiveMutable: true},
		},
		New: newFromConfig,
	})
}

// newFromConfig résout la ROM TO8D (chemin → octets) depuis la Config et construit la
// machine. Les médias (k7/fd/cart) sont montés à chaud par l'hôte/l'UI après création
// (MountTape/MountDisk/...), comme pour le MO5 — pas ici.
func newFromConfig(cfg machine.Config) (machine.Machine, error) {
	path, _ := cfg[machine.KeyROM].(string)
	if path == "" {
		return nil, fmt.Errorf("to8d: ROM TO8D requise (paramètre %q)", machine.KeyROM)
	}
	blob, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("to8d: ROM: %w", err)
	}
	return newFromROM(blob, time.Now())
}
