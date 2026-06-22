package main

import (
	"errors"

	"github.com/Lesur-ai/dcmo5/internal/machine"
)

// demoProfile est un profil de DÉMONSTRATION couvrant les 4 ParamKind (Enum, Bool,
// Int, File), destiné à valider VISUELLEMENT le rendu générique du launcher — le MO5
// réel ne déclare que des Params File. Il n'est JAMAIS enregistré via
// machine.Register : il n'apparaît donc ni dans machine.Profiles() ni via --machine,
// et ne fuit pas dans le périmètre des vraies machines (MO5/TO8D). Il est injecté
// dans la liste du launcher UNIQUEMENT si la variable d'environnement DCMO5_UI_DEMO
// est définie. Son New renvoie une erreur : « Démarrer » sert alors de test visuel
// du chemin d'erreur (le launcher reste affiché, pas de crash).
func demoProfile() machine.MachineProfile {
	return machine.MachineProfile{
		ID:     "demo",
		Name:   "Démo (rendu)",
		Family: machine.FamilyMO,
		Params: []machine.Param{
			{Key: "ram", Label: "Mémoire", Kind: machine.ParamEnum, Default: 512,
				Options: []machine.Option{{Value: 256, Label: "256 Ko"}, {Value: 512, Label: "512 Ko"}}},
			{Key: "turbo", Label: "Turbo", Kind: machine.ParamBool, Default: false},
			{Key: "vitesse", Label: "Vitesse", Kind: machine.ParamInt, Default: 1},
			{Key: "rom", Label: "ROM", Kind: machine.ParamFile, FileExt: []string{".rom"}, Required: true},
		},
		New: func(machine.Config) (machine.Machine, error) {
			return nil, errors.New("profil de démonstration : non instanciable")
		},
	}
}

// directBoot décide si le démarrage contourne le launcher pour booter directement
// l'émulateur (comportement v1). La décision repose sur la présence EXPLICITE de
// flags utilisateur (--rom ou --exec), JAMAIS sur des valeurs issues du fallback
// config : ainsi « dcmo5 » sans argument ouvre toujours le launcher, même si une ROM
// est mémorisée en configuration (revue de plan Codex, P1).
func directBoot(romFlagSet, execFlagSet bool) bool {
	return romFlagSet || execFlagSet
}
