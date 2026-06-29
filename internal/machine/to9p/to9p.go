// Package to9p expose le Thomson TO9+ derrière le contrat internal/machine.
//
// Lot #186 : ce paquet pose seulement le profil minimal et le découpage ROM
// TO9+ (64 KiB BASIC/logiciels + 16 KiB moniteur) sur le Device gate-array
// partagé. Les points réellement spécifiques au TO9+ (clavier ASCII, patchs ROM
// complets, date/boot, signature de boot) restent aux lots suivants.
package to9p

import (
	"fmt"
	"os"

	"github.com/Lesur-ai/dcmo5/internal/cpu6809"
	"github.com/Lesur-ai/dcmo5/internal/engine"
	"github.com/Lesur-ai/dcmo5/internal/keyboard"
	"github.com/Lesur-ai/dcmo5/internal/machine"
	"github.com/Lesur-ai/dcmo5/internal/machine/gatearray"
	"github.com/Lesur-ai/dcmo5/internal/media"
)

// Découpage de la ROM TO9+ (rom/to9p.rom) : BASIC/logiciels 64 Ko en premier,
// moniteur 16 Ko ensuite. C'est le même conteneur 80 Ko que le TO8D, avec des
// tables de patch et un clavier qui divergeront dans les lots suivants.
const (
	romBasicSize = 0x10000
	romMonSize   = 0x4000
	romTotalSize = romBasicSize + romMonSize
)

type adapter struct {
	*engine.Engine
	ga *gatearray.GateArray
}

var _ machine.Machine = (*adapter)(nil)

func (a *adapter) Reset() {
	a.ga.Reset()
	a.Engine.Reset()
}

func (a *adapter) Initprog() { a.ga.Initprog() }

func (a *adapter) SetKey(k machine.Key, pressed bool) { a.ga.SetKey(int(k), pressed) }

func (a *adapter) SetJoystick(j machine.JoystickInput) {
	a.ga.SetJoystick(j.Position, j.Action)
}

func (a *adapter) SetPointer(p machine.PointerInput) {
	x, y := gatearray.PenFromFramebuffer(p.X, p.Y)
	a.ga.SetPointer(x, y, p.Button)
}

func (a *adapter) MountTape(t media.Tape)           { a.ga.MountTape(t) }
func (a *adapter) EjectTape()                       { a.ga.EjectTape() }
func (a *adapter) MountDisk(d media.Disk)           { a.ga.MountDisk(d) }
func (a *adapter) EjectDisk()                       { a.ga.EjectDisk() }
func (a *adapter) MountCartridge(c media.Cartridge) { a.ga.MountCartridge(c) }
func (a *adapter) EjectCartridge()                  { a.ga.EjectCartridge() }
func (a *adapter) MountPrinter(p media.PrinterSink) { a.ga.MountPrinter(p) }
func (a *adapter) EjectPrinter()                    { a.ga.EjectPrinter() }

func (a *adapter) CPUSnapshot() cpu6809.Snapshot { return a.Engine.CPU().Snapshot() }

// KeyboardModel reste temporairement TO8D : le clavier ASCII TO9+ est le Lot 3 de
// l'EPIC #184. Le Device gate-array lui-même est partagé ; ne pas interpréter ce
// modèle comme une validation du clavier TO9+.
func (a *adapter) KeyboardModel() *keyboard.Model { return keyboard.TO8DModel() }

func splitROM(blob []byte) (romBasic, romMon []byte, err error) {
	if len(blob) != romTotalSize {
		return nil, nil, fmt.Errorf("to9p: taille ROM inattendue %d octets (attendu %d : BASIC/logiciels %d + moniteur %d)",
			len(blob), romTotalSize, romBasicSize, romMonSize)
	}
	romBasic = append([]byte(nil), blob[:romBasicSize]...)
	romMon = append([]byte(nil), blob[romBasicSize:]...)
	return romBasic, romMon, nil
}

func newFromROM(blob []byte) (machine.Machine, error) {
	romBasic, romMon, err := splitROM(blob)
	if err != nil {
		return nil, err
	}
	if rep := applyROMPatches(romMon, romBasic); !rep.OK {
		// Inatteignable aujourd'hui : splitROM garantit les tailles exactes. Conservé
		// pour que les futurs patchs TO9+ puissent refuser une mutation non sûre.
		return nil, fmt.Errorf("to9p: invariant de découpage ROM violé avant patch")
	}

	ga := gatearray.New(romMon, romBasic)
	eng := engine.New(ga, 0)
	ga.AttachCPU(eng.CPU())
	ga.AttachBeam(eng.VideoBeam)

	a := &adapter{Engine: eng, ga: ga}
	a.Reset()
	return a, nil
}

func init() {
	machine.Register(machine.MachineProfile{
		ID:     "to9p",
		Name:   "Thomson TO9+",
		Family: machine.FamilyTOGateArray,
		Params: []machine.Param{
			{Key: machine.KeyROM, Label: "ROM TO9+ (BASIC/logiciels + moniteur)", Kind: machine.ParamFile, FileExt: []string{".rom"}, Required: true},
			{Key: machine.KeyTape, Label: "Cassette", Kind: machine.ParamFile, FileExt: []string{".k7"}, LiveMutable: true},
			{Key: machine.KeyDisk, Label: "Disquette", Kind: machine.ParamFile, FileExt: []string{".fd"}, LiveMutable: true},
			{Key: machine.KeyCart, Label: "Cartouche", Kind: machine.ParamFile, FileExt: []string{".rom"}, LiveMutable: true},
		},
		New: newFromConfig,
	})
}

func newFromConfig(cfg machine.Config) (machine.Machine, error) {
	path, _ := cfg[machine.KeyROM].(string)
	if path == "" {
		return nil, fmt.Errorf("to9p: ROM TO9+ requise (paramètre %q)", machine.KeyROM)
	}
	blob, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("to9p: ROM: %w", err)
	}
	return newFromROM(blob)
}
