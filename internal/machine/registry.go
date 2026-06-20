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
func Profiles() []MachineProfile {
	out := append([]MachineProfile(nil), registry...)
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// ByID retourne le profil d'identifiant id (résolution du flag --machine).
func ByID(id string) (MachineProfile, bool) {
	for _, p := range registry {
		if p.ID == id {
			return p, true
		}
	}
	return MachineProfile{}, false
}
