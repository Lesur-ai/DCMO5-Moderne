package impl

import (
	"fmt"
	"os"
)

const cartBankSize = 0x4000 // 16 Ko par banque

// FileCartridge implémente media.Cartridge depuis un fichier .rom.
// La taille doit être un multiple de 16 Ko (1 à 4 banques).
type FileCartridge struct {
	data []byte
}

// OpenCartridge charge un fichier .rom en mémoire.
// Taille valide : 16 Ko, 32 Ko, 48 Ko ou 64 Ko.
func OpenCartridge(path string) (*FileCartridge, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cartridge: %w", err)
	}
	if len(data) == 0 || len(data)%cartBankSize != 0 || len(data) > 4*cartBankSize {
		return nil, fmt.Errorf("cartridge: taille %d invalide (doit être un multiple de 16 Ko, max 64 Ko)", len(data))
	}
	return &FileCartridge{data: data}, nil
}

func (c *FileCartridge) Bytes() []byte { return c.data }
