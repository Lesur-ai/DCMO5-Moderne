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
// Règle : un Param File média (tape/disk/cart) devient OpMount(Path) si un chemin est
// fourni, sinon OpEject. Toute AUTRE clé LiveMutable devient OpUnsupported — garde-fou
// explicite contre un futur Param Bool/Enum/Int marqué LiveMutable qui serait affiché
// dans l'overlay mais n'aurait aucun effet réel : l'appelant doit le signaler, pas
// l'appliquer silencieusement.
//
// La fonction ne décide PAS de l'ordre Mount/Eject vs reste : elle reflète DiffLive.
func LiveMediaOps(p machine.MachineProfile, old, next machine.Config) []MediaOp {
	var ops []MediaOp
	for _, ch := range DiffLive(p, old, next) {
		switch ch.Key {
		case machine.KeyTape, machine.KeyDisk, machine.KeyCart:
			path, _ := ch.Value.(string)
			if path == "" {
				ops = append(ops, MediaOp{Kind: OpEject, Key: ch.Key})
			} else {
				ops = append(ops, MediaOp{Kind: OpMount, Key: ch.Key, Path: path})
			}
		default:
			ops = append(ops, MediaOp{Kind: OpUnsupported, Key: ch.Key})
		}
	}
	return ops
}
