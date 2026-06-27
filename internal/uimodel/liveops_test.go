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
