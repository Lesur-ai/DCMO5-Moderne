package gatearray_test

// Tests d'intégration des traps d'E/S TO8D (#115) : disque secteur (params
// 0x6049–0x6050), cassette octet (0x2045), crayon/souris/clic, imprimante, son.
// Un vrai CPU 6809 est attaché au gate-array (les handlers lisent/écrivent ses
// registres) ; les médias sont des mocks.

import (
	"io"
	"testing"

	"github.com/Lesur-ai/dcmo5/internal/cpu6809"
	"github.com/Lesur-ai/dcmo5/internal/machine/gatearray"
)

func newGAWithCPU() (*gatearray.GateArray, *cpu6809.CPU) {
	g := newGA()
	cpu := cpu6809.New(g)
	g.AttachCPU(cpu)
	return g, cpu
}

// ── Médias mock ───────────────────────────────────────────────────────────────

type fakeDisk struct {
	sector    [256]byte
	formatted bool
}

func (d *fakeDisk) ReadSector(_, _, _ int) ([256]byte, error)  { return d.sector, nil }
func (d *fakeDisk) WriteSector(_, _, _ int, v [256]byte) error { d.sector = v; return nil }
func (d *fakeDisk) FormatUnit(_ int) error                     { d.formatted = true; return nil }

type fakeTape struct {
	data    []byte
	pos     int
	written []byte
}

func (t *fakeTape) ReadByte() (byte, error) {
	if t.pos >= len(t.data) {
		return 0, io.EOF
	}
	b := t.data[t.pos]
	t.pos++
	return b, nil
}
func (t *fakeTape) WriteByte(b byte) error { t.written = append(t.written, b); return nil }
func (t *fakeTape) Rewind() error          { t.pos = 0; return nil }
func (t *fakeTape) Position() int64        { return int64(t.pos) }

type fakePrinter struct{ out []byte }

func (p *fakePrinter) WriteByte(b byte) error { p.out = append(p.out, b); return nil }

// ── Son ───────────────────────────────────────────────────────────────────────

func TestSoundLevel(t *testing.T) {
	g := newGA()
	g.Write8(0xE7CF, 0x04) // sélectionne le registre musique
	g.Write8(0xE7CD, 0x2A) // niveau son
	if v := g.SoundLevel(); v != 0x2A {
		t.Errorf("SoundLevel = 0x%02X, want 0x2A", v)
	}
	g.Write8(0xE7CF, 0x00) // bit musique absent → e7cd ne change plus le son
	g.Write8(0xE7CD, 0x3F)
	if v := g.SoundLevel(); v != 0x2A {
		t.Errorf("SoundLevel = 0x%02X, want inchangé 0x2A", v)
	}
}

func TestReadE7CDMusic(t *testing.T) {
	g := newGA()
	g.Write8(0xE7CF, 0x04) // mode musique
	g.Write8(0xE7CD, 0x2A) // niveau son
	// Inc J1a : en mode musique e7cd retourne `joysAction | sound` (ref C
	// dcto8demulation.c Mgetto8d). Au repos joysAction = 0xC0 (boutons fire
	// J1/J2 relâchés, bits 0..5 à 1), donc l'OR ajoute 0xC0 au niveau son.
	// Pré-Inc J1a : ce test attendait 0x2A car joysAction était ignoré (bug
	// silencieux tant que joysAction restait à 0 — il l'était car la struct
	// n'avait pas le champ).
	if v := g.Read8(0xE7CD); v != 0xEA {
		t.Errorf("e7cd lu en mode musique = 0x%02X, want 0xEA (= joysAction 0xC0 | sound 0x2A)", v)
	}
	g.Write8(0xE7CF, 0x00) // mode action : e7cd reflète port[0x0d]
	g.Write8(0xE7CD, 0x11)
	if v := g.Read8(0xE7CD); v != 0x11 {
		t.Errorf("e7cd lu en mode action = 0x%02X, want 0x11", v)
	}
}

// TestSetJoystick_RestAfterInitprog (Inc J1a, codex P2) : Initprog() est un
// reset DOUX (RAM/ports conservés) mais doit relâcher les entrées transitoires
// — y compris le joystick. Sans ce reset, déclencher Initprog (bouton overlay,
// media error, cartouche montée) après une partie laisserait des bits
// direction/fire appuyés visibles côté CPU via 0xe7cc/0xe7cd, simulant un
// joystick agité même si l'hôte a relâché. Symétrie avec le clavier qui est
// déjà reset dans initprog().
func TestSetJoystick_RestAfterInitprog(t *testing.T) {
	g := newGA()
	// Simule J1 nord + J2 fire appuyés.
	g.SetJoystick(0xFE, 0x40)
	g.Initprog()
	g.Write8(0xE7CE, 0x04)
	g.Write8(0xE7CF, 0x04)
	g.Write8(0xE7CD, 0x00) // sound = 0 pour isoler joysAction
	if v := g.Read8(0xE7CC); v != 0xFF {
		t.Errorf("e7cc après Initprog = 0x%02X, want 0xFF (toutes directions relâchées)", v)
	}
	if v := g.Read8(0xE7CD); v != 0xC0 {
		t.Errorf("e7cd après Initprog = 0x%02X, want 0xC0 (boutons fire relâchés)", v)
	}
}

// TestSetJoystick_RestAfterHardReset (Inc J1a) : après Reset, joysPosition et
// joysAction retombent sur leur valeur de repos (0xFF, 0xC0) — la convention
// LOGIQUE INVERSÉE où 0 = appuyé. Garde-fou contre une zéro-value Go non
// reset qui ferait croire à toutes directions appuyées (cf. machine.NeutralJoystick).
func TestSetJoystick_RestAfterHardReset(t *testing.T) {
	g := newGA()
	// Simule une partie où toutes les directions/boutons J1 sont appuyées.
	g.SetJoystick(0x00, 0x00)
	g.Reset()
	g.Write8(0xE7CE, 0x04) // mux : e7cc → joysPosition
	g.Write8(0xE7CF, 0x04) // mux : e7cd → joysAction | sound (sound = 0 après Reset)
	if v := g.Read8(0xE7CC); v != 0xFF {
		t.Errorf("e7cc après Reset = 0x%02X, want 0xFF (toutes directions relâchées)", v)
	}
	if v := g.Read8(0xE7CD); v != 0xC0 {
		t.Errorf("e7cd après Reset = 0x%02X, want 0xC0 (boutons fire relâchés, sound=0)", v)
	}
}

// TestSetJoystick_PositionMuxed (Inc J1a) : lecture de e7cc retourne joysPosition
// quand port[0x0e]&4 = 1, sinon port[0x0c]. Vérifie le mux hardware et que
// SetJoystick atteint bien le registre. Prouve la résolution du « manque » e7cc
// pré-J1a (aucune entrée case dans readIO).
func TestSetJoystick_PositionMuxed(t *testing.T) {
	g := newGA()
	g.SetJoystick(0xAB, 0xCD)

	// Mux désactivé : retourne port[0x0c] (= 0 au boot).
	g.Write8(0xE7CE, 0x00)
	if v := g.Read8(0xE7CC); v != 0x00 {
		t.Errorf("e7cc sans mux = 0x%02X, want port[0x0c]=0", v)
	}

	// Mux activé : retourne joysPosition.
	g.Write8(0xE7CE, 0x04)
	if v := g.Read8(0xE7CC); v != 0xAB {
		t.Errorf("e7cc avec mux = 0x%02X, want joysPosition=0xAB", v)
	}
}

// TestSetJoystick_ActionFireButton (Inc J1a) : c'est le test du VRAI fix B4.
// Bouton fire J1 appuyé = bit 6 à 0 dans joysAction. Lu via e7cd en mode
// musique avec sound = 0, on doit avoir bit 6 à 0 (0x80 = 1000_0000). Pré-fix,
// le bit 6 était silencieusement supprimé car e7cd retournait g.sound seul.
func TestSetJoystick_ActionFireButton(t *testing.T) {
	g := newGA()
	// Bouton fire J1 appuyé (bit 6 = 0), J2 relâché (bit 7 = 1) : 0x80.
	g.SetJoystick(0xFF, 0x80)

	g.Write8(0xE7CF, 0x04) // mux : e7cd → joysAction | sound
	g.Write8(0xE7CD, 0x00) // sound = 0
	if v := g.Read8(0xE7CD); v != 0x80 {
		t.Errorf("e7cd avec fire J1 = 0x%02X, want 0x80 (= joysAction bit 6 à 0)", v)
	}

	// Sans mux : retourne port[0x0d] (= 0).
	g.Write8(0xE7CF, 0x00)
	if v := g.Read8(0xE7CD); v != 0x00 {
		t.Errorf("e7cd sans mux = 0x%02X, want port[0x0d]=0", v)
	}
}

// TestSetJoystick_ActionOrsWithSound (Inc J1a) : preuve fonctionnelle que e7cd
// fait bien l'OR entre joysAction ET sound (= fix B4, ref C). Niveau son 0x2A
// + bouton fire J2 appuyé (bit 7 = 0, J1 relâché = 0x40) doit donner 0x40 |
// 0x2A = 0x6A.
func TestSetJoystick_ActionOrsWithSound(t *testing.T) {
	g := newGA()
	g.SetJoystick(0xFF, 0x40) // J1 fire relâché (bit 6 = 1), J2 fire appuyé (bit 7 = 0)
	g.Write8(0xE7CF, 0x04)    // mode musique/joystick
	g.Write8(0xE7CD, 0x2A)    // sound = 0x2A
	if v := g.Read8(0xE7CD); v != 0x6A {
		t.Errorf("e7cd fire J2 + sound 0x2A = 0x%02X, want 0x6A (= joysAction 0x40 | sound 0x2A)", v)
	}
}

func TestReadE7C3PenButton(t *testing.T) {
	g, _ := newGAWithCPU()
	if g.Read8(0xE7C3)&0x02 != 0 {
		t.Error("bit1 (penbutton) devrait être 0 au repos")
	}
	g.SetPointer(10, 10, true) // bouton pressé
	if g.Read8(0xE7C3)&0x02 == 0 {
		t.Error("bit1 (penbutton) devrait refléter le clic dans e7c3")
	}
	if g.Read8(0xE7C3)&0x80 == 0 {
		t.Error("bit7 devrait rester armé")
	}
}

// ── Disque ────────────────────────────────────────────────────────────────────

func TestTrapDiskReadSector(t *testing.T) {
	g, _ := newGAWithCPU()
	disk := &fakeDisk{}
	for i := range disk.sector {
		disk.sector[i] = byte(i)
	}
	g.MountDisk(disk)
	g.Write8(0x6049, 0) // unité
	g.Write8(0x604A, 0)
	g.Write8(0x604B, 1)    // piste
	g.Write8(0x604C, 2)    // secteur
	g.Write8(0x604F, 0x80) // adresse de destination = 0x8000
	g.Write8(0x6050, 0x00)
	g.Trap(0x14)
	for i := 0; i < 256; i++ {
		if v := g.Read8(0x8000 + uint16(i)); v != byte(i) {
			t.Fatalf("secteur[%d] = 0x%02X, want 0x%02X", i, v, byte(i))
		}
	}
}

func TestTrapDiskNoMedia(t *testing.T) {
	g, cpu := newGAWithCPU()
	g.Trap(0x14)
	if v := g.Read8(0x604E); v != 70 {
		t.Errorf("0x604e = %d, want 70 (erreur 71-1, lecteur non prêt)", v)
	}
	if cpu.RegCC()&0x01 == 0 {
		t.Error("carry devrait être positionné (erreur disque)")
	}
}

func TestTrapDiskWriteThenFormat(t *testing.T) {
	g, _ := newGAWithCPU()
	disk := &fakeDisk{}
	g.MountDisk(disk)
	// Remplir la source 0x8000 puis écrire le secteur.
	for i := 0; i < 256; i++ {
		g.Write8(0x8000+uint16(i), byte(255-i))
	}
	g.Write8(0x6049, 0)
	g.Write8(0x604A, 0)
	g.Write8(0x604B, 0)
	g.Write8(0x604C, 1)
	g.Write8(0x604F, 0x80)
	g.Write8(0x6050, 0x00)
	g.Trap(0x15) // writeSector
	if disk.sector[0] != 255 || disk.sector[255] != 0 {
		t.Errorf("secteur écrit incohérent: [0]=%d [255]=%d", disk.sector[0], disk.sector[255])
	}
	g.Trap(0x18) // formatDisk
	if !disk.formatted {
		t.Error("FormatUnit non appelé sur trap 0x18")
	}
}

// ── Cassette ──────────────────────────────────────────────────────────────────

func TestTrapCassetteRead(t *testing.T) {
	g, cpu := newGAWithCPU()
	// 0x2045 est dans l'espace ROM en TO8D : on active le recouvrement RAM
	// write-enabled (e7e6) pour que le firmware (et ce test) puisse y écrire.
	g.Write8(0xE7E6, 0x60)
	tape := &fakeTape{data: []byte{0xAB, 0xCD}}
	g.MountTape(tape)
	g.Trap(0x42)
	if cpu.RegA() != 0xAB {
		t.Errorf("A = 0x%02X, want 0xAB (1er octet)", cpu.RegA())
	}
	if v := g.Read8(0x2045); v != 0xAB {
		t.Errorf("0x2045 = 0x%02X, want 0xAB", v)
	}
	g.Trap(0x42)
	if cpu.RegA() != 0xCD {
		t.Errorf("A = 0x%02X, want 0xCD (2e octet)", cpu.RegA())
	}
}

func TestTrapCassetteWrite(t *testing.T) {
	g, cpu := newGAWithCPU()
	tape := &fakeTape{}
	g.MountTape(tape)
	cpu.SetRegA(0x5A)
	g.Trap(0x45) // writeOctetK7
	if len(tape.written) != 1 || tape.written[0] != 0x5A {
		t.Errorf("cassette écrit %v, want [0x5A]", tape.written)
	}
}

// ── Crayon / souris / imprimante ──────────────────────────────────────────────

func TestTrapPen(t *testing.T) {
	g, cpu := newGAWithCPU()
	g.SetPointer(100, 50, false)
	g.Trap(0x4b)
	if cpu.RegX() != 50 { // mode par défaut (320) → x divisé par 2
		t.Errorf("X = %d, want 50 (100>>1)", cpu.RegX())
	}
	if cpu.RegY() != 50 {
		t.Errorf("Y = %d, want 50", cpu.RegY())
	}
	if cpu.RegCC()&0x01 != 0 {
		t.Error("carry devrait être clear (détection OK)")
	}
}

func TestTrapPenOutOfBounds(t *testing.T) {
	g, cpu := newGAWithCPU()
	g.SetPointer(700, 50, false) // x >= 640
	g.Trap(0x4b)
	if cpu.RegCC()&0x01 == 0 {
		t.Error("carry devrait être positionné (hors limites)")
	}
}

func TestTrapPen80Columns(t *testing.T) {
	g, cpu := newGAWithCPU()
	g.Write8(0xE7DC, 0x2a) // mode 80 colonnes → pleine résolution X
	g.SetPointer(600, 50, false)
	g.Trap(0x4b)
	if cpu.RegX() != 600 {
		t.Errorf("X = %d, want 600 (80 colonnes, pas de division)", cpu.RegX())
	}
}

func TestTrapMousePosition(t *testing.T) {
	g, cpu := newGAWithCPU()
	g.Write8(0xE7DC, 0x2a) // 80 colonnes
	g.SetPointer(300, 80, false)
	g.Trap(0x4e) // souris : registres X/Y + RAM 0x60d8 (x) / 0x60d6 (y)
	if cpu.RegX() != 300 {
		t.Errorf("X = %d, want 300", cpu.RegX())
	}
	if v := uint16(g.Read8(0x60D8))<<8 | uint16(g.Read8(0x60D9)); v != 300 {
		t.Errorf("RAM 0x60d8 = %d, want 300", v)
	}
	if v := uint16(g.Read8(0x60D6))<<8 | uint16(g.Read8(0x60D7)); v != 80 {
		t.Errorf("RAM 0x60d6 = %d, want 80", v)
	}
}

func TestTrapMouseButton(t *testing.T) {
	g, cpu := newGAWithCPU()
	g.SetPointer(0, 0, false)
	g.Trap(0x52)
	if cpu.RegA() != 3 {
		t.Errorf("A = %d, want 3 (bouton relâché)", cpu.RegA())
	}
	g.SetPointer(0, 0, true)
	g.Trap(0x52)
	if cpu.RegA() != 0 {
		t.Errorf("A = %d, want 0 (bouton pressé)", cpu.RegA())
	}
	if cpu.RegCC()&0x05 != 0x05 {
		t.Errorf("CC = 0x%02X, want bits 0 et 2 armés (clic)", cpu.RegCC())
	}
}

func TestTrapPrinter(t *testing.T) {
	g, cpu := newGAWithCPU()
	pr := &fakePrinter{}
	g.MountPrinter(pr)
	cpu.SetRegB(0x41)
	g.Trap(0x51)
	if len(pr.out) != 1 || pr.out[0] != 0x41 {
		t.Errorf("imprimante reçu %v, want [0x41]", pr.out)
	}
	if cpu.RegCC()&0x01 != 0 {
		t.Error("carry devrait être clear après impression")
	}
}

// ── Éjection cartouche ──────────────────────────────────────────────────────────

// TestEjectCartridgeRelaunchesMachine prouve que l'éjection RELANCE la machine (réf C
// Loadmemo("") : Initprog→Reset6809), et ne se borne pas à recalculer le banc.
//
// Discriminant non-complaisant : après éjection le CPU est resété — CC ← ResetCC et
// PC ← vecteur de reset (la machine repart sur la ROM système). L'ancien code
// (updateROMBank seul) ne touche JAMAIS au CPU : il laisse CC et PC « sales » (RED).
func TestEjectCartridgeRelaunchesMachine(t *testing.T) {
	g, cpu := newGAWithCPU()

	// Cartouche 64 Ko (4 banques distinctes) + sélection d'une banque non nulle :
	// on simule une machine en train d'exécuter du code cartouche. LoadCartridge est
	// la primitive BAS NIVEAU (sans reset CPU), pour laisser l'état CPU « sale ».
	cart := make([]byte, 0x10000)
	for b := 0; b < 4; b++ {
		for i := 0; i < 0x4000; i++ {
			cart[b*0x4000+i] = byte(0xE0 + b)
		}
	}
	g.LoadCartridge(cart)
	g.Write8(0x0003, 0) // écriture espace ROM → commute la banque cartouche 3
	if v := g.Read8(0x0000); v != 0xE3 {
		t.Fatalf("préparation : banque cartouche 3 = 0x%02X, want 0xE3", v)
	}

	// Vecteur de reset attendu (ROM moniteur) + état CPU « sale » avant éjection.
	wantPC := uint16(g.Read8(0xFFFE))<<8 | uint16(g.Read8(0xFFFF))
	cpu.SetRegCC(0xAB) // sentinelle ≠ ResetCC
	if cpu.Snapshot().PC == wantPC {
		t.Fatalf("précondition cassée : PC vaut déjà le vecteur reset 0x%04X avant éjection", wantPC)
	}

	g.EjectCartridge()

	// 1) Relance CPU (le discriminant du correctif).
	if got := cpu.Snapshot().PC; got != wantPC {
		t.Errorf("après éjection : PC = 0x%04X, want 0x%04X (vecteur reset → relance ROM système)", got, wantPC)
	}
	if got := cpu.RegCC(); got != cpu6809.ResetCC {
		t.Errorf("après éjection : CC = 0x%02X, want 0x%02X (ResetCC)", got, cpu6809.ResetCC)
	}

	// 2) Non-régression : cartouche réellement éjectée (banc revenu à 0, car[] effacé).
	if v := g.Read8(0x0000); v != 0x00 {
		t.Errorf("après éjection : 0x0000 = 0x%02X, want 0x00 (cartouche effacée, banc 0)", v)
	}
}
