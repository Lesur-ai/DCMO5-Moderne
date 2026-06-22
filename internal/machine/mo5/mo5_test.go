package mo5_test

import (
	"path/filepath"
	"testing"

	"github.com/Lesur-ai/dcmo5/internal/core"
	"github.com/Lesur-ai/dcmo5/internal/machine"
	"github.com/Lesur-ai/dcmo5/internal/machine/mo5"
)

func TestProfileRegistered(t *testing.T) {
	p, ok := machine.ByID("mo5")
	if !ok {
		t.Fatal("profil mo5 non enregistré (init manquant ?)")
	}
	if p.Family != machine.FamilyMO {
		t.Errorf("Family = %v, want FamilyMO", p.Family)
	}
	if p.New == nil {
		t.Error("New nil")
	}
	var romRequired bool
	for _, pa := range p.Params {
		if pa.Key == mo5.ParamROM && pa.Kind == machine.ParamFile && pa.Required {
			romRequired = true
		}
	}
	if !romRequired {
		t.Error("paramètre ROM (File, requis) manquant dans le profil")
	}
}

func TestNewSatisfiesContract(t *testing.T) {
	m, err := mo5.New(core.Options{})
	if err != nil {
		t.Fatalf("New = %v", err)
	}
	if w, h := m.FrameSize(); w != core.FrameWidth || h != core.FrameHeight {
		t.Errorf("FrameSize = %dx%d, want %dx%d", w, h, core.FrameWidth, core.FrameHeight)
	}
	// Les entrées idempotentes ne doivent pas paniquer (y compris souris ignorée).
	m.SetKey(machine.Key(0), true)
	m.SetKey(machine.Key(0), false)
	m.SetJoystick(machine.JoystickInput{Position: 0xFF, Action: 0xC0})
	m.SetPointer(machine.PointerInput{Kind: machine.PointerPen, X: 10, Y: 20, Button: true})
	m.SetPointer(machine.PointerInput{Kind: machine.PointerMouse, X: 1, Y: 2}) // no-op sur MO5
	m.MountPrinter(nil)
	m.EjectPrinter()
	if n := m.Step(1000); n <= 0 {
		t.Errorf("Step(1000) = %d, attendu > 0", n)
	}
	_ = m.CPUSnapshot()

	// Le rendu doit remplir un framebuffer de la taille annoncée sans paniquer.
	w, h := m.FrameSize()
	m.FramebufferInto(make([]uint32, w*h))
}

func TestNewFromConfigEmpty(t *testing.T) {
	p, ok := machine.ByID("mo5")
	if !ok {
		t.Fatal("profil mo5 absent")
	}
	m, err := p.New(machine.Config{}) // sans ROM : core tolère, démarre en état indéfini
	if err != nil {
		t.Fatalf("New(Config vide) = %v", err)
	}
	if w, _ := m.FrameSize(); w != core.FrameWidth {
		t.Errorf("FrameSize w = %d", w)
	}
}

func TestNewFromConfigEnablesIOTrace(t *testing.T) {
	// Option A : profile.New(mo5) doit activer l'instrumentation E/S quand l'env la
	// demande, SANS cas spécial dans le CLI. La trace ne se déclenche que sur un trap
	// d'E/S : on en provoque un (imprimante 0x51) et on vérifie via le compteur
	// observable IOTraceCounts — sans dépendre du flush fichier ni d'un boot ROM.
	t.Setenv("DCMO5_IO_TRACE_FILE", filepath.Join(t.TempDir(), "iotrace.log"))
	p, ok := machine.ByID("mo5")
	if !ok {
		t.Fatal("profil mo5 absent")
	}
	m, err := p.New(machine.Config{})
	if err != nil {
		t.Fatalf("New = %v", err)
	}
	traced, ok := m.(interface {
		Entreesortie(io int)
		IOTraceCounts() map[int]int
	})
	if !ok {
		t.Fatal("la machine MO5 n'expose pas Entreesortie/IOTraceCounts")
	}
	traced.Entreesortie(0x51) // trap imprimante → compté si l'instrumentation est active
	if traced.IOTraceCounts()[0x51] == 0 {
		t.Error("profile.New(mo5) n'a pas activé l'IO-trace (trap non compté) — option A non respectée")
	}
}
