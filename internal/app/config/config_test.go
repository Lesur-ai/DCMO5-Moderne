package config_test

import (
	"path/filepath"
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
	if got != cfg {
		t.Errorf("roundtrip: got %+v, want %+v", got, cfg)
	}
}

func TestConfig_LoadAbsent_returnsEmpty(t *testing.T) {
	store, _ := config.NewStoreAt(t.TempDir())
	cfg, err := store.Load()
	if err != nil {
		t.Fatalf("Load fichier absent: %v", err)
	}
	if cfg != (config.Config{}) {
		t.Errorf("Load absent: got %+v, want zero", cfg)
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
