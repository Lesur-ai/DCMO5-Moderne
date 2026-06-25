package overlay

import (
	"github.com/Lesur-ai/dcmo5/internal/machine"
	"github.com/Lesur-ai/dcmo5/internal/uimodel"
)

// switch.go — préparation PURE d'un changement de machine à chaud (overlay #117,
// Inc 2). Aucune dépendance Ebitengine ni Host : testable en CI. La part IMPURE
// (Stop du Host, teardown audio/médias, attachMachine, SetWindowSize, Start) vit
// dans internal/app (Inc 5) et CONSOMME le résultat de PrepareSwitch.

// Prep est le résultat PUR de la préparation d'un changement de machine : la Config
// validée à passer à MachineProfile.New, et les médias à monter à chaud après New.
type Prep struct {
	Config machine.Config
	Mounts []uimodel.MediaMount
}

// PrepareSwitch prépare le passage à newProfile SANS rien détruire ni instancier.
//
// Doctrine state-safety (revue de plan Codex, B2) : la validation a lieu ICI, AVANT
// que la couche app n'arrête l'ancienne machine. Une erreur (ex. ROM requise
// manquante) renvoie une erreur et la session courante reste INTACTE — on ne stoppe
// l'ancien Host qu'une fois la nouvelle config prouvée valide.
//
// Étapes :
//   - repart de uimodel.InitialValues(newProfile) (anti-fuite : aucune clé d'un
//     profil précédent ne sert de base), surchargée par persisted (config mémorisée
//     du profil cible : ROM par machine, etc.) ;
//   - auto-détecte la ROM contrôleur de disquette (ResolveDiskROM, miroir du boot CLI)
//     via exists ;
//   - valide et complète la Config (BuildConfig) ; erreur propagée sans effet de bord ;
//   - liste les médias LiveMutable à monter à chaud après New (MediaMounts).
//
// La GÉOMÉTRIE (taille de fenêtre) n'est volontairement PAS calculée ici : elle n'est
// connue qu'au runtime via Machine.FrameSize() après New. La couche app redimensionne
// la fenêtre depuis cette taille runtime (SetWindowSize, idempotent si inchangée) —
// d'où l'absence de tout « FrameSizeChanged » dans ce plan pur (revue de plan, B5).
//
// exists découple du disque pour la testabilité (os.Stat en production).
func PrepareSwitch(newProfile machine.MachineProfile, persisted machine.Config, exists func(string) bool) (Prep, error) {
	values := uimodel.InitialValues(newProfile)
	for k, v := range persisted {
		values[k] = v
	}
	if dr := uimodel.ResolveDiskROM(values, exists); dr != "" {
		values[machine.KeyDiskROM] = dr
	}
	cfg, err := uimodel.BuildConfig(newProfile, values)
	if err != nil {
		return Prep{}, err
	}
	return Prep{Config: cfg, Mounts: uimodel.MediaMounts(newProfile, cfg)}, nil
}
