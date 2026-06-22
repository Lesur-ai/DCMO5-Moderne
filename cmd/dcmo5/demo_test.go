package main

import "testing"

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
		if got := directBoot(c.rom, c.exec); got != c.want {
			t.Errorf("directBoot(rom=%v, exec=%v) = %v, want %v", c.rom, c.exec, got, c.want)
		}
	}
}

// TestDemoProfile_NotInstantiable : le profil démo n'est pas instanciable (New
// renvoie une erreur) — « Démarrer » sert de test visuel du chemin d'erreur sans crash.
func TestDemoProfile_NotInstantiable(t *testing.T) {
	p := demoProfile()
	if len(p.Params) != 4 {
		t.Fatalf("demoProfile : %d params, want 4 (Enum+Bool+Int+File)", len(p.Params))
	}
	if _, err := p.New(nil); err == nil {
		t.Error("demoProfile.New doit renvoyer une erreur (non instanciable)")
	}
}
