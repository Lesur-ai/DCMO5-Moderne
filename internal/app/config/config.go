// Package config gère les préférences utilisateur portables macOS/Linux.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const appName = "dcmo5"

// Config contient les préférences utilisateur persistées.
type Config struct {
	ROMPath     string `json:"rom_path"`     // chemin ROM système (import utilisateur)
	LastTape    string `json:"last_tape"`    // dernier fichier cassette
	LastDisk    string `json:"last_disk"`    // dernier fichier disquette
	LastCart    string `json:"last_cart"`    // dernière cartouche
	KeyboardMap string `json:"keyboard_map"` // "default" ou chemin fichier mapping custom
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
