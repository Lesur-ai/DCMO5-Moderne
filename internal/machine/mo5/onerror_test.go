package mo5

import (
	"testing"

	"github.com/Lesur-ai/dcmo5/internal/machine"
)

// TestNewFromConfigWiresIOError vérifie que le profil MO5 câble un puits d'erreurs E/S
// (parité avec le boot CLI, #144) : sans ce câblage, les sessions lancées par le launcher
// perdaient toute remontée d'erreur. On override le reporter de paquet (observabilité), on
// construit via newFromConfig, et on déclenche une vraie erreur E/S (READOCTETK7 sans
// cassette → code 11, cf. internal/core cassette_error_test). RED si newFromConfig ne
// câble pas OnError ; GREEN avec le correctif.
func TestNewFromConfigWiresIOError(t *testing.T) {
	var got int
	orig := ioErrorReporter
	ioErrorReporter = func(code int) { got = code }
	t.Cleanup(func() { ioErrorReporter = orig })

	// Sans ROM : la machine se construit (état indéfini), ce qui suffit pour le chemin E/S.
	m, err := newFromConfig(machine.Config{})
	if err != nil {
		t.Fatalf("newFromConfig: %v", err)
	}
	ad, ok := m.(*adapter)
	if !ok {
		t.Fatalf("type inattendu : %T", m)
	}
	ad.Entreesortie(0x42) // READOCTETK7 sans cassette → doit déclencher OnError(11)
	if got != 11 {
		t.Fatalf("le profil ne câble pas OnError (erreur E/S non remontée) : got %d, want 11", got)
	}
}
