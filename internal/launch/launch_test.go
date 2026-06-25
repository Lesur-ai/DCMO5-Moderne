package launch_test

import (
	"testing"

	"github.com/Lesur-ai/dcmo5/internal/launch"
	"github.com/Lesur-ai/dcmo5/internal/machine"
)

// TestDirectBoot : le boot direct (contournement du launcher) n'a lieu QUE si --rom
// ou --exec est fourni explicitement. « dcmo5 » seul (aucun flag) → launcher, même si
// une ROM est mémorisée en config (la décision n'utilise pas le fallback config).
func TestDirectBoot(t *testing.T) {
	cases := []struct {
		rom, exec, want bool
	}{
		{false, false, false}, // aucun flag → launcher
		{true, false, true},   // --rom → boot direct
		{false, true, true},   // --exec → boot direct
		{true, true, true},    // les deux → boot direct
	}
	for _, c := range cases {
		if got := launch.DirectBoot(c.rom, c.exec); got != c.want {
			t.Errorf("DirectBoot(rom=%v, exec=%v) = %v, want %v", c.rom, c.exec, got, c.want)
		}
	}
}

// TestDemoProfile_NotInstantiable : le profil démo couvre les 4 ParamKind et n'est pas
// instanciable (New renvoie une erreur) — « Démarrer » sert de test visuel du chemin
// d'erreur sans crash.
func TestDemoProfile_NotInstantiable(t *testing.T) {
	p := launch.DemoProfile()
	if len(p.Params) != 4 {
		t.Fatalf("DemoProfile : %d params, want 4 (Enum+Bool+Int+File)", len(p.Params))
	}
	if _, err := p.New(nil); err == nil {
		t.Error("DemoProfile.New doit renvoyer une erreur (non instanciable)")
	}
}

// TestSelectIndex : --machine présélectionne le profil au launcher ; un ID inconnu
// EXPLICITE est une erreur (parité boot direct), un défaut inconnu retombe sur le profil 0.
func TestSelectIndex(t *testing.T) {
	profiles := []machine.MachineProfile{{ID: "mo5"}, {ID: "to8d"}}

	if i, err := launch.SelectIndex(profiles, "to8d", true); err != nil || i != 1 {
		t.Errorf("SelectIndex(to8d, explicit) = (%d, %v), want (1, nil)", i, err)
	}
	if i, err := launch.SelectIndex(profiles, "mo5", false); err != nil || i != 0 {
		t.Errorf("SelectIndex(mo5, défaut) = (%d, %v), want (0, nil)", i, err)
	}
	if _, err := launch.SelectIndex(profiles, "m05", true); err == nil {
		t.Error("SelectIndex(m05, explicit) doit renvoyer une erreur (machine inconnue)")
	}
	if i, err := launch.SelectIndex(profiles, "inconnu", false); err != nil || i != 0 {
		t.Errorf("SelectIndex(inconnu, défaut) = (%d, %v), want (0, nil) — défaut tolérant", i, err)
	}
}
