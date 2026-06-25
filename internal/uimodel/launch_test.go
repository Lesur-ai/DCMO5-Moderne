package uimodel_test

import (
	"path/filepath"
	"testing"

	"github.com/Lesur-ai/dcmo5/internal/machine"
	"github.com/Lesur-ai/dcmo5/internal/uimodel"
)

// otherProfile : second profil aux clés DISJOINTES de fakeProfile (pour prouver
// qu'InitialValues ne laisse fuir aucune valeur d'un profil vers l'autre).
func otherProfile() machine.MachineProfile {
	return machine.MachineProfile{
		ID: "other", Name: "Other", Family: machine.FamilyTOGateArray,
		Params: []machine.Param{
			{Key: "moniteur", Label: "Moniteur", Kind: machine.ParamFile, FileExt: []string{".rom"}, Required: true}, // boot-only
			{Key: "couleur", Label: "Couleur", Kind: machine.ParamBool, Default: true},                               // Default non nil
			{Key: "memo", Label: "Memo", Kind: machine.ParamFile, FileExt: []string{".rom"}, LiveMutable: true},      // média live
		},
		New: func(machine.Config) (machine.Machine, error) { return nil, nil },
	}
}

// TestMediaMounts ne retient que les Params File ET LiveMutable à valeur non vide,
// dans l'ordre des Params : tape (live) oui ; rom (File mais boot-only) non ; turbo
// (LiveMutable mais Bool) non ; un média live à valeur vide non plus.
func TestMediaMounts(t *testing.T) {
	p := fakeProfile()
	cfg := machine.Config{
		"rom":   "/roms/mo5.rom", // File boot-only → exclu (consommé par New)
		"tape":  "/k7/jeu.k7",    // File live, non vide → inclus
		"turbo": true,            // LiveMutable mais Bool → exclu
	}
	got := uimodel.MediaMounts(p, cfg)
	if len(got) != 1 {
		t.Fatalf("MediaMounts = %d, want 1 (tape uniquement) : %+v", len(got), got)
	}
	if got[0].Key != "tape" || got[0].Path != "/k7/jeu.k7" {
		t.Errorf("MediaMounts[0] = %+v, want {tape /k7/jeu.k7}", got[0])
	}

	// Média live à valeur vide → non monté.
	if m := uimodel.MediaMounts(p, machine.Config{"tape": ""}); len(m) != 0 {
		t.Errorf("MediaMounts(tape vide) = %+v, want aucun", m)
	}
}

// TestMediaMounts_Order vérifie l'ordre des Params quand plusieurs médias live sont
// présents (other : memo après couleur).
func TestMediaMounts_Order(t *testing.T) {
	got := uimodel.MediaMounts(otherProfile(), machine.Config{"memo": "/c/m.rom", "moniteur": "/c/mon.rom"})
	if len(got) != 1 || got[0].Key != "memo" { // moniteur boot-only exclu
		t.Fatalf("MediaMounts(other) = %+v, want [memo]", got)
	}
}

// TestInitialValues_NoLeakBetweenProfiles : repartir des valeurs d'un profil ne doit
// JAMAIS conserver une clé du profil précédent.
func TestInitialValues_NoLeakBetweenProfiles(t *testing.T) {
	// fake : defaults ram=512, turbo=false, speed=1 (rom/tape sans Default → absents).
	vf := uimodel.InitialValues(fakeProfile())
	if vf["ram"] != 512 || vf["turbo"] != false || vf["speed"] != 1 {
		t.Errorf("InitialValues(fake) = %+v, want ram=512 turbo=false speed=1", vf)
	}
	if _, ok := vf["rom"]; ok {
		t.Errorf("InitialValues(fake) ne doit pas contenir rom (pas de Default) : %+v", vf)
	}

	// Bascule vers other : aucune clé de fake (ram/turbo/speed/rom/tape) ne doit subsister.
	vo := uimodel.InitialValues(otherProfile())
	for _, leaked := range []string{"ram", "turbo", "speed", "rom", "tape"} {
		if _, ok := vo[leaked]; ok {
			t.Errorf("InitialValues(other) fuite la clé %q de fake : %+v", leaked, vo)
		}
	}
	if vo["couleur"] != true { // seul Default non nil d'other
		t.Errorf("InitialValues(other) = %+v, want couleur=true", vo)
	}
}

// TestResolveDiskROM vérifie l'auto-détection de la ROM contrôleur cd90-640.rom à côté de
// la ROM système (miroir du boot CLI) : détectée si présente ; jamais d'écrasement d'une
// disk-rom explicite ; rien sans ROM ou sans fichier voisin.
func TestResolveDiskROM(t *testing.T) {
	const romDir = "/roms"
	rom := filepath.Join(romDir, "mo5.rom")
	candidate := filepath.Join(romDir, "cd90-640.rom")
	existsOnly := func(want string) func(string) bool {
		return func(p string) bool { return p == want }
	}

	// disk-rom fournie explicitement → ne pas écraser.
	if got := uimodel.ResolveDiskROM(
		machine.Config{machine.KeyROM: rom, machine.KeyDiskROM: "/x/custom.rom"},
		existsOnly(candidate),
	); got != "" {
		t.Errorf("disk-rom explicite : got %q, want \"\"", got)
	}

	// ROM + cd90-640.rom voisin présent → auto-détecté.
	if got := uimodel.ResolveDiskROM(machine.Config{machine.KeyROM: rom}, existsOnly(candidate)); got != candidate {
		t.Errorf("auto-détection : got %q, want %q", got, candidate)
	}

	// ROM mais aucun cd90-640.rom voisin → rien.
	if got := uimodel.ResolveDiskROM(machine.Config{machine.KeyROM: rom}, func(string) bool { return false }); got != "" {
		t.Errorf("cd90-640 absent : got %q, want \"\"", got)
	}

	// Pas de ROM système → rien à quoi adosser l'auto-détection.
	if got := uimodel.ResolveDiskROM(machine.Config{}, existsOnly(candidate)); got != "" {
		t.Errorf("sans ROM : got %q, want \"\"", got)
	}
}
