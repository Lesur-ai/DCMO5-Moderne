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

func TestProfilesIsCopy(t *testing.T) {
	saved := registry
	t.Cleanup(func() { registry = saved })
	registry = nil
	Register(MachineProfile{ID: "mo5"})

	// Muter la tranche retournée ne doit pas affecter le registre interne.
	out := Profiles()
	out[0].ID = "muté"
	if again := Profiles(); again[0].ID != "mo5" {
		t.Fatalf("Profiles() expose le registre interne : %q", again[0].ID)
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
