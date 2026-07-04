package to8d

import (
	"hash/fnv"
	"os"
	"testing"
	"time"

	"github.com/Lesur-ai/dcmo5/internal/machine"
)

// testBootDate est la date FIXE injectée au boot dans les tests : elle garde le boot
// reproductible (le chemin de production passe time.Now()). Format attendu : "02-01-26".
var testBootDate = time.Date(2026, time.January, 2, 0, 0, 0, 0, time.UTC)

// bootSignature est la signature FNV-1a du framebuffer TO8D après bootCycles cycles
// depuis le reset, calculée sur la ROM réelle (rom/to8d.rom). C'est un VERROU
// anti-régression du boot : toute dérive du chemin reset → exécution → décodage vidéo
// la fait changer. Valeur mesurée puis figée (cf. TestBootDeterministic).
const (
	bootCycles    = 1_200_000  // ~60 trames (50 Hz @ 1 MHz) : laisse le moniteur dessiner
	bootSignature = 0x23b3abf5 // FNV-1a du framebuffer après bootCycles (rom/to8d.rom, date fixe testBootDate)
)

// mustBoot construit une machine TO8D depuis la ROM réelle versionnée, avec la date
// de boot FIXE testBootDate (déterminisme). On passe par newFromROM plutôt que
// newFromConfig pour ne pas dépendre de time.Now() du chemin de production.
func mustBoot(t *testing.T) machine.Machine {
	t.Helper()
	blob, err := os.ReadFile(romTestPath())
	if err != nil {
		t.Fatalf("lecture ROM TO8D : %v", err)
	}
	m, err := newFromROM(blob, testBootDate)
	if err != nil {
		t.Fatalf("boot TO8D : %v", err)
	}
	return m
}

// frameAfter capture le framebuffer après avoir exécuté cycles cycles.
func frameAfter(m machine.Machine, cycles int) []uint32 {
	for done := 0; done < cycles; {
		done += m.Step(cycles - done)
	}
	w, h := m.FrameSize()
	fb := make([]uint32, w*h)
	m.FramebufferInto(fb)
	return fb
}

func fnv1a(fb []uint32) uint32 {
	h := fnv.New32a()
	var b [4]byte
	for _, px := range fb {
		b[0] = byte(px)
		b[1] = byte(px >> 8)
		b[2] = byte(px >> 16)
		b[3] = byte(px >> 24)
		h.Write(b[:])
	}
	return h.Sum32()
}

func equalFB(a, b []uint32) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// uniform indique que tout le framebuffer est d'une seule couleur (écran vide /
// bordure seule) : preuve qu'aucun contenu n'a été rendu.
func uniform(fb []uint32) bool {
	for _, p := range fb {
		if p != fb[0] {
			return false
		}
	}
	return true
}

// TestProfileRegistered vérifie que l'import du paquet enregistre le profil TO8D,
// résoluble par le launcher (machine.Profiles) et par --machine to8d (machine.ByID),
// avec une ROM système requise (boot-only).
func TestProfileRegistered(t *testing.T) {
	p, ok := machine.ByID("to8d")
	if !ok {
		t.Fatal("profil to8d non enregistré")
	}
	if p.Name != "Thomson TO8D" || p.Family != machine.FamilyTOGateArray {
		t.Fatalf("profil to8d = {Name:%q Family:%d}", p.Name, p.Family)
	}
	var rom *machine.Param
	for i := range p.Params {
		if p.Params[i].Key == machine.KeyROM {
			rom = &p.Params[i]
		}
	}
	if rom == nil || !rom.Required || rom.Kind != machine.ParamFile {
		t.Fatalf("paramètre ROM to8d invalide : %+v", rom)
	}
}

// TestNewFromConfig_Errors couvre les chemins d'erreur de construction.
func TestNewFromConfig_Errors(t *testing.T) {
	if _, err := newFromConfig(machine.Config{}); err == nil {
		t.Error("ROM absente : erreur attendue")
	}
	if _, err := newFromConfig(machine.Config{machine.KeyROM: "/inexistant.rom"}); err == nil {
		t.Error("ROM introuvable : erreur attendue")
	}
	if _, err := newFromROM(make([]byte, 1024), testBootDate); err == nil {
		t.Error("taille ROM invalide : erreur attendue")
	}
}

// TestBootDeterministic est le test de boot du moniteur TO8D sur la ROM réelle. Il
// n'affirme pas « le moniteur est lisible » (validation visuelle owner), mais des
// propriétés VÉRIFIABLES et non complaisantes :
//  1. le moniteur a RENDU DU CONTENU : le framebuffer n'est plus uniforme (sans le
//     câblage du faisceau e7e7/e7ca, le firmware boucle et l'écran reste vide) ;
//  2. le boot a fait progresser le CPU : PC a quitté le vecteur reset (0xFDC8) ;
//  3. DÉTERMINISME : deux instances fraîches produisent un framebuffer identique
//     (pas d'état partagé ; la date de boot est injectée mais FIXE via testBootDate) ;
//  4. signature stable (verrou anti-régression du chemin reset → exécution → vidéo).
func TestBootDeterministic(t *testing.T) {
	m1 := mustBoot(t)
	w, h := m1.FrameSize()
	fbReset := make([]uint32, w*h)
	m1.FramebufferInto(fbReset)
	if pc := m1.CPUSnapshot().PC; pc != 0xFDC8 {
		t.Fatalf("PC au reset = 0x%04x, attendu le vecteur reset 0xFDC8", pc)
	}

	fbBoot := frameAfter(m1, bootCycles)

	if uniform(fbBoot) {
		t.Fatal("framebuffer uniforme après boot : le moniteur n'a rien rendu (synchro faisceau ?)")
	}
	if equalFB(fbReset, fbBoot) {
		t.Fatal("framebuffer inchangé depuis le reset : le boot n'a rien dessiné")
	}
	if pc := m1.CPUSnapshot().PC; pc == 0xFDC8 {
		t.Fatal("PC toujours au vecteur reset : le CPU n'a pas exécuté")
	}

	fbBoot2 := frameAfter(mustBoot(t), bootCycles)
	if !equalFB(fbBoot, fbBoot2) {
		t.Fatal("boot non déterministe : deux instances divergent")
	}

	got := fnv1a(fbBoot)
	if bootSignature == 0 {
		t.Fatalf("signature de boot à figer : bootSignature = 0x%08x", got)
	}
	if got != bootSignature {
		t.Fatalf("signature framebuffer boot = 0x%08x, attendu 0x%08x (régression du boot ?)", got, bootSignature)
	}
}
