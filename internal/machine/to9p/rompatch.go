package to9p

// patchReport reprend la forme des patchers MO5/TO8D. Pour le TO9+ Lot #186,
// OK=false ne reconnaît aucune variante : il signale seulement une violation
// d'invariant de taille avant mutation.
type patchReport struct {
	Applied int
	Already int
	OK      bool
}

// applyROMPatches est le point d'extension TO9+ pour les patchs de traps ROM.
//
// Lot #186 : aucun point de patch TO9+ n'est encore validé, donc cette fonction
// est volontairement un no-op. Le contrôle de taille est redondant avec splitROM
// dans le chemin de production actuel ; il garde le contrat forward-compatible du
// patcher : patchs en mémoire uniquement, garde structurelle tout-ou-rien, jamais
// d'écriture sur le fichier ROM fourni.
func applyROMPatches(romMon, romBasic []byte) patchReport {
	if len(romMon) != romMonSize || len(romBasic) != romBasicSize {
		return patchReport{OK: false}
	}
	return patchReport{OK: true}
}
