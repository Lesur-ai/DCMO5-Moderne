// Package menu porte la logique d'état du menu de pilotage MO5.
//
// Toute la navigation, le filtrage de fichiers et les décisions vivent ici,
// sans aucune dépendance graphique : le package est entièrement testable
// headless. La couche application (internal/app) consomme ce modèle pour le
// rendu Ebitengine et l'exécution des actions ; elle ne décide de rien.
package menu

import (
	"sort"
	"strings"
)

// State est l'état d'affichage du menu.
type State int

const (
	StateClosed State = iota // émulation en cours, pas de menu
	StateMain                // menu principal (liste d'actions)
	StateBrowse              // navigateur de fichiers
)

// Action est une intention produite par le modèle, exécutée par la couche
// application (qui seule connaît la machine et le système de fichiers).
type Action int

const (
	ActNone        Action = iota
	ActResume             // fermer le menu, reprendre l'émulation
	ActReset              // reset matériel (efface la RAM)
	ActInitprog           // reset doux (relance le programme, garde la RAM)
	ActQuit               // quitter l'application
	ActEjectTape          // éjecter la cassette
	ActEjectDisk          // éjecter la disquette
	ActEjectCart          // éjecter la cartouche
	ActMountChosen        // un fichier a été choisi (voir Chosen())
)

// Kind distingue la cible d'un chargement de fichier.
type Kind int

const (
	KindNone Kind = iota
	KindTape
	KindDisk
	KindCart
)

// extensionsByKind liste les extensions acceptées par type de média
// (minuscules, point inclus).
var extensionsByKind = map[Kind][]string{
	KindTape: {".k7"},
	KindDisk: {".fd"},
	KindCart: {".rom", ".m5", ".memo5"},
}

// mainEntry est une ligne du menu principal.
type mainEntry struct {
	Label  string
	action Action
	kind   Kind // non-nul pour les actions « charger »
}

// mainMenu est la liste ordonnée des actions du menu principal.
var mainMenu = []mainEntry{
	{"Charger cassette (.k7)", ActNone, KindTape},
	{"Charger disquette (.fd)", ActNone, KindDisk},
	{"Charger cartouche (.rom)", ActNone, KindCart},
	{"Ejecter cassette", ActEjectTape, KindNone},
	{"Ejecter disquette", ActEjectDisk, KindNone},
	{"Ejecter cartouche", ActEjectCart, KindNone},
	{"Init prog", ActInitprog, KindNone},
	{"Reset machine", ActReset, KindNone},
	{"Reprendre", ActResume, KindNone},
	{"Quitter", ActQuit, KindNone},
}

// Entry est une entrée de répertoire fournie au modèle (abstrait l'OS).
type Entry struct {
	Name  string
	IsDir bool
}

// Lister liste le contenu d'un répertoire. Injecté pour rester testable.
type Lister func(dir string) ([]Entry, error)

// Model porte tout l'état du menu de pilotage.
type Model struct {
	state State

	mainIndex int // sélection dans le menu principal

	// Navigateur de fichiers
	kind        Kind
	dir         string
	entries     []Entry // entries[0] est toujours « .. » (remonter)
	browseIndex int
	chosen      string // chemin complet du fichier choisi (après ActMountChosen)
	lister      Lister
}

// NewModel crée un menu fermé utilisant le lister fourni.
func NewModel(lister Lister) *Model {
	return &Model{state: StateClosed, lister: lister}
}

// IsOpen indique si le menu capte les entrées (émulation suspendue).
func (m *Model) IsOpen() bool { return m.state != StateClosed }

// Toggle ouvre le menu principal s'il est fermé, le ferme sinon.
func (m *Model) Toggle() {
	if m.state == StateClosed {
		m.state = StateMain
		m.mainIndex = 0
	} else {
		m.state = StateClosed
	}
}

// Close ferme le menu sans produire d'action.
func (m *Model) Close() { m.state = StateClosed }

// SetMainIndex positionne la sélection du menu principal (clamp, ignore hors
// bornes). Utilisé par le survol souris.
func (m *Model) SetMainIndex(i int) {
	if i >= 0 && i < len(mainMenu) {
		m.mainIndex = i
	}
}

// SetBrowseIndex positionne la sélection du navigateur (clamp, ignore hors
// bornes). Utilisé par le survol et la molette souris.
func (m *Model) SetBrowseIndex(i int) {
	if i >= 0 && i < len(m.entries) {
		m.browseIndex = i
	}
}

// VisibleWindow calcule la tranche d'entrées affichable dans le navigateur pour
// une hauteur de maxVisible lignes, centrée sur la sélection. Retourne l'index
// de la première entrée visible et le nombre d'entrées visibles. Logique pure,
// partagée par le rendu et le hit-test souris pour rester cohérents.
func (m *Model) VisibleWindow(maxVisible int) (first, count int) {
	n := len(m.entries)
	if maxVisible <= 0 || n == 0 {
		return 0, 0
	}
	if n <= maxVisible {
		return 0, n
	}
	first = m.browseIndex - maxVisible/2
	if first < 0 {
		first = 0
	}
	if first > n-maxVisible {
		first = n - maxVisible
	}
	return first, maxVisible
}

// MoveUp déplace la sélection vers le haut (avec bouclage).
func (m *Model) MoveUp() {
	switch m.state {
	case StateMain:
		m.mainIndex = wrapIndex(m.mainIndex-1, len(mainMenu))
	case StateBrowse:
		m.browseIndex = wrapIndex(m.browseIndex-1, len(m.entries))
	}
}

// MoveDown déplace la sélection vers le bas (avec bouclage).
func (m *Model) MoveDown() {
	switch m.state {
	case StateMain:
		m.mainIndex = wrapIndex(m.mainIndex+1, len(mainMenu))
	case StateBrowse:
		m.browseIndex = wrapIndex(m.browseIndex+1, len(m.entries))
	}
}

// wrapIndex renvoie i borné circulairement dans [0,n). n<=0 → 0.
func wrapIndex(i, n int) int {
	if n <= 0 {
		return 0
	}
	return ((i % n) + n) % n
}

// Back remonte d'un niveau : du navigateur on revient au menu principal ;
// du menu principal on ferme le menu.
func (m *Model) Back() {
	switch m.state {
	case StateBrowse:
		m.state = StateMain
	case StateMain:
		m.state = StateClosed
	}
}

// Activate valide la sélection courante et retourne l'action à exécuter par la
// couche application. startDir est le répertoire de départ du navigateur.
func (m *Model) Activate(startDir string) Action {
	switch m.state {
	case StateMain:
		return m.activateMain(startDir)
	case StateBrowse:
		return m.activateBrowse()
	default:
		return ActNone
	}
}

func (m *Model) activateMain(startDir string) Action {
	if m.mainIndex < 0 || m.mainIndex >= len(mainMenu) {
		return ActNone
	}
	e := mainMenu[m.mainIndex]
	if e.kind != KindNone {
		// Action « charger » : ouvrir le navigateur, pas d'action immédiate.
		m.openBrowser(e.kind, startDir)
		return ActNone
	}
	if e.action == ActResume {
		m.state = StateClosed
	}
	return e.action
}

// openBrowser bascule en mode navigateur sur dir, pour le type de média donné.
func (m *Model) openBrowser(kind Kind, dir string) {
	m.kind = kind
	m.state = StateBrowse
	m.chosen = ""
	m.loadDir(dir)
}

// loadDir charge les entrées de dir, filtrées par le type de média courant.
// La première entrée est toujours « .. » pour remonter.
func (m *Model) loadDir(dir string) {
	m.dir = dir
	m.browseIndex = 0
	m.entries = []Entry{{Name: "..", IsDir: true}}
	if m.lister == nil {
		return
	}
	raw, err := m.lister(dir)
	if err != nil {
		return
	}
	var dirs, files []Entry
	for _, e := range raw {
		if strings.HasPrefix(e.Name, ".") {
			continue // masquer les entrées cachées
		}
		if e.IsDir {
			dirs = append(dirs, e)
		} else if m.matchesExtension(e.Name) {
			files = append(files, e)
		}
	}
	sort.Slice(dirs, func(i, j int) bool { return dirs[i].Name < dirs[j].Name })
	sort.Slice(files, func(i, j int) bool { return files[i].Name < files[j].Name })
	m.entries = append(m.entries, dirs...)
	m.entries = append(m.entries, files...)
}

// matchesExtension teste si name correspond à une extension du média courant.
func (m *Model) matchesExtension(name string) bool {
	lower := strings.ToLower(name)
	for _, ext := range extensionsByKind[m.kind] {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	return false
}

// activateBrowse traite la sélection dans le navigateur : « .. » et dossiers
// naviguent, un fichier produit ActMountChosen.
func (m *Model) activateBrowse() Action {
	if m.browseIndex < 0 || m.browseIndex >= len(m.entries) {
		return ActNone
	}
	e := m.entries[m.browseIndex]
	if e.IsDir {
		if e.Name == ".." {
			m.loadDir(parentDir(m.dir))
		} else {
			m.loadDir(joinPath(m.dir, e.Name))
		}
		return ActNone
	}
	m.chosen = joinPath(m.dir, e.Name)
	m.state = StateClosed
	return ActMountChosen
}

// Chosen retourne le chemin du dernier fichier choisi et le type de média visé.
func (m *Model) Chosen() (path string, kind Kind) { return m.chosen, m.kind }

// ── Accesseurs pour le rendu ──────────────────────────────────────────────────

func (m *Model) State() State       { return m.state }
func (m *Model) MainIndex() int     { return m.mainIndex }
func (m *Model) BrowseIndex() int   { return m.browseIndex }
func (m *Model) Entries() []Entry   { return m.entries }
func (m *Model) CurrentDir() string { return m.dir }
func (m *Model) Kind() Kind         { return m.kind }
func (m *Model) MainLabels() []string {
	labels := make([]string, len(mainMenu))
	for i, e := range mainMenu {
		labels[i] = e.Label
	}
	return labels
}

// ── Manipulation de chemins (style slash, indépendante de l'OS) ───────────────

func parentDir(dir string) string {
	dir = strings.TrimRight(dir, "/")
	if dir == "" {
		return "/"
	}
	i := strings.LastIndex(dir, "/")
	if i <= 0 {
		return "/"
	}
	return dir[:i]
}

func joinPath(dir, name string) string {
	if strings.HasSuffix(dir, "/") {
		return dir + name
	}
	return dir + "/" + name
}
