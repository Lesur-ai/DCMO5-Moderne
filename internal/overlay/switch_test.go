package overlay_test

import (
	"testing"

	"github.com/Lesur-ai/dcmo5/internal/machine"
	"github.com/Lesur-ai/dcmo5/internal/overlay"
)

// noFile : exists qui ne trouve aucun fichier (pas d'auto-détection disk-rom).
func noFile(string) bool { return false }

// Switch valide : préparer le passage à TO8D avec sa ROM + une cassette produit une
// Config validée et la liste des médias à monter à chaud (la cassette, pas la ROM).
func TestPrepareSwitch_Valid(t *testing.T) {
	to8d, ok := machine.ByID("to8d")
	if !ok {
		t.Fatal("profil to8d introuvable")
	}
	prep, err := overlay.PrepareSwitch(to8d, machine.Config{
		machine.KeyROM:  "rom/to8d.rom",
		machine.KeyTape: "game.k7",
	}, noFile)
	if err != nil {
		t.Fatalf("PrepareSwitch (valide) → err = %v", err)
	}
	if prep.Config[machine.KeyROM] != "rom/to8d.rom" {
		t.Errorf("ROM absente de la config préparée : %+v", prep.Config)
	}
	// La ROM (boot-only) ne se monte PAS à chaud ; la cassette (LiveMutable) si.
	for _, m := range prep.Mounts {
		if m.Key == machine.KeyROM {
			t.Errorf("la ROM (boot-only) ne doit jamais figurer dans les médias à monter : %+v", prep.Mounts)
		}
	}
	if len(prep.Mounts) != 1 || prep.Mounts[0].Key != machine.KeyTape || prep.Mounts[0].Path != "game.k7" {
		t.Errorf("médias à monter = %+v, want [tape:game.k7]", prep.Mounts)
	}
}

// State-safety (B2) : un switch vers TO8D SANS sa ROM requise échoue À LA PRÉPARATION
// (avant toute destruction de la session courante). C'est l'invariant clé : la
// validation précède l'arrêt de l'ancienne machine.
func TestPrepareSwitch_MissingRequiredROM_FailsAtPrepare(t *testing.T) {
	to8d, ok := machine.ByID("to8d")
	if !ok {
		t.Fatal("profil to8d introuvable")
	}
	prep, err := overlay.PrepareSwitch(to8d, machine.Config{}, noFile)
	if err == nil {
		t.Fatal("switch vers TO8D sans ROM doit échouer à la préparation (et NE PAS engager la destruction)")
	}
	if prep.Config != nil || prep.Mounts != nil {
		t.Errorf("préparation en échec doit renvoyer un Prep vide, got %+v", prep)
	}
}

// Auto-détection de la ROM contrôleur (miroir CLI) : si « cd90-640.rom » existe à côté
// de la ROM système et qu'aucune disk-rom n'est fournie, elle est injectée dans la config.
func TestPrepareSwitch_AutoDetectsDiskROM(t *testing.T) {
	mo5, ok := machine.ByID("mo5")
	if !ok {
		t.Fatal("profil mo5 introuvable")
	}
	exists := func(p string) bool { return p == "roms/cd90-640.rom" }
	prep, err := overlay.PrepareSwitch(mo5, machine.Config{machine.KeyROM: "roms/mo5.rom"}, exists)
	if err != nil {
		t.Fatalf("PrepareSwitch MO5 → err = %v", err)
	}
	if prep.Config[machine.KeyDiskROM] != "roms/cd90-640.rom" {
		t.Errorf("disk-rom auto-détectée attendue, got %v", prep.Config[machine.KeyDiskROM])
	}
	// La disk-rom est boot-only (consommée par New) : jamais montée à chaud.
	for _, m := range prep.Mounts {
		if m.Key == machine.KeyDiskROM {
			t.Errorf("la disk-rom (boot-only) ne doit pas figurer dans les médias à monter : %+v", prep.Mounts)
		}
	}
}

// Anti-fuite : la base est InitialValues(newProfile), pas une config héritée. On
// prépare MO5 avec sa ROM ; sans disk-rom fournie et sans auto-détection, la config
// ne doit PAS inventer de disk-rom.
func TestPrepareSwitch_NoDiskROMWhenAbsent(t *testing.T) {
	mo5, ok := machine.ByID("mo5")
	if !ok {
		t.Fatal("profil mo5 introuvable")
	}
	prep, err := overlay.PrepareSwitch(mo5, machine.Config{machine.KeyROM: "roms/mo5.rom"}, noFile)
	if err != nil {
		t.Fatalf("PrepareSwitch MO5 → err = %v", err)
	}
	if dr, present := prep.Config[machine.KeyDiskROM]; present && dr != "" {
		t.Errorf("aucune disk-rom ne doit être injectée si absente/non détectée, got %v", dr)
	}
}
