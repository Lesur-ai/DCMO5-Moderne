package overlay_test

import (
	"testing"

	"github.com/Lesur-ai/dcmo5/internal/machine"
	"github.com/Lesur-ai/dcmo5/internal/overlay"
	"github.com/Lesur-ai/dcmo5/internal/uimodel"

	// Enregistrement des vrais profils pour le test data-driven MO5/TO8D ci-dessous.
	_ "github.com/Lesur-ai/dcmo5/internal/machine/mo5"
	_ "github.com/Lesur-ai/dcmo5/internal/machine/to8d"
	_ "github.com/Lesur-ai/dcmo5/internal/machine/to9p"
)

func TestModel_ZeroValueIsClosed(t *testing.T) {
	var m overlay.Model
	if m.IsOpen() || m.State() != overlay.StateClosed {
		t.Fatalf("zéro-value doit être fermé : state=%v open=%v", m.State(), m.IsOpen())
	}
}

func TestModel_OpenClose(t *testing.T) {
	var m overlay.Model
	m.Open()
	if !m.IsOpen() || m.State() != overlay.StateMain {
		t.Fatalf("Open → StateMain ouvert, got %v", m.State())
	}
	m.Close()
	if m.IsOpen() || m.State() != overlay.StateClosed {
		t.Fatalf("Close → fermé, got %v", m.State())
	}
}

// Open sur un overlay DÉJÀ ouvert (en navigateur) ne doit pas réinitialiser la vue :
// sinon un réappui parasite ferait perdre la navigation en cours.
func TestModel_OpenIsNoopWhenAlreadyOpen(t *testing.T) {
	var m overlay.Model
	m.Open()
	m.GoBrowse("tape")
	m.Open()
	if m.State() != overlay.StateBrowse || m.BrowseKey() != "tape" {
		t.Fatalf("Open sur overlay ouvert a réinitialisé la navigation : state=%v key=%q", m.State(), m.BrowseKey())
	}
}

func TestModel_BackStackBrowseToMainToClosed(t *testing.T) {
	var m overlay.Model
	m.Open()
	m.GoBrowse("disk")
	if m.State() != overlay.StateBrowse || m.BrowseKey() != "disk" {
		t.Fatalf("GoBrowse → Browse(disk), got state=%v key=%q", m.State(), m.BrowseKey())
	}
	m.Back() // Browse → Main + browseKey effacé
	if m.State() != overlay.StateMain || m.BrowseKey() != "" {
		t.Fatalf("Back depuis Browse → Main sans browseKey, got state=%v key=%q", m.State(), m.BrowseKey())
	}
	m.Back() // Main → fermé
	if m.IsOpen() {
		t.Fatalf("Back depuis Main doit fermer l'overlay, got state=%v", m.State())
	}
	m.Back() // déjà fermé : no-op
	if m.IsOpen() {
		t.Fatalf("Back sur overlay fermé doit rester fermé, got %v", m.State())
	}
}

// Back depuis la confirmation de changement de machine revient à la vue principale
// (ne ferme PAS l'overlay) : l'utilisateur annule le switch sans quitter l'overlay.
func TestModel_BackFromConfirmSwitchGoesToMain(t *testing.T) {
	var m overlay.Model
	m.Open()
	m.GoConfirmSwitch()
	if m.State() != overlay.StateConfirmSwitch || m.BrowseKey() != "" {
		t.Fatalf("GoConfirmSwitch, got state=%v key=%q", m.State(), m.BrowseKey())
	}
	m.Back()
	if m.State() != overlay.StateMain {
		t.Fatalf("Back depuis ConfirmSwitch → Main, got %v", m.State())
	}
}

func TestModel_ToggleClosesFromAnyView(t *testing.T) {
	var m overlay.Model
	m.Toggle() // fermé → ouvert (Main)
	if !m.IsOpen() || m.State() != overlay.StateMain {
		t.Fatalf("Toggle depuis fermé → Main ouvert, got %v", m.State())
	}
	m.GoBrowse("cart")
	m.Toggle() // ouvert (en navigateur) → tout fermer
	if m.IsOpen() || m.BrowseKey() != "" {
		t.Fatalf("Toggle depuis ouvert (Browse) doit fermer, got state=%v key=%q", m.State(), m.BrowseKey())
	}
	m.Open()
	m.GoConfirmSwitch()
	m.Toggle() // ouvert (confirmation) → fermer aussi
	if m.IsOpen() {
		t.Fatalf("Toggle depuis ouvert (ConfirmSwitch) doit fermer, got state=%v", m.State())
	}
}

// Invariant : browseKey ne doit JAMAIS rester non vide hors de StateBrowse. On part
// du navigateur (clé posée) et on vérifie que CHAQUE sortie l'efface.
func TestModel_BrowseKeyClearedLeavingBrowse(t *testing.T) {
	leave := map[string]func(*overlay.Model){
		"GoMain":          (*overlay.Model).GoMain,
		"GoConfirmSwitch": (*overlay.Model).GoConfirmSwitch,
		"Close":           (*overlay.Model).Close,
		"Toggle":          (*overlay.Model).Toggle,
		"Back":            (*overlay.Model).Back,
	}
	for name, leaveFn := range leave {
		var m overlay.Model
		m.Open()
		m.GoBrowse("tape")
		if m.BrowseKey() != "tape" {
			t.Fatalf("%s: précondition GoBrowse a échoué", name)
		}
		leaveFn(&m)
		if m.State() == overlay.StateBrowse {
			t.Errorf("%s: ne doit pas rester en StateBrowse", name)
		}
		if m.BrowseKey() != "" {
			t.Errorf("%s: browseKey doit être vide hors StateBrowse, got %q", name, m.BrowseKey())
		}
	}
}

func TestShouldPause(t *testing.T) {
	cases := []struct {
		name               string
		paused, open, want bool
	}{
		{"actif, overlay fermé", false, false, false},
		{"overlay ouvert gèle", false, true, true},
		{"F3-pause SANS overlay reste en pause", true, false, true}, // cas de régression clé
		{"les deux", true, true, true},
	}
	for _, c := range cases {
		if got := overlay.ShouldPause(c.paused, c.open); got != c.want {
			t.Errorf("%s : ShouldPause(paused=%v, open=%v) = %v, want %v", c.name, c.paused, c.open, got, c.want)
		}
	}
}

// TestDescribeLive_RealProfiles vérifie l'acceptance #117/#190 « data-driven » :
// l'overlay n'expose QUE les médias LiveMutable (tape/disk/cart) et JAMAIS les
// paramètres boot-only (rom système, rom contrôleur). Test sur les VRAIS profils
// enregistrés (pas un profil factice) — il échoue si un profil marquait par erreur la
// ROM système LiveMutable, ou oubliait un média.
func TestDescribeLive_RealProfiles(t *testing.T) {
	for _, id := range []string{"mo5", "to8d", "to9p"} {
		prof, ok := machine.ByID(id)
		if !ok {
			t.Fatalf("profil %q introuvable dans le registre", id)
		}
		live := uimodel.DescribeLive(prof, machine.Config{})
		keys := map[string]bool{}
		for _, d := range live {
			keys[d.Key] = true
			if !d.LiveMutable {
				t.Errorf("%s: DescribeLive a laissé passer un Param non LiveMutable : %q", id, d.Key)
			}
		}
		for _, want := range []string{machine.KeyTape, machine.KeyDisk, machine.KeyCart} {
			if !keys[want] {
				t.Errorf("%s: média LiveMutable %q absent de DescribeLive", id, want)
			}
		}
		for _, forbidden := range []string{machine.KeyROM, machine.KeyDiskROM} {
			if keys[forbidden] {
				t.Errorf("%s: paramètre boot-only %q NE doit PAS apparaître dans l'overlay", id, forbidden)
			}
		}
	}
}
