package uimodel_test

import (
	"errors"
	"testing"

	"github.com/Lesur-ai/dcmo5/internal/machine"
	"github.com/Lesur-ai/dcmo5/internal/uimodel"
)

// fakeProfile : profil LOCAL (jamais enregistré via machine.Register, donc invisible
// de machine.Profiles()) couvrant les 4 Kinds + Required + LiveMutable + Validate +
// le chemin nil-Validate (cas réel : MO5 ne déclare aucun Validate).
func fakeProfile() machine.MachineProfile {
	return machine.MachineProfile{
		ID: "fake", Name: "Fake", Family: machine.FamilyMO,
		Params: []machine.Param{
			{Key: "ram", Label: "RAM", Kind: machine.ParamEnum, Default: 512, // Enum, boot-only, Validate nil
				Options: []machine.Option{{Value: 256, Label: "256K"}, {Value: 512, Label: "512K"}}},
			{Key: "turbo", Label: "Turbo", Kind: machine.ParamBool, Default: false, LiveMutable: true}, // Bool, live, Validate nil
			{Key: "speed", Label: "Vitesse", Kind: machine.ParamInt, Default: 1, // Int + Validate
				Validate: func(v any) error {
					if i, ok := v.(int); !ok || i < 1 {
						return errors.New("vitesse doit être >= 1")
					}
					return nil
				}},
			{Key: "rom", Label: "ROM", Kind: machine.ParamFile, FileExt: []string{".rom"}, Required: true},         // File, Required, boot-only
			{Key: "tape", Label: "Cassette", Kind: machine.ParamFile, FileExt: []string{".k7"}, LiveMutable: true}, // File, live
		},
		New: func(cfg machine.Config) (machine.Machine, error) { return nil, nil }, // non appelé par uimodel
	}
}

func TestDescribe(t *testing.T) {
	p := fakeProfile()
	cfg := machine.Config{"turbo": true} // override ; les autres prennent leur Default
	got := uimodel.Describe(p, cfg)
	if len(got) != len(p.Params) {
		t.Fatalf("Describe = %d descripteurs, want %d", len(got), len(p.Params))
	}
	if got[0].Key != "ram" || got[0].Kind != machine.ParamEnum || got[0].Value != 512 || len(got[0].Options) != 2 {
		t.Errorf("descr[0] ram = %+v", got[0])
	}
	if got[1].Key != "turbo" || got[1].Kind != machine.ParamBool || got[1].Value != true || !got[1].LiveMutable {
		t.Errorf("descr[1] turbo (override + livemutable) = %+v", got[1])
	}
	if got[2].Value != 1 {
		t.Errorf("descr[2] speed value = %v, want default 1", got[2].Value)
	}
	if got[3].Key != "rom" || got[3].Kind != machine.ParamFile || !got[3].Required ||
		len(got[3].FileExt) != 1 || got[3].FileExt[0] != ".rom" {
		t.Errorf("descr[3] rom (required + fileext) = %+v", got[3])
	}
}

func TestDescribeLive_OnlyLiveMutable(t *testing.T) {
	p := fakeProfile()
	// fakeProfile : LiveMutable = turbo (Bool) + tape (File) ; boot-only = ram, speed, rom.
	got := uimodel.DescribeLive(p, machine.Config{})
	if len(got) != 2 {
		t.Fatalf("DescribeLive = %d descripteurs, want 2 (turbo, tape) : %+v", len(got), got)
	}
	if got[0].Key != "turbo" || got[1].Key != "tape" { // ordre = ordre des Params du profil
		t.Errorf("ordre/clés DescribeLive = [%s, %s], want [turbo, tape]", got[0].Key, got[1].Key)
	}
	for _, d := range got {
		if !d.LiveMutable {
			t.Errorf("DescribeLive a laissé passer un Param boot-only : %+v", d)
		}
		if d.Key == "rom" || d.Key == "ram" || d.Key == "speed" {
			t.Errorf("DescribeLive ne doit JAMAIS inclure le boot-only %q", d.Key)
		}
	}
}

func TestValidate_RequiredMissing(t *testing.T) {
	p := fakeProfile()
	errs := uimodel.Validate(p, machine.Config{"speed": 1}) // rom (Required) absent
	if errs["rom"] == nil {
		t.Error("rom requis absent doit produire une erreur")
	}
	if errs["ram"] != nil || errs["turbo"] != nil {
		t.Error("ram/turbo (non requis, avec Default) ne doivent pas être en erreur")
	}
}

func TestValidate_NilValidateNoPanicAndAccepts(t *testing.T) {
	p := fakeProfile()
	// ram/turbo/rom/tape ont Validate nil → ne doivent NI paniquer NI rejeter.
	errs := uimodel.Validate(p, machine.Config{"rom": "boot.rom", "speed": 1})
	if len(errs) != 0 {
		t.Errorf("config valide → 0 erreur, got %v", errs)
	}
}

func TestValidate_ValidatePropagates(t *testing.T) {
	p := fakeProfile()
	errs := uimodel.Validate(p, machine.Config{"rom": "boot.rom", "speed": 0}) // speed invalide
	if errs["speed"] == nil {
		t.Error("speed=0 doit échouer via Param.Validate")
	}
}

func TestBuildConfig_FillsDefaultsAndValidates(t *testing.T) {
	p := fakeProfile()
	in := machine.Config{"rom": "boot.rom"}
	cfg, err := uimodel.BuildConfig(p, in)
	if err != nil {
		t.Fatalf("BuildConfig (valide) → err = %v", err)
	}
	if cfg["ram"] != 512 || cfg["turbo"] != false || cfg["speed"] != 1 || cfg["rom"] != "boot.rom" {
		t.Errorf("BuildConfig défauts/valeurs = %+v", cfg)
	}
	if len(in) != 1 { // l'entrée ne doit pas être mutée (défauts ajoutés à la copie seulement)
		t.Errorf("BuildConfig a muté la config d'entrée : %+v", in)
	}
}

func TestFakeProfileNeverRegistered(t *testing.T) {
	// Invariant : uimodel prend le profil EN PARAMÈTRE ; le profil factice ne doit
	// JAMAIS apparaître dans le registre global (sinon il polluerait le launcher).
	_ = fakeProfile()
	for _, p := range machine.Profiles() {
		if p.ID == "fake" {
			t.Fatal("le profil factice ne doit pas être enregistré dans machine.Profiles()")
		}
	}
}

func TestBuildConfig_InvalidReturnsError(t *testing.T) {
	p := fakeProfile()
	if _, err := uimodel.BuildConfig(p, machine.Config{"rom": "boot.rom", "speed": 0}); err == nil {
		t.Error("BuildConfig avec speed=0 doit retourner une erreur")
	}
	if _, err := uimodel.BuildConfig(p, machine.Config{}); err == nil {
		t.Error("BuildConfig sans rom (Required) doit retourner une erreur")
	}
}

func TestDiffLive_OnlyLiveMutable(t *testing.T) {
	p := fakeProfile()
	old := machine.Config{"ram": 512, "turbo": false, "tape": ""}
	next := machine.Config{"ram": 256, "turbo": true, "tape": "game.k7"} // ram=boot-only ; turbo+tape=live
	got := uimodel.DiffLive(p, old, next)
	vals := map[string]any{}
	for _, c := range got {
		vals[c.Key] = c.Value
	}
	if _, ok := vals["ram"]; ok {
		t.Error("ram (boot-only) ne doit JAMAIS apparaître dans un diff live")
	}
	if vals["turbo"] != true || vals["tape"] != "game.k7" {
		t.Errorf("turbo+tape (LiveMutable changés) attendus, got %+v", got)
	}
	if len(got) != 2 {
		t.Errorf("2 changements live attendus, got %d (%+v)", len(got), got)
	}
	if got[0].Key != "turbo" || got[1].Key != "tape" { // ordre = ordre des Params du profil
		t.Errorf("ordre des changements live = [%s, %s], want [turbo, tape]", got[0].Key, got[1].Key)
	}
}

func TestDiffLive_NoChange(t *testing.T) {
	p := fakeProfile()
	c := machine.Config{"ram": 512, "turbo": true, "tape": "x.k7"}
	if got := uimodel.DiffLive(p, c, c); len(got) != 0 {
		t.Errorf("config identique → 0 changement, got %+v", got)
	}
}

func TestListDir(t *testing.T) {
	lister := func(dir string) ([]uimodel.Entry, error) {
		return []uimodel.Entry{
			{Name: "zeb.rom", IsDir: false},
			{Name: "abc.rom", IsDir: false},
			{Name: "notes.txt", IsDir: false}, // mauvaise extension → filtré
			{Name: ".hidden", IsDir: true},    // caché → masqué
			{Name: "sub", IsDir: true},
			{Name: "AAA.ROM", IsDir: false}, // extension insensible à la casse
		}, nil
	}
	got := uimodel.ListDir(lister, "/x", []string{".rom"})
	// attendu : "..", dossiers triés ("sub"), puis fichiers .rom triés (octet : AAA.ROM, abc.rom, zeb.rom)
	want := []uimodel.Entry{
		{Name: "..", IsDir: true},
		{Name: "sub", IsDir: true},
		{Name: "AAA.ROM", IsDir: false},
		{Name: "abc.rom", IsDir: false},
		{Name: "zeb.rom", IsDir: false},
	}
	if len(got) != len(want) {
		t.Fatalf("ListDir = %d entrées, want %d : %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("entrée[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}
