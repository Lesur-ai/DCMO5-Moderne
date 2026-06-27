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

// --- NextProfile : cible d'un bouton « Changer de machine » ---

// Deux machines : le bouton bascule vers « l'autre », dans les deux sens.
func TestNextProfile_Toggle(t *testing.T) {
	profiles := []machine.MachineProfile{{ID: "mo5"}, {ID: "to8d"}}
	if p, ok := overlay.NextProfile(profiles, "mo5"); !ok || p.ID != "to8d" {
		t.Errorf("NextProfile(mo5) = %q,%v, want to8d,true", p.ID, ok)
	}
	if p, ok := overlay.NextProfile(profiles, "to8d"); !ok || p.ID != "mo5" {
		t.Errorf("NextProfile(to8d) = %q,%v, want mo5,true", p.ID, ok)
	}
}

// Zéro ou une seule machine : aucune AUTRE cible → false (l'UI masque le bouton).
func TestNextProfile_SingleOrEmpty(t *testing.T) {
	if _, ok := overlay.NextProfile([]machine.MachineProfile{{ID: "mo5"}}, "mo5"); ok {
		t.Error("une seule machine : aucune autre cible, want false")
	}
	if _, ok := overlay.NextProfile(nil, "mo5"); ok {
		t.Error("aucune machine : want false")
	}
}

// Identifiant courant absent de la liste : repli sur le premier profil.
func TestNextProfile_CurrentAbsentFallsToFirst(t *testing.T) {
	profiles := []machine.MachineProfile{{ID: "mo5"}, {ID: "to8d"}}
	if p, ok := overlay.NextProfile(profiles, "inconnu"); !ok || p.ID != "mo5" {
		t.Errorf("courante absente → premier profil, got %q,%v", p.ID, ok)
	}
}

// Trois machines : cyclage déterministe (preuve que ce n'est pas un simple binaire).
func TestNextProfile_CyclesThreeMachines(t *testing.T) {
	profiles := []machine.MachineProfile{{ID: "a"}, {ID: "b"}, {ID: "c"}}
	if p, _ := overlay.NextProfile(profiles, "b"); p.ID != "c" {
		t.Errorf("cycle b→c attendu, got %q", p.ID)
	}
	if p, _ := overlay.NextProfile(profiles, "c"); p.ID != "a" {
		t.Errorf("cycle c→a attendu, got %q", p.ID)
	}
}

// --- SwitchPersisted : config mémorisée (ROM) de la machine cible ---

// ROM connue → injectée sous KeyROM.
func TestSwitchPersisted_RomKnown(t *testing.T) {
	target := machine.MachineProfile{ID: "to8d"}
	cfg := overlay.SwitchPersisted(target, func(id string) string {
		if id == "to8d" {
			return "rom/to8d.rom"
		}
		return ""
	})
	if cfg[machine.KeyROM] != "rom/to8d.rom" {
		t.Errorf("cfg[rom] = %v, want rom/to8d.rom", cfg[machine.KeyROM])
	}
}

// ROM inconnue (jamais configurée) ou résolveur nil → config VIDE : PrepareSwitch
// échouera proprement sur le Param ROM requis, AVANT tout arrêt de la machine courante.
func TestSwitchPersisted_RomUnknownOrNil(t *testing.T) {
	target := machine.MachineProfile{ID: "to8d"}
	if cfg := overlay.SwitchPersisted(target, func(string) string { return "" }); len(cfg) != 0 {
		t.Errorf("ROM inconnue : config vide attendue, got %+v", cfg)
	}
	if cfg := overlay.SwitchPersisted(target, nil); len(cfg) != 0 {
		t.Errorf("romFor nil : config vide attendue, got %+v", cfg)
	}
}
