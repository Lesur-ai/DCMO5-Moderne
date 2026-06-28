package emu

// host_joystick_test.go — Inc J2a : tests d'effet observable du pipeline
// Host.SetInput → Host.tick → machine.SetJoystick → port machine → CPU 6809.
//
// Ces tests valident que le champ InputState.Joystick (ajouté en J2a) est bien
// propagé jusqu'au code utilisateur via le port matériel 0xA7CC/0xA7CD. Sans
// quoi, J0 (NeutralJoystick) + J1a (gate-array TO8D) + J1b (parité bits)
// resteraient inertes — la couche hôte ne saurait pas écrire dedans.

import (
	"testing"

	"github.com/Lesur-ai/dcmo5/internal/core"
	"github.com/Lesur-ai/dcmo5/internal/machine"
	"github.com/Lesur-ai/dcmo5/internal/machine/mo5"
)

// TestHostNew_PublishesNeutralJoystick (J2a.T1) : sans aucun SetInput préalable,
// Host.New() doit initialiser son InputState avec machine.NeutralJoystick (=
// 0xFF/0xC0). Si la zéro-value Go {0x00, 0x00} était utilisée, la machine
// interpréterait l'état comme « toutes directions appuyées + boutons enfoncés »
// dès la première frame — visible et catastrophique. Test observable via le
// port joystick MO5 après un tick.
func TestHostNew_PublishesNeutralJoystick(t *testing.T) {
	m := nopMachine(t)
	h := New(mo5.Wrap(m), 1)
	// Activer le mux joystick côté MO5 (port[0x0E] bit 2 = 1) AVANT le tick :
	// sinon Read8(0xA7CC) renverrait port[0x0C] (= 0) au lieu de joysPosition.
	m.Write8(0xA7CE, 0x04)
	m.Write8(0xA7CF, 0x04)
	h.tick(1)
	if v := m.Read8(0xA7CC); v != 0xFF {
		t.Errorf("Host.New sans SetInput : Read8(0xA7CC) = 0x%02X, want 0xFF (NeutralJoystick.Position)", v)
	}
	if v := m.Read8(0xA7CD); v != 0xC0 {
		t.Errorf("Host.New sans SetInput : Read8(0xA7CD) = 0x%02X, want 0xC0 (NeutralJoystick.Action)", v)
	}
}

// TestHostTick_PublishesCustomJoystick_MO5 (J2a.T2) : SetInput publie un état
// joystick custom, Host.tick le propage à la machine via SetJoystick. Lecture
// directe du port (sans CPU) pour isoler le chemin host → machine du chemin
// machine → CPU.
func TestHostTick_PublishesCustomJoystick_MO5(t *testing.T) {
	m := nopMachine(t)
	h := New(mo5.Wrap(m), 1)
	m.Write8(0xA7CE, 0x04)
	m.Write8(0xA7CF, 0x04)
	// J1 nord + J1 fire appuyés (logique inversée : Position bit 0 = 0, Action
	// bit 6 = 0).
	in := InputState{
		Keys:     make([]bool, core.KeyCount),
		Joystick: machine.JoystickInput{Position: 0xFE, Action: 0x80},
	}
	h.SetInput(in)
	h.tick(1)
	if v := m.Read8(0xA7CC); v != 0xFE {
		t.Errorf("Read8(0xA7CC) après SetInput = 0x%02X, want 0xFE (J1 nord)", v)
	}
	if v := m.Read8(0xA7CD); v != 0x80 {
		t.Errorf("Read8(0xA7CD) après SetInput = 0x%02X, want 0x80 (J1 fire)", v)
	}
}

// TestHostTick_JoystickViaCPU6809_MO5 (J2a.T3) : test BOUT-EN-BOUT. Consomme
// enfin le harness J0 (loadMO5Program). Un petit programme 6809 active le mux
// joystick puis lit 0xA7CC et stocke en RAM[$2000]. Après Host.tick avec un
// joystick custom dans InputState, RAM[$2000] doit contenir la valeur publiée
// — preuve que le pipeline Host.SetInput → Host.tick → machine.SetJoystick
// → port matériel → CPU lit bien la valeur. C'est ce test qui justifie tout
// le harness CPU 6809 de J0.
func TestHostTick_JoystickViaCPU6809_MO5(t *testing.T) {
	// Programme 6809 (13 octets) :
	//   LDA #$04        86 04
	//   STA $A7CE       B7 A7 CE   ; mux position activé
	//   LDA $A7CC       B6 A7 CC   ; lit le port joystick (= joysPosition)
	//   STA $2000       B7 20 00   ; stocke en RAM utilisateur
	//   BRA *           20 FE      ; boucle
	prog := testProgram{
		romOffset: 0x0000,
		code: []byte{
			0x86, 0x04,
			0xB7, 0xA7, 0xCE,
			0xB6, 0xA7, 0xCC,
			0xB7, 0x20, 0x00,
			0x20, 0xFE,
		},
	}
	m := loadMO5Program(t, prog)
	h := New(mo5.Wrap(m), 1)
	// J2 ouest + J2 fire appuyés (Position bit 6 = 0 → 0xBF, Action bit 7 = 0
	// → 0x40). Valeurs distinctes du repos pour éviter une fausse-positive si
	// le pipeline ne publiait rien.
	in := InputState{
		Keys:     make([]bool, core.KeyCount),
		Joystick: machine.JoystickInput{Position: 0xBF, Action: 0x40},
	}
	h.SetInput(in)
	// 64 cycles : largement assez pour exécuter LDA, STA, LDA, STA (≈ 5+5+5+5 =
	// 20 cycles) avant la boucle BRA. La 4e instruction STA $2000 écrit la
	// valeur en RAM utilisateur ; le test la lit ensuite.
	h.tick(64)
	if got := m.Read8(0x2000); got != 0xBF {
		t.Fatalf("RAM[$2000] = 0x%02X après tick, want 0xBF (= joysPosition publié par Host.SetInput, vu par le CPU via 0xA7CC)", got)
	}
}
