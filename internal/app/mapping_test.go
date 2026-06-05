package app_test

import (
	"strings"
	"testing"

	"github.com/Lesur-ai/dcmo5/internal/app"
	"github.com/Lesur-ai/dcmo5/internal/spec"
	"github.com/hajimehoshi/ebiten/v2"
)

func TestLogicalSizeMatchesSpec(t *testing.T) {
	w, h := app.LogicalSize()
	if w != spec.FrameWidth || h != spec.FrameHeight {
		t.Errorf("LogicalSize() = (%d,%d), want (%d,%d)", w, h, spec.FrameWidth, spec.FrameHeight)
	}
}

func TestLogicalSizeStable(t *testing.T) {
	w1, h1 := app.LogicalSize()
	w2, h2 := app.LogicalSize()
	if w1 != w2 || h1 != h2 {
		t.Errorf("LogicalSize() instable")
	}
}

func TestKeyMappingNoDuplicates(t *testing.T) {
	seen := map[int]string{}
	// Doublons légitimes : SHIFT (0x38), CNT (0x35), ACC (0x36) ont chacun une
	// variante gauche et droite sur le même index MO5.
	legit := map[int]bool{0x38: true, 0x35: true, 0x36: true}
	for eKey, mo5Key := range app.KeyMapping() {
		if legit[mo5Key] {
			continue
		}
		if prev, dup := seen[mo5Key]; dup {
			t.Errorf("doublon MO5 0x%02X: %v et %v", mo5Key, prev, eKey)
		}
		seen[mo5Key] = eKey.String()
	}
}

func TestKeyMappingValidRange(t *testing.T) {
	for eKey, mo5Key := range app.KeyMapping() {
		if mo5Key < 0 || mo5Key >= spec.KeyMax {
			t.Errorf("touche %v → index MO5 %d hors-bornes [0,%d)", eKey, mo5Key, spec.KeyMax)
		}
	}
}

// TestKeyMapping_NoCharacterKeys est une garde anti-régression : les touches de
// caractère (lettres, chiffres) ne doivent JAMAIS être mappées en positionnel.
// Elles passent par la saisie caractère (CharToMO5Key + AppendInputChars), qui
// respecte le layout OS. Les mapper ici réintroduirait le bug AZERTY→QWERTY :
// un « a » AZERTY serait lu à la position « q » d'un clavier QWERTY.
func TestKeyMapping_NoCharacterKeys(t *testing.T) {
	letters := []ebiten.Key{
		ebiten.KeyA, ebiten.KeyB, ebiten.KeyC, ebiten.KeyD, ebiten.KeyE,
		ebiten.KeyF, ebiten.KeyG, ebiten.KeyH, ebiten.KeyI, ebiten.KeyJ,
		ebiten.KeyK, ebiten.KeyL, ebiten.KeyM, ebiten.KeyN, ebiten.KeyO,
		ebiten.KeyP, ebiten.KeyQ, ebiten.KeyR, ebiten.KeyS, ebiten.KeyT,
		ebiten.KeyU, ebiten.KeyV, ebiten.KeyW, ebiten.KeyX, ebiten.KeyY, ebiten.KeyZ,
	}
	digits := []ebiten.Key{
		ebiten.Key0, ebiten.Key1, ebiten.Key2, ebiten.Key3, ebiten.Key4,
		ebiten.Key5, ebiten.Key6, ebiten.Key7, ebiten.Key8, ebiten.Key9,
	}
	mapping := app.KeyMapping()
	for _, k := range append(letters, digits...) {
		if mo5, found := mapping[k]; found {
			t.Errorf("touche caractère %v mappée en positionnel (→0x%02X) : "+
				"casse l'indépendance de layout (AZERTY→QWERTY)", k, mo5)
		}
	}
}

// ── TitleForState ─────────────────────────────────────────────────────────────

func TestTitle_Normal(t *testing.T) {
	got := app.TitleForState(false, false, "mo5.rom", "", "")
	if !strings.Contains(got, "mo5.rom") {
		t.Errorf("titre normal: %q ne contient pas 'mo5.rom'", got)
	}
	if strings.Contains(got, "PAUSE") || strings.Contains(got, "manquante") {
		t.Errorf("titre normal ne doit pas contenir PAUSE/manquante: %q", got)
	}
}

func TestTitle_ROMmissing(t *testing.T) {
	got := app.TitleForState(true, false, "", "", "")
	if !strings.Contains(got, "manquante") {
		t.Errorf("ROM manquante: %q ne contient pas 'manquante'", got)
	}
}

func TestTitle_Paused(t *testing.T) {
	got := app.TitleForState(false, true, "mo5.rom", "", "")
	if !strings.Contains(got, "[PAUSE]") {
		t.Errorf("pause: %q ne contient pas '[PAUSE]'", got)
	}
}

func TestTitle_WithTape(t *testing.T) {
	got := app.TitleForState(false, false, "mo5.rom", "jeu.k7", "")
	if !strings.Contains(got, "jeu.k7") {
		t.Errorf("avec tape: %q ne contient pas 'jeu.k7'", got)
	}
}

func TestTitle_PausedROMmissing(t *testing.T) {
	got := app.TitleForState(true, true, "", "", "")
	if !strings.Contains(got, "manquante") || !strings.Contains(got, "[PAUSE]") {
		t.Errorf("pause+ROM manquante: %q", got)
	}
}
