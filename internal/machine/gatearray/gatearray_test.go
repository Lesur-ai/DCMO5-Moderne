package gatearray_test

// Tests de la carte mémoire et du banking gate-array (#112). Boîte noire : tout
// passe par New/Read8/Write8/LoadCartridge. ROMs synthétiques (motifs distincts
// par banque) pour observer la commutation. Aucune ROM Thomson copyright utilisée.

import (
	"testing"

	"github.com/Lesur-ai/dcmo5/internal/machine/gatearray"
)

// romMonPattern : moniteur 16 Ko = 2 banques de 8 Ko, remplies 0x50 / 0x60.
func romMonPattern() []byte {
	r := make([]byte, 0x4000)
	for i := range r {
		if i < 0x2000 {
			r[i] = 0x50
		} else {
			r[i] = 0x60
		}
	}
	return r
}

// romBasicPattern : ROM interne 64 Ko = 4 banques de 16 Ko, banque b remplie 0xB0+b.
func romBasicPattern() []byte {
	r := make([]byte, 0x10000)
	for b := 0; b < 4; b++ {
		for i := 0; i < 0x4000; i++ {
			r[b*0x4000+i] = byte(0xB0 + b)
		}
	}
	return r
}

func newGA() *gatearray.GateArray {
	return gatearray.New(romMonPattern(), romBasicPattern())
}

// TestDispatchByPage vérifie que chaque zone de la carte mémoire est indépendante
// (RAM vidéo 0x4000, RAM user 0x6000–0x9000, banque RAM 0xA000).
func TestDispatchByPage(t *testing.T) {
	g := newGA()
	g.Write8(0x4000, 0x11) // RAM vidéo
	g.Write8(0x6000, 0x22) // RAM user
	g.Write8(0x9FFF, 0x33) // RAM user (haut)
	g.Write8(0xA000, 0x44) // banque RAM
	if v := g.Read8(0x4000); v != 0x11 {
		t.Errorf("RAM vidéo 0x4000 = 0x%02X, want 0x11", v)
	}
	if v := g.Read8(0x6000); v != 0x22 {
		t.Errorf("RAM user 0x6000 = 0x%02X, want 0x22", v)
	}
	if v := g.Read8(0x9FFF); v != 0x33 {
		t.Errorf("RAM user 0x9FFF = 0x%02X, want 0x33", v)
	}
	if v := g.Read8(0xA000); v != 0x44 {
		t.Errorf("banque RAM 0xA000 = 0x%02X, want 0x44", v)
	}
}

// TestVideoPageSwitch vérifie la commutation page couleurs/formes via e7c3 bit0 :
// les deux pages 8 Ko sont des zones RAM distinctes.
func TestVideoPageSwitch(t *testing.T) {
	g := newGA()
	g.Write8(0xE7C3, 0x00) // page 0 (couleurs)
	g.Write8(0x4000, 0xAA)
	g.Write8(0xE7C3, 0x01) // page 1 (formes)
	g.Write8(0x4000, 0xBB)
	if v := g.Read8(0x4000); v != 0xBB {
		t.Errorf("page 1 0x4000 = 0x%02X, want 0xBB", v)
	}
	g.Write8(0xE7C3, 0x00) // retour page 0
	if v := g.Read8(0x4000); v != 0xAA {
		t.Errorf("page 0 0x4000 = 0x%02X, want 0xAA (isolation des pages vidéo)", v)
	}
}

// TestRAMBankTO8 vérifie les 32 banques RAM commutables en mode TO8 (e7e7 bit4
// armé, banque via e7e5) : chaque banque est une zone mémoire distincte.
func TestRAMBankTO8(t *testing.T) {
	g := newGA()
	g.Write8(0xE7E7, 0x10) // mode TO8 (e7e5 pilote la banque)
	// Écrire une signature distincte dans 0xA000 pour quelques banques.
	for _, b := range []byte{0, 1, 5, 31} {
		g.Write8(0xE7E5, b)
		g.Write8(0xA000, 0xC0|b)
	}
	for _, b := range []byte{0, 1, 5, 31} {
		g.Write8(0xE7E5, b)
		if v := g.Read8(0xA000); v != 0xC0|b {
			t.Errorf("banque RAM %d : 0xA000 = 0x%02X, want 0x%02X", b, v, 0xC0|b)
		}
	}
}

// TestRAMBankCompat vérifie le mode compatibilité TO7/70 via e7c9 (sans e7e7 bit4)
// : deux configurations e7c9 sélectionnent deux banques distinctes.
func TestRAMBankCompat(t *testing.T) {
	g := newGA()
	// e7e7 bit4 à 0 → c'est e7c9 qui pilote la banque RAM.
	g.Write8(0xE7C9, 0x08) // nrambank 0
	g.Write8(0xB000, 0x77)
	g.Write8(0xE7C9, 0x10) // nrambank 1
	g.Write8(0xB000, 0x88)
	g.Write8(0xE7C9, 0x08) // retour nrambank 0
	if v := g.Read8(0xB000); v != 0x77 {
		t.Errorf("compat banque 0 : 0xB000 = 0x%02X, want 0x77 (isolation e7c9)", v)
	}
}

// TestROMBankInternal vérifie la ROM interne (e7c3 bit2=1) et la commutation de
// banque par écriture dans l'espace ROM (carflags = a&3).
func TestROMBankInternal(t *testing.T) {
	g := newGA()
	g.Write8(0xE7C3, 0x04) // ROM interne active (bit2)
	// Sélectionner chaque banque via écriture d'adresse a&3 dans l'espace ROM.
	for b := 0; b < 4; b++ {
		g.Write8(uint16(b), 0) // commute carflags = a&3 = b
		if v := g.Read8(0x0000); v != byte(0xB0+b) {
			t.Errorf("ROM interne banque %d : 0x0000 = 0x%02X, want 0x%02X", b, v, 0xB0+b)
		}
	}
}

// TestROMBankCartridge vérifie le routage cartouche (e7c3 bit2=0, défaut reset) et
// la commutation de banque cartouche.
func TestROMBankCartridge(t *testing.T) {
	g := newGA()
	// Cartouche 64 Ko : banque b remplie de 0xE0+b.
	cart := make([]byte, 0x10000)
	for b := 0; b < 4; b++ {
		for i := 0; i < 0x4000; i++ {
			cart[b*0x4000+i] = byte(0xE0 + b)
		}
	}
	g.LoadCartridge(cart)
	// e7c3 bit2=0 par défaut → cartouche active.
	for b := 0; b < 4; b++ {
		g.Write8(uint16(b), 0) // carflags = b
		if v := g.Read8(0x0000); v != byte(0xE0+b) {
			t.Errorf("cartouche banque %d : 0x0000 = 0x%02X, want 0x%02X", b, v, 0xE0+b)
		}
	}
	// Bascule ROM interne : la même adresse lit désormais le BASIC, pas la cartouche.
	g.Write8(0x0000, 0)    // carflags = 0
	g.Write8(0xE7C3, 0x04) // ROM interne
	if v := g.Read8(0x0000); v != 0xB0 {
		t.Errorf("après bascule ROM interne : 0x0000 = 0x%02X, want 0xB0", v)
	}
}

// TestROMSystemBank vérifie la commutation de banque du moniteur système via
// e7c3 bit4 (deux banques de 8 Ko).
func TestROMSystemBank(t *testing.T) {
	g := newGA()
	// 0xF000 → romMon offset (0xF000-0xE000)=0x1000 + banque*0x2000.
	g.Write8(0xE7C3, 0x00) // banque système 0
	if v := g.Read8(0xF000); v != 0x50 {
		t.Errorf("ROM sys banque 0 : 0xF000 = 0x%02X, want 0x50", v)
	}
	g.Write8(0xE7C3, 0x10) // banque système 1 (bit4)
	if v := g.Read8(0xF000); v != 0x60 {
		t.Errorf("ROM sys banque 1 : 0xF000 = 0x%02X, want 0x60", v)
	}
}

// TestOverlayE7E6Inversion vérifie le recouvrement de l'espace ROM par la RAM
// (e7e6 bit5) AVEC l'inversion des deux segments de 8 Ko : une cellule écrite
// dans le segment bas de la banque RAM est lue par l'espace ROM page haute, et
// réciproquement.
func TestOverlayE7E6Inversion(t *testing.T) {
	g := newGA()
	const b = 3
	// Poser deux valeurs distinctes dans les segments bas/haut de la banque RAM b
	// via la banque RAM commutable (vue NON inversée : 0xA000=bas, 0xC000=haut).
	g.Write8(0xE7E7, 0x10) // mode TO8
	g.Write8(0xE7E5, b)
	g.Write8(0xA000, 0x11) // segment bas  (ram[b<<14 + 0x0000])
	g.Write8(0xC000, 0x99) // segment haut (ram[b<<14 + 0x2000])

	// Recouvrement ROM par la RAM banque b (lecture seule).
	g.Write8(0xE7E6, 0x20|b)
	// Inversion : la page basse CPU (0x0000) lit le segment HAUT, la page haute
	// (0x2000) lit le segment BAS.
	if v := g.Read8(0x0000); v != 0x99 {
		t.Errorf("recouvrement 0x0000 = 0x%02X, want 0x99 (segment haut via inversion)", v)
	}
	if v := g.Read8(0x2000); v != 0x11 {
		t.Errorf("recouvrement 0x2000 = 0x%02X, want 0x11 (segment bas via inversion)", v)
	}
}

// TestOverlayE7E6WriteProtect vérifie la protection d'écriture du recouvrement :
// écrire dans l'espace ROM n'est permis que si e7e6 a les bits 5 ET 6 armés.
func TestOverlayE7E6WriteProtect(t *testing.T) {
	g := newGA()
	const b = 2
	g.Write8(0xE7E7, 0x10)
	g.Write8(0xE7E5, b)
	g.Write8(0xC000, 0x99) // segment haut banque b (lu par 0x0000 en recouvrement)

	// Recouvrement bit5 seul → écriture interdite.
	g.Write8(0xE7E6, 0x20|b)
	g.Write8(0x0000, 0x55) // doit être ignorée
	if v := g.Read8(0x0000); v != 0x99 {
		t.Errorf("écriture recouvrement sans bit6 = 0x%02X, want 0x99 (doit être ignorée)", v)
	}

	// Recouvrement bits 5+6 → écriture autorisée (dans le segment haut, inversion).
	g.Write8(0xE7E6, 0x60|b)
	g.Write8(0x0000, 0x55)
	if v := g.Read8(0x0000); v != 0x55 {
		t.Errorf("écriture recouvrement avec bits 5+6 = 0x%02X, want 0x55", v)
	}
}

// TestResetClearsRAM vérifie que Reset réamorce la RAM (motif 0x00/0xFF) et l'état
// de banking.
func TestResetClearsRAM(t *testing.T) {
	g := newGA()
	g.Write8(0xE7E7, 0x10)
	g.Write8(0xE7E5, 7)
	g.Write8(0xA000, 0xFF)
	g.Reset()
	// Après reset : mode TO8 désarmé, banque RAM par compat (e7c9=0x0f → banque 0).
	// La cellule réamorcée suit le motif d'init (bit7 de l'index physique).
	g.Write8(0xE7E7, 0x10)
	g.Write8(0xE7E5, 7)
	got := g.Read8(0xA000)
	if got == 0xFF {
		t.Error("Reset n'a pas réamorcé la banque RAM (valeur d'avant reset persistante)")
	}
}

// TestReadE7C3Status vérifie le registre d'état e7c3 en lecture : bit7 toujours
// armé (réf C), bits de banking écrits reflétés (masqués 0x3d).
func TestReadE7C3Status(t *testing.T) {
	g := newGA()
	if v := g.Read8(0xE7C3); v&0x80 == 0 {
		t.Errorf("e7c3 lu = 0x%02X, bit7 (0x80) attendu armé", v)
	}
	g.Write8(0xE7C3, 0x14) // ROM interne (bit2) + banque syst (bit4) ; 0x14 & 0x3d = 0x14
	if v := g.Read8(0xE7C3); v != (0x14 | 0x80) {
		t.Errorf("e7c3 lu = 0x%02X, want 0x%02X (état + bits écrits)", v, 0x14|0x80)
	}
}

// TestLoadCartridgeResetsBank vérifie qu'un (re)chargement de cartouche repart sur
// la banque 0, même si une banque non nulle était sélectionnée auparavant.
func TestLoadCartridgeResetsBank(t *testing.T) {
	g := newGA()
	cart := make([]byte, 0x10000)
	for b := 0; b < 4; b++ {
		for i := 0; i < 0x4000; i++ {
			cart[b*0x4000+i] = byte(0xE0 + b)
		}
	}
	g.LoadCartridge(cart)
	g.Write8(0x0003, 0) // sélectionne la banque cartouche 3
	if v := g.Read8(0x0000); v != 0xE3 {
		t.Fatalf("préparation : banque 3 = 0x%02X, want 0xE3", v)
	}
	g.LoadCartridge(cart) // rechargement → banque 0
	if v := g.Read8(0x0000); v != 0xE0 {
		t.Errorf("après rechargement : 0x0000 = 0x%02X, want 0xE0 (banque réinitialisée)", v)
	}
}
