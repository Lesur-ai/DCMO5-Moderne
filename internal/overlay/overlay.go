// Package overlay porte la logique PURE de l'overlay Échap (lot #117, PR-D) : la
// machine d'état des vues (fermé / principal / navigateur de fichiers / confirmation
// d'un changement de machine) et la composition de la pause.
//
// CONTRAINTE : aucun import Ebitengine/ebitenui ici — c'est la part testable EN
// HEADLESS. Le rendu et l'orchestration (impurs, non testés en CI) vivent dans
// internal/app et réutilisent ce Model + uimodel.DescribeLive.
//
// La navigation ENTRE widgets (Tab/flèches/Entrée) est déléguée au focus natif
// d'ebitenui : le Model ne porte donc PAS d'index de sélection (contrairement au
// menu v1), seulement la vue courante et la cible du navigateur de fichiers.
package overlay

// State est la vue courante de l'overlay.
type State int

const (
	StateClosed        State = iota // overlay fermé : émulation au premier plan
	StateMain                       // vue principale : actions + paramètres LiveMutable
	StateBrowse                     // navigateur de fichiers (choix d'un média)
	StateConfirmSwitch              // confirmation d'un changement de machine
)

// Model est l'état PUR de l'overlay. Le zéro-value est un overlay fermé, prêt à
// l'emploi (aucun constructeur requis).
type Model struct {
	state     State
	browseKey string // clé du Param File en cours de navigation (valide si state == StateBrowse)
}

// State retourne la vue courante.
func (m *Model) State() State { return m.state }

// IsOpen indique si l'overlay est affiché (toute vue sauf StateClosed).
func (m *Model) IsOpen() bool { return m.state != StateClosed }

// BrowseKey retourne la clé du Param File en cours de navigation (vide hors StateBrowse).
func (m *Model) BrowseKey() string { return m.browseKey }

// Open ouvre l'overlay sur la vue principale. No-op s'il est DÉJÀ ouvert : on ne
// réinitialise pas une navigation en cours (ex. réappui parasite).
func (m *Model) Open() {
	if m.state == StateClosed {
		m.state = StateMain
	}
}

// Close ferme l'overlay et oublie toute navigation en cours.
func (m *Model) Close() {
	m.state = StateClosed
	m.browseKey = ""
}

// Toggle ouvre l'overlay s'il est fermé, le ferme sinon (Échap au premier niveau).
func (m *Model) Toggle() {
	if m.IsOpen() {
		m.Close()
	} else {
		m.Open()
	}
}

// GoBrowse passe au navigateur de fichiers pour le Param File de clé key.
func (m *Model) GoBrowse(key string) {
	m.state = StateBrowse
	m.browseKey = key
}

// GoConfirmSwitch passe à la confirmation d'un changement de machine.
func (m *Model) GoConfirmSwitch() {
	m.state = StateConfirmSwitch
	m.browseKey = ""
}

// GoMain (re)vient à la vue principale depuis un sous-écran.
func (m *Model) GoMain() {
	m.state = StateMain
	m.browseKey = ""
}

// Back remonte d'un niveau (touche Échap) : un sous-écran (navigateur, confirmation)
// revient à la vue principale ; la vue principale ferme l'overlay (reprise de
// l'émulation). No-op si déjà fermé.
func (m *Model) Back() {
	switch m.state {
	case StateBrowse, StateConfirmSwitch:
		m.GoMain()
	case StateMain:
		m.Close()
	}
}

// ShouldPause indique si l'émulation doit être suspendue, en COMPOSANT la pause
// explicite (F3) et l'ouverture de l'overlay : l'émulation gèle dès que l'une OU
// l'autre est vraie. Conséquence voulue (parité menu v1) : fermer l'overlay alors
// que l'utilisateur avait mis en pause (F3) AVANT de l'ouvrir laisse l'émulation EN
// PAUSE. C'est une fonction libre (pas une méthode) : la pause explicite vit côté app.
func ShouldPause(paused, overlayOpen bool) bool {
	return paused || overlayOpen
}
