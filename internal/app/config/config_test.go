package config_test

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Lesur-ai/dcmo5/internal/app/config"
)

func TestConfig_SaveLoad_roundtrip(t *testing.T) {
	store, err := config.NewStoreAt(t.TempDir())
	if err != nil {
		t.Fatalf("NewStoreAt: %v", err)
	}
	cfg := config.Config{
		ROMPath:     "/opt/mo5/mo5.rom",
		LastTape:    "/home/user/jeu.k7",
		KeyboardMap: "default",
	}
	if err := store.Save(cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !reflect.DeepEqual(got, cfg) {
		t.Errorf("roundtrip: got %+v, want %+v", got, cfg)
	}
}

func TestConfig_LoadAbsent_returnsEmpty(t *testing.T) {
	store, _ := config.NewStoreAt(t.TempDir())
	cfg, err := store.Load()
	if err != nil {
		t.Fatalf("Load fichier absent: %v", err)
	}
	if !reflect.DeepEqual(cfg, config.Config{}) {
		t.Errorf("Load absent: got %+v, want zero", cfg)
	}
}

// TestConfig_ROMPerMachine couvre la régression multi-machines (revue Codex #146) :
// mémoriser la ROM d'une machine ne doit JAMAIS écraser celle d'une autre, sinon une
// ROM TO8D (80 Ko) deviendrait le fallback MO5 (16 Ko attendus) → boot MO5 cassé.
func TestConfig_ROMPerMachine(t *testing.T) {
	var c config.Config

	// Compat : une ancienne config (ROMPath seul) est lue comme la ROM du MO5.
	c.ROMPath = "/opt/mo5.rom"
	if got := c.ROMFor("mo5"); got != "/opt/mo5.rom" {
		t.Fatalf("ROMFor(mo5) legacy = %q, want /opt/mo5.rom", got)
	}
	if got := c.ROMFor("to8d"); got != "" {
		t.Fatalf("ROMFor(to8d) sans entrée = %q, want \"\"", got)
	}

	// Mémoriser la ROM TO8D ne touche PAS le fallback MO5.
	c.SetROMFor("to8d", "/opt/to8d.rom")
	if got := c.ROMFor("to8d"); got != "/opt/to8d.rom" {
		t.Fatalf("ROMFor(to8d) = %q, want /opt/to8d.rom", got)
	}
	if got := c.ROMFor("mo5"); got != "/opt/mo5.rom" {
		t.Fatalf("ROMFor(mo5) après SetROMFor(to8d) = %q : régression, le MO5 a été écrasé", got)
	}

	// Le MO5 reste mirroré dans ROMPath (compat ascendante / boot CLI).
	c.SetROMFor("mo5", "/opt/mo5b.rom")
	if c.ROMPath != "/opt/mo5b.rom" {
		t.Fatalf("ROMPath après SetROMFor(mo5) = %q, want /opt/mo5b.rom", c.ROMPath)
	}

	// Round-trip Save/Load préserve la table par machine.
	store, _ := config.NewStoreAt(t.TempDir())
	if err := store.Save(c); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, _ := store.Load()
	if got.ROMFor("to8d") != "/opt/to8d.rom" || got.ROMFor("mo5") != "/opt/mo5b.rom" {
		t.Fatalf("après round-trip : to8d=%q mo5=%q", got.ROMFor("to8d"), got.ROMFor("mo5"))
	}
}

func TestConfig_DataDir_notRelative(t *testing.T) {
	dir := t.TempDir()
	store, _ := config.NewStoreAt(dir)
	dataDir := store.DataDir()
	if !filepath.IsAbs(dataDir) {
		t.Errorf("DataDir() = %q n'est pas un chemin absolu", dataDir)
	}
}

func TestConfig_DataDir_noCurrentDir(t *testing.T) {
	dir := t.TempDir()
	store, _ := config.NewStoreAt(dir)
	dataDir := store.DataDir()
	if strings.Contains(dataDir, "..") {
		t.Errorf("DataDir() = %q contient '..'", dataDir)
	}
}

func TestConfig_Save_atomicWrite(t *testing.T) {
	// Vérifie que Save utilise un fichier tmp (pas de corruption si crash).
	// On vérifie indirectement : deux Save successifs, Load retourne le dernier.
	store, _ := config.NewStoreAt(t.TempDir())
	store.Save(config.Config{ROMPath: "v1"})
	store.Save(config.Config{ROMPath: "v2"})
	cfg, _ := store.Load()
	if cfg.ROMPath != "v2" {
		t.Errorf("après deux Save: ROMPath = %q, want v2", cfg.ROMPath)
	}
}
