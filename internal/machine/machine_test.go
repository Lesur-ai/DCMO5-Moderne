package machine

import "testing"

func TestRegistry(t *testing.T) {
	saved := registry
	t.Cleanup(func() { registry = saved })
	registry = nil

	// Enregistrement volontairement dans le désordre pour vérifier le tri par ID.
	Register(MachineProfile{ID: "to8d", Name: "Thomson TO8D", Family: FamilyTOGateArray})
	Register(MachineProfile{ID: "mo5", Name: "Thomson MO5", Family: FamilyMO})

	got := Profiles()
	if len(got) != 2 || got[0].ID != "mo5" || got[1].ID != "to8d" {
		t.Fatalf("Profiles() non triés par ID : %+v", got)
	}
	if p, ok := ByID("to8d"); !ok || p.Name != "Thomson TO8D" || p.Family != FamilyTOGateArray {
		t.Fatalf("ByID(to8d) = %+v, %v", p, ok)
	}
	if _, ok := ByID("absent"); ok {
		t.Fatal("ByID(absent) doit retourner false")
	}
}

func TestProfilesIsDeepCopy(t *testing.T) {
	saved := registry
	t.Cleanup(func() { registry = saved })
	registry = nil
	Register(MachineProfile{ID: "mo5", Params: []Param{
		{Key: "rom", FileExt: []string{".rom"}, Options: []Option{{Value: 1, Label: "a"}}},
	}})

	// Muter tous les niveaux du résultat ne doit pas corrompre le registre global.
	out := Profiles()
	out[0].ID = "muté"
	out[0].Params[0].Key = "muté"
	out[0].Params[0].FileExt[0] = ".muté"
	out[0].Params[0].Options[0].Label = "muté"

	again := Profiles()
	switch {
	case again[0].ID != "mo5":
		t.Errorf("ID corrompu : %q", again[0].ID)
	case again[0].Params[0].Key != "rom":
		t.Errorf("Param.Key corrompu : %q", again[0].Params[0].Key)
	case again[0].Params[0].FileExt[0] != ".rom":
		t.Errorf("Param.FileExt corrompu : %q", again[0].Params[0].FileExt[0])
	case again[0].Params[0].Options[0].Label != "a":
		t.Errorf("Param.Options corrompu : %q", again[0].Params[0].Options[0].Label)
	}

	// ByID retourne aussi une copie profonde.
	p, _ := ByID("mo5")
	p.Params[0].FileExt[0] = ".x"
	if again2, _ := ByID("mo5"); again2.Params[0].FileExt[0] != ".rom" {
		t.Errorf("ByID expose le registre : %q", again2.Params[0].FileExt[0])
	}
}

func TestIRQLines(t *testing.T) {
	var l IRQLines
	if l.Pending() {
		t.Fatal("IRQLines vide ne doit pas être pending")
	}
	l.Assert(IRQTimer)
	if !l.Pending() || !l.IsAsserted(IRQTimer) || l.IsAsserted(IRQFrame) {
		t.Fatalf("après Assert(IRQTimer) : pending=%v timer=%v frame=%v", l.Pending(), l.IsAsserted(IRQTimer), l.IsAsserted(IRQFrame))
	}
	// Niveau-déclenché : une seconde source coexiste, le clear est par source.
	l.Assert(IRQFrame)
	l.Clear(IRQTimer)
	if l.IsAsserted(IRQTimer) || !l.IsAsserted(IRQFrame) || !l.Pending() {
		t.Fatal("Clear(IRQTimer) ne doit relâcher que le timer")
	}
	l.Reset()
	if l.Pending() {
		t.Fatal("après Reset, aucune ligne ne doit être assertée")
	}
}
