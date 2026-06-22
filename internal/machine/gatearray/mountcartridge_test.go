package gatearray_test

// Tests de MountCartridge (#132). Boîte noire : SetKey / Read8 / Write8 /
// MountCartridge. Réf C : Loadmemo() (dcto8ddevices.c:219) = RAZ partielle de la
// RAM (i < 0xc000) + Initprog(), et NON Hardreset(). Conséquences fidèles ici :
//   - capslock préservé (Hardreset le forcerait à true — finding revue PR #130) ;
//   - ports d'E/S préservés (Hardreset les remettrait à zéro) ;
//   - banques RAM hautes (≥ 0xc000) préservées (Hardreset les effacerait) ;
//   - décodage vidéo forcé en 320x16 (Initprog) mais e7dc préservé (Loadmemo) ;
//   - RAM basse réamorcée et cartouche routée (la sémantique utile de Loadmemo).
// Les helpers selectSysBank1 / keyY / keyCapsLock viennent de keyboard_test.go,
// les helpers vidéo (setColor/writeColorByte/...) de video_test.go, newGA de
// gatearray_test.go (même package gatearray_test).

import (
	"testing"

	"github.com/Lesur-ai/dcmo5/internal/media"
)

// fakeCartridge : cartouche mock exposant un contenu brut (contrat media.Cartridge).
type fakeCartridge struct{ data []byte }

func (c *fakeCartridge) Bytes() []byte { return c.data }

var _ media.Cartridge = (*fakeCartridge)(nil)

// TestMountCartridgePreservesCapsLock prouve que MountCartridge n'écrase pas
// capslock (réf C Loadmemo n'appelle pas Hardreset, donc ne touche pas capslock).
// RED sans fix : MountCartridge → Reset() → hardReset() force capslock=true, la
// lettre prend alors le bit 0x80 (0x82 au lieu de 0x02).
func TestMountCartridgePreservesCapsLock(t *testing.T) {
	g := newGA()
	g.SetKey(keyCapsLock, true) // capslock true (hard reset) → false

	g.MountCartridge(&fakeCartridge{data: make([]byte, 0x4000)})

	// Banque système 1 sélectionnée APRÈS montage (les ports ont pu être réécrits).
	selectSysBank1(t, g)
	g.SetKey(keyY, true) // lettre : sensible au capslock
	if v := g.Read8(0xF0F8); v != keyY {
		t.Errorf("après montage, lettre = 0x%02X, want 0x%02X (capslock OFF préservé ; "+
			"un hardReset le remettrait à true → 0x%02X)", v, keyY, keyY|0x80)
	}
}

// TestMountCartridgePreservesPorts prouve que MountCartridge ne remet pas les
// ports à zéro (réf C Loadmemo préserve port[]). e7e5 est inerte ici (e7e7 bit4=0)
// et non recalculé par Initprog → un montage fidèle le conserve.
// RED sans fix : hardReset() met tous les ports à zéro → e7e5 relu à 0x00.
func TestMountCartridgePreservesPorts(t *testing.T) {
	g := newGA()
	const want = 0x15
	g.Write8(0xE7E5, want)
	if v := g.Read8(0xE7E5); v != want {
		t.Fatalf("préparation : e7e5 = 0x%02X, want 0x%02X", v, want)
	}

	g.MountCartridge(&fakeCartridge{data: make([]byte, 0x4000)})

	if v := g.Read8(0xE7E5); v != want {
		t.Errorf("après montage : e7e5 = 0x%02X, want 0x%02X (port préservé ; "+
			"un hardReset le remettrait à 0x00)", v, want)
	}
}

// TestMountCartridgePreservesHighRAMBank prouve que le montage ne réamorce QUE les
// premiers 0xc000 octets de RAM (réf C Loadmemo : i < 0xc000), laissant intactes
// les banques RAM hautes. La banque TO8 3 mappe 0xA000 sur l'index physique 0xc000
// (hors zone réamorcée). RED sans fix : resetRAM() réamorçait toute la RAM → la
// signature 0x77 serait effacée (relue 0x00). Verrou de borne : une RAZ jusqu'à
// 0xc000 inclus (i <= 0xc000) effacerait aussi cet octet et ferait échouer le test.
func TestMountCartridgePreservesHighRAMBank(t *testing.T) {
	g := newGA()
	g.Write8(0xE7E7, 0x10) // mode banque TO8 (e7e7 bit4)
	g.Write8(0xE7E5, 3)    // banque RAM 3 → 0xA000 = index physique 0xc000
	g.Write8(0xA000, 0x77) // signature dans la banque haute

	g.MountCartridge(&fakeCartridge{data: make([]byte, 0x4000)})

	// Resélection de la banque 3 (isole l'étendue RAM de la préservation des ports).
	g.Write8(0xE7E7, 0x10)
	g.Write8(0xE7E5, 3)
	if v := g.Read8(0xA000); v != 0x77 {
		t.Errorf("après montage : banque RAM haute 0xA000 = 0x%02X, want 0x77 préservé "+
			"(réf C Loadmemo réamorce seulement i < 0xc000 ; un reset RAM complet l'effacerait)", v)
	}
}

// TestMountCartridgeResetsRAMAndMapsCartridge est une garde de non-régression :
// la sémantique UTILE de Loadmemo (RAM réamorcée + cartouche routée) doit survivre
// au passage de hardReset() à resetRAM()+softReset(). Elle passe avant comme après
// le fix ; elle attrape un refactor qui oublierait la RAZ RAM ou casserait le
// routage cartouche.
func TestMountCartridgeResetsRAMAndMapsCartridge(t *testing.T) {
	const addr = 0x6000             // RAM utilisateur
	resetVal := newGA().Read8(addr) // valeur après reset propre (référence)

	g := newGA()
	g.Write8(addr, resetVal^0xFF) // salir avec une valeur distincte
	if g.Read8(addr) == resetVal {
		t.Fatalf("préparation : la valeur salie doit différer de la valeur de reset")
	}

	cart := &fakeCartridge{data: make([]byte, 0x4000)}
	cart.data[0x0100] = 0xCC // octet repère
	g.MountCartridge(cart)

	// Cartouche routée dans l'état par défaut (e7c3 bit2=0 : espace ROM = cartouche).
	// NB : ce test couvre le routage depuis l'état de reset. Le cas d'un montage à
	// chaud par-dessus un e7c3 bit2=1 préservé (BASIC interne) relève du câblage
	// média à chaud (lot ultérieur) — cf. note « hors périmètre » de l'issue #132.
	if v := g.Read8(0x0100); v != 0xCC {
		t.Errorf("après montage : 0x0100 = 0x%02X, want 0xCC (cartouche routée)", v)
	}
	// RAM réamorcée.
	if v := g.Read8(addr); v != resetVal {
		t.Errorf("après montage : RAM[0x%04X] = 0x%02X (RAM non réamorcée), want 0x%02X",
			addr, v, resetVal)
	}
}

// TestMountCartridgeForcesStandardVideoMode prouve la sémantique vidéo fidèle du
// montage : Initprog force le décodage à 320x16 (réf C : Decodevideo = Decode320x16,
// dcto8demulation.c:330 — SANS relire e7dc), tandis que le registre e7dc (port[0x1c])
// est PRÉSERVÉ (réf C Loadmemo ne touche pas port[]). C'est l'état post-Initprog
// fidèle : décodage standard + e7dc conservé, que le firmware réécrit ensuite.
//
// Verrou anti-mutation : un « réalignement » de vmode sur e7dc dans initprog — ou
// une normalisation de e7dc — divergerait du C et ferait échouer ce test. Il
// distingue 320x16 de 640x2 par le motif décodé (320x16 : chaque bit = 2 px ;
// 640x2 : 1 px par bit), donc un faux mode produirait d'autres pixels.
func TestMountCartridgeForcesStandardVideoMode(t *testing.T) {
	g := newGA()
	g.Write8(0xE7DC, 0x2a) // mode 640x2 (80 colonnes) AVANT montage

	g.MountCartridge(&fakeCartridge{data: make([]byte, 0x4000)})

	// e7dc préservé (Loadmemo ne touche pas les ports) — bien que le décodage repasse
	// en standard : c'est l'« incohérence » fidèle de l'état post-Initprog.
	if v := g.Read8(0xE7DC); v != 0x2a {
		t.Errorf("après montage : e7dc = 0x%02X, want 0x2a préservé (port non touché)", v)
	}

	// Décodage forcé en 320x16 (mêmes octets que TestDecode320x16Doubling) :
	// 4 px forme puis 12 px fond. En 640x2 ces octets donneraient un autre motif.
	setColor(g, 1, 1, 0, 0) // fond
	setColor(g, 2, 0, 2, 0) // forme
	writeColorByte(g, 0xD1) // bg=1 fg=2
	writeFormByte(g, 0xC0)  // 2 bits hauts = forme
	fb := newFrame()
	g.DecodeFrame(fb)
	for k := 0; k < 4; k++ {
		if v := fb[firstActivePixel(k)]; v != wantColor(0, 2, 0) {
			t.Errorf("après montage, pixel %d = 0x%08X, want forme (décodage 320x16 forcé)", k, v)
		}
	}
	for k := 4; k < 16; k++ {
		if v := fb[firstActivePixel(k)]; v != wantColor(1, 0, 0) {
			t.Errorf("après montage, pixel %d = 0x%08X, want fond (décodage 320x16 forcé)", k, v)
		}
	}
}
