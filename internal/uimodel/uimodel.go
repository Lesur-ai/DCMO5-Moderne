// Package uimodel est la couche PURE de l'IHM v2 (lot #117) : il transforme le
// schéma déclaratif d'une machine (machine.MachineProfile / Param) en descripteurs
// neutres rendus ensuite par ebitenui (launcher et overlay), valide les saisies,
// construit la Config passée à MachineProfile.New, et calcule les changements
// applicables à chaud. Il porte aussi un navigateur de fichiers pur.
//
// CONTRAINTE : aucun import Ebitengine/ebitenui ici. C'est le cœur testable EN
// HEADLESS du lot ; le rendu graphique (non testable en CI) vit dans internal/app.
//
// Toutes les fonctions prennent le machine.MachineProfile EN PARAMÈTRE — jamais via
// machine.Profiles()/Register — pour rester déterministes et ne pas dépendre du
// registre global (init()).
package uimodel

import (
	"fmt"
	"reflect"

	"github.com/Lesur-ai/dcmo5/internal/machine"
)

// WidgetDescriptor est la représentation neutre d'un Param et de sa valeur courante,
// destinée au rendu ebitenui. Aucune dépendance UI.
type WidgetDescriptor struct {
	Key         string
	Label       string
	Kind        machine.ParamKind
	Value       any // valeur courante : cfg[Key] si présent, sinon Param.Default
	Options     []machine.Option
	FileExt     []string
	Required    bool
	LiveMutable bool
}

// Describe construit, dans l'ordre des Params du profil, un descripteur par Param,
// en résolvant la valeur courante depuis cfg (sinon Default).
func Describe(p machine.MachineProfile, cfg machine.Config) []WidgetDescriptor {
	out := make([]WidgetDescriptor, 0, len(p.Params))
	for _, param := range p.Params {
		out = append(out, WidgetDescriptor{
			Key:   param.Key,
			Label: param.Label,
			Kind:  param.Kind,
			Value: resolveValue(param, cfg),
			// Copies défensives : un appelant (UI) ne doit pas pouvoir muter le profil
			// via le descripteur (cohérent avec le deep-copy du registre machine).
			Options:     append([]machine.Option(nil), param.Options...),
			FileExt:     append([]string(nil), param.FileExt...),
			Required:    param.Required,
			LiveMutable: param.LiveMutable,
		})
	}
	return out
}

// resolveValue retourne la valeur courante d'un Param : cfg[Key] si présent, sinon
// son Default.
func resolveValue(param machine.Param, cfg machine.Config) any {
	if v, ok := cfg[param.Key]; ok {
		return v
	}
	return param.Default
}

// isEmpty indique si une valeur résolue compte comme « absente » au sens Required.
func isEmpty(v any) bool {
	return v == nil || v == ""
}

// Validate applique, pour chaque Param : la contrainte Required, puis Param.Validate
// si non nil (nil-safe). Retourne les erreurs indexées par Key (map vide si tout est
// valide).
func Validate(p machine.MachineProfile, cfg machine.Config) map[string]error {
	errs := map[string]error{}
	for _, param := range p.Params {
		v := resolveValue(param, cfg)
		if param.Required && isEmpty(v) {
			errs[param.Key] = fmt.Errorf("paramètre %q requis", param.Key)
			continue
		}
		if param.Validate != nil { // nil-safe : MO5 ne déclare aucun Validate
			if err := param.Validate(v); err != nil {
				errs[param.Key] = err
			}
		}
	}
	return errs
}

// BuildConfig part des valeurs saisies, complète les Default manquants, valide, et
// retourne la Config à passer à MachineProfile.New. Erreur si la validation échoue.
func BuildConfig(p machine.MachineProfile, values machine.Config) (machine.Config, error) {
	out := machine.Config{}
	for k, v := range values { // copie : ne mute pas l'entrée
		out[k] = v
	}
	for _, param := range p.Params { // complète les Default manquants
		if _, ok := out[param.Key]; !ok && param.Default != nil {
			out[param.Key] = param.Default
		}
	}
	if errs := Validate(p, out); len(errs) > 0 {
		for _, param := range p.Params { // erreur déterministe (ordre des Params)
			if err := errs[param.Key]; err != nil {
				return nil, fmt.Errorf("%s : %w", param.Key, err)
			}
		}
	}
	return out, nil
}

// LiveChange est un changement applicable à chaud (overlay) : un Param LiveMutable
// dont la valeur a changé.
type LiveChange struct {
	Key   string
	Value any
}

// DiffLive retourne les changements applicables À CHAUD entre old et new, dans
// l'ordre des Params : UNIQUEMENT les Params LiveMutable dont la valeur diffère. Un
// Param boot-only modifié n'apparaît JAMAIS (il exige un redémarrage, pas l'overlay).
func DiffLive(p machine.MachineProfile, old, next machine.Config) []LiveChange {
	var out []LiveChange
	for _, param := range p.Params {
		if !param.LiveMutable {
			continue // boot-only : jamais applicable à chaud
		}
		// DeepEqual : Config porte des `any` ; une comparaison `!=` paniquerait sur
		// une valeur non comparable (slice, map). Le contrat générique l'autorise.
		if !reflect.DeepEqual(old[param.Key], next[param.Key]) {
			out = append(out, LiveChange{Key: param.Key, Value: next[param.Key]})
		}
	}
	return out
}
