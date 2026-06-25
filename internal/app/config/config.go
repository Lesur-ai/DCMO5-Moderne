// Package config gère les préférences utilisateur portables macOS/Linux.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const appName = "dcmo5"

// legacyROMMachine est la machine dont la ROM était stockée dans le champ global
// ROMPath AVANT le support multi-machines (le MO5 était alors la seule machine
// sélectionnable). Les configs existantes sont ainsi relues sans migration explicite.
const legacyROMMachine = "mo5"

// Config contient les préférences utilisateur persistées.
type Config struct {
	ROMPath string `json:"rom_path"` // ROM MO5 (legacy ; miroir de ROMByMachine["mo5"] pour compat)
	// ROMByMachine mémorise le chemin ROM PAR machine (multi-machines, #118) : chaque
	// modèle a sa ROM (taille/format propres), un champ global unique mélangerait par
	// ex. la ROM TO8D (80 Ko) et le fallback MO5 (16 Ko).
	ROMByMachine map[string]string `json:"rom_by_machine,omitempty"`
	LastTape     string            `json:"last_tape"`    // dernier fichier cassette
	LastDisk     string            `json:"last_disk"`    // dernier fichier disquette
	LastCart     string            `json:"last_cart"`    // dernière cartouche
	KeyboardMap  string            `json:"keyboard_map"` // "default" ou chemin fichier mapping custom
}

// ROMFor retourne le chemin ROM mémorisé pour la machine machineID, ou "" si aucun.
// Repli sur l'ancien champ global ROMPath pour le MO5 (compat des configs écrites
// avant le support multi-machines).
func (c Config) ROMFor(machineID string) string {
	if p := c.ROMByMachine[machineID]; p != "" {
		return p
	}
	if machineID == legacyROMMachine {
		return c.ROMPath
	}
	return ""
}

// SetROMFor mémorise le chemin ROM de la machine machineID SANS écraser celui des
// autres machines. Le MO5 met aussi à jour ROMPath (compat ascendante / boot CLI).
func (c *Config) SetROMFor(machineID, path string) {
	if c.ROMByMachine == nil {
		c.ROMByMachine = map[string]string{}
	}
	c.ROMByMachine[machineID] = path
	if machineID == legacyROMMachine {
		c.ROMPath = path
	}
}

// Store gère la persistence des préférences et le répertoire de données.
type Store struct {
	configPath string
	dataDir    string
}

// NewStore crée un Store utilisant les répertoires OS standard.
// macOS : ~/Library/Application Support/dcmo5/
// Linux : ~/.config/dcmo5/
func NewStore() (*Store, error) {
	cfgDir, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("config: répertoire config OS: %w", err)
	}
	dir := filepath.Join(cfgDir, appName)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("config: créer répertoire: %w", err)
	}
	return &Store{
		configPath: filepath.Join(dir, "config.json"),
		dataDir:    dir,
	}, nil
}

// NewStoreAt crée un Store dans un répertoire explicite (tests).
func NewStoreAt(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("config: créer répertoire: %w", err)
	}
	return &Store{
		configPath: filepath.Join(dir, "config.json"),
		dataDir:    dir,
	}, nil
}

// Load charge la configuration depuis le fichier. Retourne Config{} si absent.
func (s *Store) Load() (Config, error) {
	data, err := os.ReadFile(s.configPath)
	if os.IsNotExist(err) {
		return Config{}, nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("config: lire: %w", err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("config: parser: %w", err)
	}
	return cfg, nil
}

// Save persiste la configuration dans le fichier.
func (s *Store) Save(cfg Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("config: sérialiser: %w", err)
	}
	tmp := s.configPath + ".tmp"
	if err := os.WriteFile(tmp, data, 0o640); err != nil {
		return fmt.Errorf("config: écrire: %w", err)
	}
	return os.Rename(tmp, s.configPath)
}

// DataDir retourne le répertoire de données utilisateur de l'application.
// Ce chemin ne dépend jamais du répertoire courant.
func (s *Store) DataDir() string { return s.dataDir }
