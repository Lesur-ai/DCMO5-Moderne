package uimodel

import (
	"sort"
	"strings"
)

// Entry est une entrée du navigateur de fichiers (fichier ou dossier).
type Entry struct {
	Name  string
	IsDir bool
}

// Lister énumère le contenu d'un répertoire. Injecté (os.ReadDir en production,
// factice en test) pour rendre le navigateur testable sans système de fichiers réel.
type Lister func(dir string) ([]Entry, error)

// ListDir liste dir via lister : « .. » en tête (remonter), puis les dossiers triés,
// puis les fichiers triés filtrés par fileExt (suffixe, insensible à la casse). Les
// entrées cachées (préfixe « . ») sont masquées. Logique pure portée d'internal/menu.
func ListDir(lister Lister, dir string, fileExt []string) []Entry {
	out := []Entry{{Name: "..", IsDir: true}}
	if lister == nil {
		return out
	}
	raw, err := lister(dir)
	if err != nil {
		return out
	}
	var dirs, files []Entry
	for _, e := range raw {
		if strings.HasPrefix(e.Name, ".") {
			continue // entrées cachées masquées
		}
		if e.IsDir {
			dirs = append(dirs, e)
		} else if matchesExt(e.Name, fileExt) {
			files = append(files, e)
		}
	}
	sort.Slice(dirs, func(i, j int) bool { return dirs[i].Name < dirs[j].Name })
	sort.Slice(files, func(i, j int) bool { return files[i].Name < files[j].Name })
	out = append(out, dirs...)
	out = append(out, files...)
	return out
}

// matchesExt teste si name se termine par l'une des extensions (insensible à la
// casse). Une liste vide n'applique aucun filtre (tout fichier accepté).
func matchesExt(name string, exts []string) bool {
	if len(exts) == 0 {
		return true
	}
	lower := strings.ToLower(name)
	for _, ext := range exts {
		if strings.HasSuffix(lower, strings.ToLower(ext)) {
			return true
		}
	}
	return false
}
