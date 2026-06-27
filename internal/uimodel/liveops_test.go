package uimodel_test

import (
	"testing"

	"github.com/Lesur-ai/dcmo5/internal/machine"
	"github.com/Lesur-ai/dcmo5/internal/uimodel"
)

// profilMedia : profil de test avec les trois Params File média LiveMutable plus une
// ROM boot-only (pour prouver qu'un changement boot-only ne produit AUCUNE op).
func profilMedia() machine.MachineProfile {
	return machine.MachineProfile{
		ID: "test",
		Params: []machine.Param{
			{Key: machine.KeyROM, Kind: machine.ParamFile, Required: true},     // boot-only
			{Key: machine.KeyTape, Kind: machine.ParamFile, LiveMutable: true}, // média
			{Key: machine.KeyDisk, Kind: machine.ParamFile, LiveMutable: true}, // média
			{Key: machine.KeyCart, Kind: machine.ParamFile, LiveMutable: true}, // média
		},
	}
}

func TestLiveMediaOps_NewFileMounts(t *testing.T) {
	p := profilMedia()
	old := machine.Config{}
	next := machine.Config{machine.KeyTape: "/jeux/aigle.k7"}

	ops := uimodel.LiveMediaOps(p, old, next)
	if len(ops) != 1 {
		t.Fatalf("ops = %d, want 1 : %+v", len(ops), ops)
	}
	if ops[0] != (uimodel.MediaOp{Kind: uimodel.OpMount, Key: machine.KeyTape, Path: "/jeux/aigle.k7"}) {
		t.Errorf("op = %+v, want Mount tape /jeux/aigle.k7", ops[0])
	}
}

func TestLiveMediaOps_EmptiedFileEjects(t *testing.T) {
	p := profilMedia()
	old := machine.Config{machine.KeyDisk: "/d.fd"}
	next := machine.Config{machine.KeyDisk: ""} // chemin vidé → éjection

	ops := uimodel.LiveMediaOps(p, old, next)
	if len(ops) != 1 {
		t.Fatalf("ops = %d, want 1 : %+v", len(ops), ops)
	}
	if ops[0] != (uimodel.MediaOp{Kind: uimodel.OpEject, Key: machine.KeyDisk}) {
		t.Errorf("op = %+v, want Eject disk", ops[0])
	}
}

func TestLiveMediaOps_NoChangeNoOps(t *testing.T) {
	p := profilMedia()
	cfg := machine.Config{machine.KeyTape: "/x.k7", machine.KeyDisk: "/y.fd"}
	if ops := uimodel.LiveMediaOps(p, cfg, cfg); len(ops) != 0 {
		t.Errorf("config identique : ops = %+v, want aucune", ops)
	}
}

// TestLiveMediaOps_BootOnlyIgnored : un changement d'un Param boot-only (ROM) ne doit
// produire AUCUNE op (DiffLive l'exclut déjà — verrou contre un remontage à chaud).
func TestLiveMediaOps_BootOnlyIgnored(t *testing.T) {
	p := profilMedia()
	old := machine.Config{machine.KeyROM: "/a.rom"}
	next := machine.Config{machine.KeyROM: "/b.rom"}
	if ops := uimodel.LiveMediaOps(p, old, next); len(ops) != 0 {
		t.Errorf("changement ROM boot-only : ops = %+v, want aucune", ops)
	}
}

// TestLiveMediaOps_UnknownLiveKeyUnsupported : une clé LiveMutable NON média (futur
// Bool/Enum/Int) ne doit PAS être appliquée silencieusement → OpUnsupported, à signaler.
func TestLiveMediaOps_UnknownLiveKeyUnsupported(t *testing.T) {
	p := machine.MachineProfile{
		ID: "test",
		Params: []machine.Param{
			{Key: "turbo", Kind: machine.ParamBool, LiveMutable: true},
		},
	}
	old := machine.Config{"turbo": false}
	next := machine.Config{"turbo": true}

	ops := uimodel.LiveMediaOps(p, old, next)
	if len(ops) != 1 {
		t.Fatalf("ops = %d, want 1 : %+v", len(ops), ops)
	}
	if ops[0].Kind != uimodel.OpUnsupported || ops[0].Key != "turbo" {
		t.Errorf("op = %+v, want Unsupported turbo (jamais appliqué en silence)", ops[0])
	}
}

// TestLiveMediaOps_NonStringMediaValueUnsupported : une clé média (tape/disk/cart) dont
// la valeur n'est PAS une string (bool, int, nil…) est une anomalie. Elle ne doit JAMAIS
// produire d'éjection silencieuse (le bug qu'un `Value.(string)` raté provoquerait : ""
// → OpEject) ni de montage : seulement OpUnsupported, à signaler.
func TestLiveMediaOps_NonStringMediaValueUnsupported(t *testing.T) {
	p := profilMedia()
	cases := map[string]any{
		"bool":   true,
		"int":    123,
		"nilVal": nil, // clé présente mais valeur nil : anomalie, pas une éjection
	}
	for name, bad := range cases {
		t.Run(name, func(t *testing.T) {
			old := machine.Config{machine.KeyTape: "/jeux/aigle.k7"} // un média était monté…
			next := machine.Config{machine.KeyTape: bad}             // …remplacé par une valeur invalide

			ops := uimodel.LiveMediaOps(p, old, next)
			if len(ops) != 1 {
				t.Fatalf("ops = %d, want 1 : %+v", len(ops), ops)
			}
			if ops[0].Kind != uimodel.OpUnsupported || ops[0].Key != machine.KeyTape {
				t.Errorf("op = %+v, want Unsupported tape (ni Mount ni Eject silencieux)", ops[0])
			}
		})
	}
}

// TestLiveMediaOps_OrderFollowsParams : plusieurs médias changés → ops dans l'ordre des
// Params du profil (déterminisme, hérité de DiffLive).
func TestLiveMediaOps_OrderFollowsParams(t *testing.T) {
	p := profilMedia()
	old := machine.Config{}
	next := machine.Config{machine.KeyCart: "/c.rom", machine.KeyTape: "/t.k7"}

	ops := uimodel.LiveMediaOps(p, old, next)
	if len(ops) != 2 {
		t.Fatalf("ops = %d, want 2 : %+v", len(ops), ops)
	}
	// Ordre des Params : tape (avant) puis cart.
	if ops[0].Key != machine.KeyTape || ops[1].Key != machine.KeyCart {
		t.Errorf("ordre = [%s, %s], want [tape, cart]", ops[0].Key, ops[1].Key)
	}
}

// --- LiveMediaConfig : projection état monté observé → config média live ---

// TestLiveMediaConfig_OnlyMountedMedia : seuls les médias présents dans `mounted` (donc
// réellement montés) apparaissent, avec leur valeur. Un média non monté n'est PAS projeté.
func TestLiveMediaConfig_OnlyMountedMedia(t *testing.T) {
	p := profilMedia()
	mounted := map[string]string{machine.KeyTape: "aigle.k7"} // disk/cart non montés

	cfg := uimodel.LiveMediaConfig(p, mounted)
	if len(cfg) != 1 {
		t.Fatalf("cfg = %d clés, want 1 : %+v", len(cfg), cfg)
	}
	if cfg[machine.KeyTape] != "aigle.k7" {
		t.Errorf("cfg[tape] = %v, want \"aigle.k7\"", cfg[machine.KeyTape])
	}
}

// TestLiveMediaConfig_NothingMounted : aucun média monté → config vide (pas de média fantôme).
func TestLiveMediaConfig_NothingMounted(t *testing.T) {
	if cfg := uimodel.LiveMediaConfig(profilMedia(), map[string]string{}); len(cfg) != 0 {
		t.Errorf("rien monté : cfg = %+v, want vide", cfg)
	}
}

// TestLiveMediaConfig_EmptyNameAbsent : une clé présente mais de NOM VIDE n'est pas un média
// réellement monté → non projetée. Garde-fou contre un témoin de montage dégénéré côté
// appelant (un média monté a toujours un nom non vide).
func TestLiveMediaConfig_EmptyNameAbsent(t *testing.T) {
	p := profilMedia()
	mounted := map[string]string{machine.KeyTape: "", machine.KeyDisk: "d.fd"}

	cfg := uimodel.LiveMediaConfig(p, mounted)
	if _, present := cfg[machine.KeyTape]; present {
		t.Errorf("clé média à nom vide projetée à tort : %+v", cfg)
	}
	if len(cfg) != 1 || cfg[machine.KeyDisk] != "d.fd" {
		t.Errorf("cfg = %+v, want {disk:d.fd} seul", cfg)
	}
}

// TestLiveMediaConfig_BootOnlyExcluded : la ROM (ParamFile mais boot-only, non LiveMutable)
// ne doit JAMAIS apparaître, même si elle figure dans `mounted` — l'overlay ne projette
// que le live, pas un réglage boot-only éditable à tort.
func TestLiveMediaConfig_BootOnlyExcluded(t *testing.T) {
	p := profilMedia()
	mounted := map[string]string{machine.KeyROM: "mo5.rom", machine.KeyTape: "x.k7"}

	cfg := uimodel.LiveMediaConfig(p, mounted)
	if _, present := cfg[machine.KeyROM]; present {
		t.Errorf("rom boot-only projetée à tort : %+v", cfg)
	}
	if len(cfg) != 1 || cfg[machine.KeyTape] != "x.k7" {
		t.Errorf("cfg = %+v, want {tape:x.k7} seul", cfg)
	}
}

// TestLiveMediaConfig_LiveNonFileExcluded : un Param LiveMutable NON-File (ex. turbo bool)
// présent dans `mounted` ne doit pas être projeté — LiveMediaConfig ne porte que des médias
// fichier, pas des réglages live arbitraires.
func TestLiveMediaConfig_LiveNonFileExcluded(t *testing.T) {
	p := machine.MachineProfile{
		ID: "test",
		Params: []machine.Param{
			{Key: "turbo", Kind: machine.ParamBool, LiveMutable: true}, // live mais pas un fichier
			{Key: machine.KeyTape, Kind: machine.ParamFile, LiveMutable: true},
		},
	}
	mounted := map[string]string{"turbo": "true", machine.KeyTape: "x.k7"}

	cfg := uimodel.LiveMediaConfig(p, mounted)
	if _, present := cfg["turbo"]; present {
		t.Errorf("réglage live non-File projeté à tort : %+v", cfg)
	}
	if len(cfg) != 1 || cfg[machine.KeyTape] != "x.k7" {
		t.Errorf("cfg = %+v, want {tape:x.k7} seul", cfg)
	}
}

// TestLiveMediaConfig_UnknownKeyIgnored : une clé de `mounted` que le profil ne déclare
// PAS est ignorée (garde-fou de concordance profil/état observé).
func TestLiveMediaConfig_UnknownKeyIgnored(t *testing.T) {
	p := profilMedia()
	mounted := map[string]string{"floppy5": "z.img", machine.KeyDisk: "d.fd"}

	cfg := uimodel.LiveMediaConfig(p, mounted)
	if _, present := cfg["floppy5"]; present {
		t.Errorf("clé hors profil projetée : %+v", cfg)
	}
	if len(cfg) != 1 || cfg[machine.KeyDisk] != "d.fd" {
		t.Errorf("cfg = %+v, want {disk:d.fd} seul", cfg)
	}
}

// TestLiveMediaConfig_ZeroProfile : profil zéro-value (Params nil, ex. ByID échoué) → config
// VIDE quelle que soit `mounted`. Sans schéma, rien ne peut être projeté comme média live :
// c'est le comportement sûr (pas de média demandé reclassé monté faute de profil).
func TestLiveMediaConfig_ZeroProfile(t *testing.T) {
	mounted := map[string]string{machine.KeyTape: "x.k7", machine.KeyDisk: "d.fd"}
	if cfg := uimodel.LiveMediaConfig(machine.MachineProfile{}, mounted); len(cfg) != 0 {
		t.Errorf("profil zéro : cfg = %+v, want vide", cfg)
	}
}
