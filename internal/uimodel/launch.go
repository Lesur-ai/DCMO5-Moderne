package uimodel

import (
	"path/filepath"

	"github.com/Lesur-ai/dcmo5/internal/machine"
)

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

// ResolveDiskROM auto-détecte la ROM contrôleur de disquette « cd90-640.rom » à côté de
// la ROM système, MIROIR du boot CLI (cmd/dcmo5) : sans elle, un disque .fd choisi au
// launcher démarrerait sans contrôleur → DOS inopérant, alors que « dcmo5 --rom … --disk … »
// fonctionne avec les mêmes fichiers. Elle ne fait rien si une disk-rom est déjà fournie
// explicitement (pas d'écrasement) ou si aucune ROM n'est choisie. exists découple du
// disque pour la testabilité (os.Stat en production). Retourne le chemin à injecter dans
// la config (clé machine.KeyDiskROM) avant profile.New, ou "" s'il n'y a rien à faire.
func ResolveDiskROM(cfg machine.Config, exists func(string) bool) string {
	if dr, _ := cfg[machine.KeyDiskROM].(string); dr != "" {
		return "" // disk-rom explicite : ne pas écraser le choix de l'utilisateur
	}
	rom, _ := cfg[machine.KeyROM].(string)
	if rom == "" {
		return "" // pas de ROM système → rien à quoi adosser l'auto-détection
	}
	candidate := filepath.Join(filepath.Dir(rom), "cd90-640.rom")
	if exists(candidate) {
		return candidate
	}
	return ""
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
