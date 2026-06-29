package config_test

import (
	"os"
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

// TestConfig_JoystickKeyboard_persistence (B9) : le toggle joystick clavier est
// une préférence GLOBALE (pas par machine, séance design 29/06/2026). Vérifie
// que le champ est persisté, lu à la valeur par défaut quand absent, et ne
// pollue pas les autres champs.
func TestConfig_JoystickKeyboard_persistence(t *testing.T) {
	store, _ := config.NewStoreAt(t.TempDir())
	// Défaut : false (absent du JSON → zéro-value Go).
	cfg, err := store.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.JoystickKeyboard {
		t.Error("JoystickKeyboard devrait être false par défaut (fichier absent)")
	}
	// Activer et persister.
	cfg.JoystickKeyboard = true
	cfg.SetROMFor("mo5", "/opt/mo5.rom")
	if err := store.Save(cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}
	// Relire : le toggle est restauré et la ROM n'est pas affectée.
	got, err := store.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !got.JoystickKeyboard {
		t.Error("JoystickKeyboard devrait être true après Save/Load")
	}
	if got.ROMFor("mo5") != "/opt/mo5.rom" {
		t.Errorf("ROMFor(mo5) = %q, want /opt/mo5.rom (pas pollué par JoystickKeyboard)", got.ROMFor("mo5"))
	}
	// Désactiver et vérifier que omitempty ne casse pas la relecture.
	got.JoystickKeyboard = false
	if err := store.Save(got); err != nil {
		t.Fatalf("Save(false): %v", err)
	}
	got2, err := store.Load()
	if err != nil {
		t.Fatalf("Load after Save(false): %v", err)
	}
	if got2.JoystickKeyboard {
		t.Error("JoystickKeyboard devrait être false après re-Save(false)")
	}
}

func TestJoystickKeyboardPreference(t *testing.T) {
	t.Run("nil store disables preference", func(t *testing.T) {
		if got := config.JoystickKeyboardPreference(nil); got {
			t.Fatal("JoystickKeyboardPreference(nil) = true, want false")
		}
	})

	t.Run("persisted value is restored", func(t *testing.T) {
		store, err := config.NewStoreAt(t.TempDir())
		if err != nil {
			t.Fatalf("NewStoreAt: %v", err)
		}
		if err := store.Save(config.Config{JoystickKeyboard: true}); err != nil {
			t.Fatalf("Save: %v", err)
		}
		if got := config.JoystickKeyboardPreference(store); !got {
			t.Fatal("JoystickKeyboardPreference(store) = false, want true")
		}
	})

	t.Run("load error disables preference", func(t *testing.T) {
		dir := t.TempDir()
		store, err := config.NewStoreAt(dir)
		if err != nil {
			t.Fatalf("NewStoreAt: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte("{"), 0o600); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
		if got := config.JoystickKeyboardPreference(store); got {
			t.Fatal("JoystickKeyboardPreference(corrupt store) = true, want false")
		}
	})
}

func TestPersistJoystickKeyboard(t *testing.T) {
	t.Run("nil store is a no-op", func(t *testing.T) {
		if err := config.PersistJoystickKeyboard(nil, true); err != nil {
			t.Fatalf("PersistJoystickKeyboard(nil): %v", err)
		}
	})

	t.Run("updates toggle and preserves other fields", func(t *testing.T) {
		store, err := config.NewStoreAt(t.TempDir())
		if err != nil {
			t.Fatalf("NewStoreAt: %v", err)
		}
		before := config.Config{
			LastTape:    "/media/game.k7",
			LastDisk:    "/media/disk.fd",
			LastCart:    "/media/cart.rom",
			KeyboardMap: "default",
		}
		before.SetROMFor("mo5", "/rom/mo5.rom")
		before.SetROMFor("to8d", "/rom/to8d.rom")
		if err := store.Save(before); err != nil {
			t.Fatalf("Save: %v", err)
		}

		if err := config.PersistJoystickKeyboard(store, true); err != nil {
			t.Fatalf("PersistJoystickKeyboard(true): %v", err)
		}

		got, err := store.Load()
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if !got.JoystickKeyboard {
			t.Fatal("JoystickKeyboard = false, want true")
		}
		if got.ROMFor("mo5") != "/rom/mo5.rom" || got.ROMFor("to8d") != "/rom/to8d.rom" {
			t.Fatalf("ROM mappings not preserved: mo5=%q to8d=%q", got.ROMFor("mo5"), got.ROMFor("to8d"))
		}
		if got.LastTape != before.LastTape || got.LastDisk != before.LastDisk || got.LastCart != before.LastCart || got.KeyboardMap != before.KeyboardMap {
			t.Fatalf("non-joystick fields not preserved: got=%+v before=%+v", got, before)
		}
	})

	t.Run("load error does not overwrite config file", func(t *testing.T) {
		dir := t.TempDir()
		store, err := config.NewStoreAt(dir)
		if err != nil {
			t.Fatalf("NewStoreAt: %v", err)
		}
		configPath := filepath.Join(dir, "config.json")
		if err := os.WriteFile(configPath, []byte("{"), 0o600); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}

		if err := config.PersistJoystickKeyboard(store, true); err == nil {
			t.Fatal("PersistJoystickKeyboard(corrupt config) = nil, want error")
		}
		data, err := os.ReadFile(configPath)
		if err != nil {
			t.Fatalf("ReadFile: %v", err)
		}
		if string(data) != "{" {
			t.Fatalf("corrupt config was overwritten: %q", string(data))
		}
	})
}
