// Package launch porte la logique PURE de routage du démarrage (launcher vs boot
// direct) et le profil de démonstration de l'IHM (lot #117, PR-C2). Il ne dépend QUE
// d'internal/machine — surtout PAS d'Ebitengine/internal/app — afin de rester
// testable en CI : tout paquet important internal/app initialise GLFW au lancement du
// binaire de test et panique en environnement headless. cmd/dcmo5 (qui importe
// internal/app) doit donc rester sans test ; cette logique testable vit ici.
package launch

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Lesur-ai/dcmo5/internal/machine"
)

// DirectBoot décide si le démarrage contourne le launcher pour booter directement
// l'émulateur (comportement v1). La décision repose sur la présence EXPLICITE de
// flags utilisateur (--rom ou --exec), JAMAIS sur des valeurs issues du fallback
// config : ainsi « dcmo5 » sans argument ouvre toujours le launcher, même si une ROM
// est mémorisée en configuration (revue de plan Codex, P1).
func DirectBoot(romFlagSet, execFlagSet bool) bool {
	return romFlagSet || execFlagSet
}

// DirectBootSupported indique si un profil enregistré peut contourner le launcher
// dans le périmètre courant. Le MO5 conserve le chemin historique ; le TO9+ est
// activé par le lot #186. Le TO8D reste volontairement launcher-only tant que son
// chemin CLI direct n'a pas été validé explicitement.
func DirectBootSupported(machineID string) bool {
	switch machineID {
	case "mo5", "to9p":
		return true
	default:
		return false
	}
}

// SelectIndex résout l'index du profil dont l'ID == machineID dans profiles, pour
// PRÉSÉLECTIONNER la machine demandée via --machine au launcher (au lieu de l'ignorer).
// explicit indique si --machine a été fourni EXPLICITEMENT : un ID inconnu fourni
// explicitement est une ERREUR (parité avec la validation du boot direct) ; sinon (valeur
// par défaut, ex. "mo5") on retombe silencieusement sur le premier profil. Sépare ainsi
// « typo signalée » de « défaut tolérant ».
func SelectIndex(profiles []machine.MachineProfile, machineID string, explicit bool) (int, error) {
	for i, p := range profiles {
		if p.ID == machineID {
			return i, nil
		}
	}
	if explicit {
		ids := make([]string, 0, len(profiles))
		for _, p := range profiles {
			ids = append(ids, p.ID)
		}
		return 0, fmt.Errorf("machine inconnue %q. Disponibles : %s", machineID, strings.Join(ids, ", "))
	}
	return 0, nil
}

// DemoProfile est un profil de DÉMONSTRATION couvrant les 4 ParamKind (Enum, Bool,
// Int, File), destiné à valider VISUELLEMENT le rendu générique du launcher — le MO5
// réel ne déclare que des Params File. Il n'est JAMAIS enregistré via
// machine.Register : il n'apparaît donc ni dans machine.Profiles() ni via --machine,
// et ne fuit pas dans le périmètre des vraies machines (MO5/TO8D). Il est injecté
// dans la liste du launcher UNIQUEMENT si la variable d'environnement DCMO5_UI_DEMO
// est définie. Son New renvoie une erreur : « Démarrer » sert alors de test visuel du
// chemin d'erreur (le launcher reste affiché, pas de crash).
func DemoProfile() machine.MachineProfile {
	return machine.MachineProfile{
		ID:     "demo",
		Name:   "Démo (rendu)",
		Family: machine.FamilyMO,
		Params: []machine.Param{
			{Key: "ram", Label: "Mémoire", Kind: machine.ParamEnum, Default: 512,
				Options: []machine.Option{{Value: 256, Label: "256 Ko"}, {Value: 512, Label: "512 Ko"}}},
			{Key: "turbo", Label: "Turbo", Kind: machine.ParamBool, Default: false},
			{Key: "vitesse", Label: "Vitesse", Kind: machine.ParamInt, Default: 1},
			{Key: machine.KeyROM, Label: "ROM", Kind: machine.ParamFile, FileExt: []string{".rom"}, Required: true},
		},
		New: func(machine.Config) (machine.Machine, error) {
			return nil, errors.New("profil de démonstration : non instanciable")
		},
	}
}
