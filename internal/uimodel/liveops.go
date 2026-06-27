package uimodel

import "github.com/Lesur-ai/dcmo5/internal/machine"

// MediaOpKind classe l'effet d'un changement applicable à chaud sur un média.
type MediaOpKind int

const (
	OpMount       MediaOpKind = iota // monter le média Key depuis Path
	OpEject                          // éjecter le média Key (Path vide)
	OpUnsupported                    // clé LiveMutable sans traduction média : à SIGNALER, jamais appliquer en silence
)

// MediaOp est l'opération que l'hôte doit exécuter pour refléter un changement live
// décidé dans l'overlay. Couche PURE : aucune E/S, aucun import Ebitengine — c'est
// l'appelant (internal/app) qui exécute MountTape/EjectDisk/… selon Kind et Key.
type MediaOp struct {
	Kind MediaOpKind
	Key  string // tape/disk/cart (OpMount/OpEject) ou la clé brute (OpUnsupported)
	Path string // chemin à monter (OpMount uniquement) ; vide sinon
}

// LiveMediaOps traduit les changements applicables à chaud (DiffLive) en opérations
// média typées, dans l'ordre des Params.
//
// Règle pour un Param File média (tape/disk/cart) — la valeur DOIT être une string :
//   - chaîne vide "" → OpEject (sentinel d'éjection, cohérent avec le reste du code) ;
//   - chaîne non vide → OpMount(Path).
//
// Toute valeur média NON-string (nil, bool, int…) est une anomalie : OpUnsupported, à
// SIGNALER. On n'éjecte JAMAIS en silence sur un type inattendu — sans ce garde-fou,
// un `ch.Value.(string)` raté donnerait "" donc une éjection silencieuse, exactement
// le bug que cette projection prétend interdire.
//
// Toute AUTRE clé LiveMutable (futur Param Bool/Enum/Int marqué live) devient également
// OpUnsupported : affichée dans l'overlay mais sans traduction média, elle doit être
// signalée, pas appliquée silencieusement sans effet réel.
//
// La fonction ne décide PAS de l'ordre Mount/Eject vs reste : elle reflète DiffLive.
func LiveMediaOps(p machine.MachineProfile, old, next machine.Config) []MediaOp {
	var ops []MediaOp
	for _, ch := range DiffLive(p, old, next) {
		switch ch.Key {
		case machine.KeyTape, machine.KeyDisk, machine.KeyCart:
			path, ok := ch.Value.(string)
			switch {
			case !ok:
				// valeur média non-string : anomalie, jamais d'éjection silencieuse.
				ops = append(ops, MediaOp{Kind: OpUnsupported, Key: ch.Key})
			case path == "":
				ops = append(ops, MediaOp{Kind: OpEject, Key: ch.Key})
			default:
				ops = append(ops, MediaOp{Kind: OpMount, Key: ch.Key, Path: path})
			}
		default:
			ops = append(ops, MediaOp{Kind: OpUnsupported, Key: ch.Key})
		}
	}
	return ops
}

// LiveMediaConfig projette l'état des médias RÉELLEMENT montés en une machine.Config
// restreinte aux Params médias modifiables à chaud du profil. C'est l'opération inverse
// de LiveMediaOps : LiveMediaOps dit ce qui CHANGE, LiveMediaConfig fournit l'état COURANT
// — la base `old` que l'overlay passe à DescribeLive/DiffLive.
//
// Pour chaque Param à la fois ParamFile ET LiveMutable, la clé est incluse UNIQUEMENT si
// elle figure dans `mounted` AVEC un nom NON VIDE (média effectivement ouvert/monté). Un
// média non monté — clé absente ou valeur vide — n'apparaît pas → aucun média fantôme
// affiché (un média réellement monté a toujours un nom non vide).
//
// Le filtrage est piloté par le SCHÉMA du profil (data-driven, symétrique MO5/TO8D), ce
// qui en fait un garde-fou de concordance : une clé de `mounted` que le profil ne déclare
// pas comme média live (boot-only comme rom, ou non-File, ou clé inconnue) est ignorée —
// jamais projetée comme un média éditable. La couche impure (internal/app) construit
// `mounted` depuis sa source de vérité vivante (closers/noms) ; elle ne stocke donc PAS de
// config média parallèle qui pourrait diverger.
func LiveMediaConfig(p machine.MachineProfile, mounted map[string]string) machine.Config {
	cfg := machine.Config{}
	for _, param := range p.Params {
		if param.Kind != machine.ParamFile || !param.LiveMutable {
			continue // boot-only (rom) ou non-File : hors champ de l'overlay live
		}
		if v, ok := mounted[param.Key]; ok && v != "" {
			cfg[param.Key] = v
		}
	}
	return cfg
}
