package machine

import "sort"

// registry contient les profils enregistrés par les paquets de machines via init().
var registry []MachineProfile

// Register ajoute un profil au registre. Appelé en init() par chaque paquet machine
// (ex. internal/machine/mo5). Non concurrent-safe : l'enregistrement a lieu au
// chargement des paquets, avant toute utilisation.
func Register(p MachineProfile) {
	registry = append(registry, p)
}

// Profiles retourne les profils enregistrés, triés par ID (ordre stable pour l'UI).
// Chaque profil est une copie profonde (cf. cloneProfile) : muter le résultat ne
// corrompt pas le registre global.
func Profiles() []MachineProfile {
	out := make([]MachineProfile, len(registry))
	for i, p := range registry {
		out[i] = cloneProfile(p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// ByID retourne une copie profonde du profil d'identifiant id (résolution du flag
// --machine). La copie protège le schéma statique d'une mutation par l'appelant.
func ByID(id string) (MachineProfile, bool) {
	for _, p := range registry {
		if p.ID == id {
			return cloneProfile(p), true
		}
	}
	return MachineProfile{}, false
}

// cloneProfile copie en profondeur les tranches du descripteur (Params et, pour
// chaque Param, Options/FileExt) afin qu'un appelant (launcher/overlay) ne puisse pas
// corrompre le registre global en mutant le schéma rendu (revue Codex, P2). Default
// (any) et Validate (func) sont partagés : traités comme immuables par convention.
func cloneProfile(p MachineProfile) MachineProfile {
	cp := p
	if p.Params != nil {
		cp.Params = make([]Param, len(p.Params))
		for i, pa := range p.Params {
			if pa.Options != nil {
				pa.Options = append([]Option(nil), pa.Options...)
			}
			if pa.FileExt != nil {
				pa.FileExt = append([]string(nil), pa.FileExt...)
			}
			cp.Params[i] = pa
		}
	}
	return cp
}
