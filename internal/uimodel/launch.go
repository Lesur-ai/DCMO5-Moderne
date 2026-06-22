package uimodel

import "github.com/Lesur-ai/dcmo5/internal/machine"

// launch.go — coutures PURES du démarrage depuis le launcher (lot #117, PR-C2).
// Aucune dépendance Ebitengine/ebitenui : testable en CI, contrairement au rendu
// (internal/app/launcher.go). Elles cadrent deux décisions que le rendu se contente
// d'appliquer : QUELS médias monter après profile.New, et QUELLES valeurs (re)poser
// quand on (re)sélectionne un profil.

// MediaMount décrit un média à monter À CHAUD après la création de la machine
// (profile.New ne monte pas les médias — cf. mo5.newFromConfig). Key est la clé du
// Param (ex. "tape"), Path le chemin choisi par l'utilisateur.
type MediaMount struct {
	Key  string
	Path string
}

// MediaMounts retourne, dans l'ordre des Params, les médias à monter à chaud après
// profile.New : les Params File ET LiveMutable dont la valeur courante est un chemin
// non vide. Les Params File boot-only (ex. "rom", "disk-rom") sont consommés par
// profile.New lui-même et n'apparaissent donc PAS ici. La couche app traduit ensuite
// chaque Key en appel de montage typé (MountTape/MountDisk/MountCartridge).
func MediaMounts(p machine.MachineProfile, cfg machine.Config) []MediaMount {
	var out []MediaMount
	for _, param := range p.Params {
		if param.Kind != machine.ParamFile || !param.LiveMutable {
			continue // rom/disk-rom (boot-only) consommés par New ; non-fichiers ignorés
		}
		if path, ok := resolveValue(param, cfg).(string); ok && path != "" {
			out = append(out, MediaMount{Key: param.Key, Path: path})
		}
	}
	return out
}

// InitialValues retourne les valeurs de départ d'un profil : le Default de chaque
// Param qui en déclare un (non nil). Utilisé quand l'utilisateur (re)sélectionne un
// profil dans le launcher : on REPART de ces valeurs, ce qui garantit qu'aucune
// saisie d'un profil précédent (ex. "rom"/"tape") ne fuit vers le profil suivant
// dont le schéma de clés diffère. L'appelant peut ensuite surcharger des clés
// connues (ex. chemin ROM mémorisé en config).
func InitialValues(p machine.MachineProfile) machine.Config {
	out := machine.Config{}
	for _, param := range p.Params {
		if param.Default != nil {
			out[param.Key] = param.Default
		}
	}
	return out
}
