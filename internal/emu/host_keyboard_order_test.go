package emu

// host_keyboard_order_test.go — Inc Kc : Host.tick applique les modificateurs
// (SHIFT/CNT/ACC) AVANT les autres touches, par construction. Sur TO8D le
// gate-array latch le scancode caractère avec l'état modificateurs courant ;
// l'ordre est donc critique. Sur MO5 la matrice est passive (insensible à
// l'ordre) : Kc ne change rien pour MO5 — c'est ce que prouve OrderDoesNotAffectMO5.

import (
	"testing"

	"github.com/Lesur-ai/dcmo5/internal/cpu6809"
	"github.com/Lesur-ai/dcmo5/internal/core"
	"github.com/Lesur-ai/dcmo5/internal/keyboard"
	"github.com/Lesur-ai/dcmo5/internal/machine"
	"github.com/Lesur-ai/dcmo5/internal/machine/mo5"
	"github.com/Lesur-ai/dcmo5/internal/media"
)

// setKeyCall enregistre un appel SetKey reçu, utilisé pour observer l'ordre
// d'application des touches dans Host.tick sans dépendre du gate-array réel.
type setKeyCall struct {
	k       int
	pressed bool
}

// loggedMachine décore une machine.Machine en enregistrant la séquence exacte
// des appels SetKey. Le reste des méthodes délègue à la machine sous-jacente :
// l'instrumentation est strictement non intrusive.
type loggedMachine struct {
	inner machine.Machine
	calls []setKeyCall
	model *keyboard.Model // si non nil, override KeyboardModel (pour test fake)
}

func (l *loggedMachine) SetKey(k machine.Key, pressed bool) {
	l.calls = append(l.calls, setKeyCall{int(k), pressed})
	l.inner.SetKey(k, pressed)
}
func (l *loggedMachine) KeyboardModel() *keyboard.Model {
	if l.model != nil {
		return l.model
	}
	return l.inner.KeyboardModel()
}
func (l *loggedMachine) Step(cycles int) int          { return l.inner.Step(cycles) }
func (l *loggedMachine) Reset()                       { l.inner.Reset() }
func (l *loggedMachine) Initprog()                    { l.inner.Initprog() }
func (l *loggedMachine) SetJoystick(j machine.JoystickInput) {
	l.inner.SetJoystick(j)
}
func (l *loggedMachine) SetPointer(p machine.PointerInput) { l.inner.SetPointer(p) }
func (l *loggedMachine) FrameSize() (int, int)             { return l.inner.FrameSize() }
func (l *loggedMachine) FramebufferInto(dst []uint32)      { l.inner.FramebufferInto(dst) }
func (l *loggedMachine) AudioSampleRate() int              { return l.inner.AudioSampleRate() }
func (l *loggedMachine) DrainAudio(dst []uint8) int        { return l.inner.DrainAudio(dst) }
func (l *loggedMachine) MountTape(t media.Tape)            { l.inner.MountTape(t) }
func (l *loggedMachine) EjectTape()                        { l.inner.EjectTape() }
func (l *loggedMachine) MountDisk(d media.Disk)            { l.inner.MountDisk(d) }
func (l *loggedMachine) EjectDisk()                        { l.inner.EjectDisk() }
func (l *loggedMachine) MountCartridge(c media.Cartridge)  { l.inner.MountCartridge(c) }
func (l *loggedMachine) EjectCartridge()                   { l.inner.EjectCartridge() }
func (l *loggedMachine) MountPrinter(p media.PrinterSink)  { l.inner.MountPrinter(p) }
func (l *loggedMachine) EjectPrinter()                     { l.inner.EjectPrinter() }
func (l *loggedMachine) CPUSnapshot() cpu6809.Snapshot     { return l.inner.CPUSnapshot() }

// TestHostTick_ModifiersFirstInSetKeySequence vérifie l'INVARIANT central de Kc :
// dans la séquence des appels SetKey reçus par la machine pendant un tick, tous
// les indices retournés par Model.ModifierKeys() apparaissent AVANT le premier
// indice non-modificateur. Le test observe la séquence réelle (pas l'ordre
// d'itération de la boucle) via un décorateur — ce que voit la machine est ce
// qui compte sur TO8D pour le latching gate-array.
func TestHostTick_ModifiersFirstInSetKeySequence(t *testing.T) {
	m := nopMachine(t)
	lm := &loggedMachine{inner: mo5.Wrap(m)}
	h := New(lm, 1)
	in := InputState{Keys: make([]bool, core.KeyCount)}
	in.Keys[keyboard.Mo5KeyShift] = true
	in.Keys[keyboard.Mo5KeyCNT] = true
	in.Keys[0x20] = true // ESPACE, non-modificateur
	h.SetInput(in)
	h.tick(1)

	modIndices := map[int]struct{}{}
	for _, k := range keyboard.MO5Model().ModifierKeys() {
		modIndices[k] = struct{}{}
	}
	if len(modIndices) < 2 {
		t.Fatalf("ModifierKeys() MO5 = %v, want au moins SHIFT et CNT", keyboard.MO5Model().ModifierKeys())
	}

	firstNonMod := -1
	for i, c := range lm.calls {
		if _, isMod := modIndices[c.k]; !isMod {
			firstNonMod = i
			break
		}
	}
	if firstNonMod == -1 {
		t.Fatalf("aucun appel non-modificateur dans %d appels — Host.tick devrait boucler sur tous les indices", len(lm.calls))
	}
	// Tous les appels jusqu'à firstNonMod doivent être des modifs.
	for i := 0; i < firstNonMod; i++ {
		if _, isMod := modIndices[lm.calls[i].k]; !isMod {
			t.Fatalf("appel n°%d = SetKey(%#x, %v) avant le premier non-modificateur (idx %d) — modifs doivent être appliquées EN PREMIER", i, lm.calls[i].k, lm.calls[i].pressed, firstNonMod)
		}
	}
	// Tous les modifs déclarés DOIVENT avoir été posés (avec leur état réel).
	seenMods := map[int]bool{}
	for i := 0; i < firstNonMod; i++ {
		seenMods[lm.calls[i].k] = true
	}
	for k := range modIndices {
		if !seenMods[k] {
			t.Errorf("modificateur %#x absent de la passe modifs (séquence : %v)", k, lm.calls[:firstNonMod])
		}
	}
}

// TestHostTick_OrderDoesNotAffectMO5Matrix : la matrice MO5 (scannée passivement
// par la ROM) est insensible à l'ordre d'application des touches. Garde-fou
// anti-régression : Kc ne change rien d'observable côté MO5. SHIFT et A pressés
// simultanément doivent rester pressés après tick, quel que soit l'ordre interne.
func TestHostTick_OrderDoesNotAffectMO5Matrix(t *testing.T) {
	m := nopMachine(t)
	h := New(mo5.Wrap(m), 1)
	in := InputState{Keys: make([]bool, core.KeyCount)}
	in.Keys[keyboard.Mo5KeyShift] = true
	in.Keys[0x2D] = true // 'A' MO5 (cf. charToMO5)
	h.SetInput(in)
	h.tick(1)

	// Lecture matrice via port 0xA7C1 (col = (val&0xFE)>>1, bit 0x80 du retour
	// indique relâchée). Cf. host_test.go TestHost_TickAppliesInput.
	for _, k := range []int{keyboard.Mo5KeyShift, 0x2D} {
		m.Write8(0xA7C1, byte(k)<<1)
		got := m.Read8(0xA7C1)
		if got&0x80 != 0 {
			t.Errorf("touche %#x non pressée dans la matrice MO5 (port=0x%02X) — la 2e passe devrait toutes les poser", k, got)
		}
	}
}

// TestHostTick_ReleasesModifierFrameN1_MO5 : frame N+1 doit propager le
// relâchement d'un modificateur. Si la boucle de 2 passes oubliait d'appeler
// SetKey(SHIFT, false) à la 2e frame, la matrice MO5 verrait SHIFT toujours
// posé (faux positif). C'est un cas que l'ancien `for k := range in.Keys`
// couvrait gratuitement ; la nouvelle structure doit le couvrir aussi.
func TestHostTick_ReleasesModifierFrameN1_MO5(t *testing.T) {
	m := nopMachine(t)
	h := New(mo5.Wrap(m), 1)

	// Frame N : SHIFT + A pressés.
	in := InputState{Keys: make([]bool, core.KeyCount)}
	in.Keys[keyboard.Mo5KeyShift] = true
	in.Keys[0x2D] = true
	h.SetInput(in)
	h.tick(1)

	// Frame N+1 : SHIFT relâché, A maintenu.
	in.Keys[keyboard.Mo5KeyShift] = false
	h.SetInput(in)
	h.tick(1)

	m.Write8(0xA7C1, byte(keyboard.Mo5KeyShift)<<1)
	got := m.Read8(0xA7C1)
	if got&0x80 == 0 {
		t.Errorf("SHIFT toujours pressé frame N+1 (port=0x%02X) — la 2e passe devrait propager false", got)
	}
}

// TestHostTick_NegativeACCSafe : si une machine retourne ACCKey < 0 dans son
// modèle (pas de touche ACC), Host.tick NE DOIT PAS appeler SetKey(-1, …) ni
// indexer in.Keys avec un indice négatif. Garde-fou de robustesse pour les
// machines futures qui n'auraient pas de touche d'accent dédiée.
func TestHostTick_NegativeACCSafe(t *testing.T) {
	// Modèle synthétique : MO5 réel mais override KeyboardModel avec un modèle
	// dont ACCKey = -1. ModifierKeys() doit alors retourner [ShiftKey, CNTKey]
	// uniquement — c'est ce que consomme Host.tick.
	noACC := *keyboard.MO5Model()
	noACC.ACCKey = -1

	m := nopMachine(t)
	lm := &loggedMachine{inner: mo5.Wrap(m), model: &noACC}
	h := New(lm, 1)
	in := InputState{Keys: make([]bool, core.KeyCount)}
	in.Keys[0x2D] = true // 'A' MO5
	h.SetInput(in)
	h.tick(1)

	for i, c := range lm.calls {
		if c.k < 0 {
			t.Fatalf("appel n°%d = SetKey(%d, %v) : indice négatif interdit", i, c.k, c.pressed)
		}
		if c.k >= len(in.Keys) {
			t.Fatalf("appel n°%d = SetKey(%d, %v) : indice hors borne (len=%d)", i, c.k, c.pressed, len(in.Keys))
		}
	}
}
