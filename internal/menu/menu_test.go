package menu

// menu_test.go — tests de la logique du menu (purs, headless).
// Non complaisants : chaque cas vérifie une transition d'état ou une décision
// observable, pas seulement l'absence d'erreur.

import "testing"

// fakeLister simule un système de fichiers en mémoire : dir → entrées.
func fakeLister(fs map[string][]Entry) Lister {
	return func(dir string) ([]Entry, error) {
		return fs[dir], nil
	}
}

func TestModel_OpensAndCloses(t *testing.T) {
	m := NewModel(nil)
	if m.IsOpen() {
		t.Fatal("un menu neuf doit être fermé")
	}
	m.Toggle()
	if !m.IsOpen() || m.State() != StateMain {
		t.Fatalf("après Toggle: state=%v, want StateMain ouvert", m.State())
	}
	m.Toggle()
	if m.IsOpen() {
		t.Fatal("second Toggle doit refermer le menu")
	}
}

func TestModel_MainNavigationWraps(t *testing.T) {
	m := NewModel(nil)
	m.Toggle()
	n := len(m.MainLabels())
	// Remonter depuis le premier item doit boucler vers le dernier.
	m.MoveUp()
	if got := m.MainIndex(); got != n-1 {
		t.Errorf("MoveUp depuis 0: index=%d, want %d (bouclage)", got, n-1)
	}
	// Redescendre boucle vers 0.
	m.MoveDown()
	if got := m.MainIndex(); got != 0 {
		t.Errorf("MoveDown depuis %d: index=%d, want 0", n-1, got)
	}
}

// findMainIndex retourne l'index du libellé contenant sub, ou -1.
func findMainIndex(m *Model, sub string) int {
	for i, l := range m.MainLabels() {
		if containsFold(l, sub) {
			return i
		}
	}
	return -1
}

func containsFold(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && indexFold(s, sub) >= 0)
}

func indexFold(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if equalFold(s[i:i+len(sub)], sub) {
			return i
		}
	}
	return -1
}

func equalFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		ca, cb := a[i], b[i]
		if 'A' <= ca && ca <= 'Z' {
			ca += 'a' - 'A'
		}
		if 'A' <= cb && cb <= 'Z' {
			cb += 'a' - 'A'
		}
		if ca != cb {
			return false
		}
	}
	return true
}

func TestModel_EjectProducesAction(t *testing.T) {
	m := NewModel(nil)
	m.Toggle()
	idx := findMainIndex(m, "Ejecter cassette")
	if idx < 0 {
		t.Fatal("item 'Ejecter cassette' introuvable")
	}
	m.mainIndex = idx
	if act := m.Activate("/"); act != ActEjectTape {
		t.Errorf("Activate sur Ejecter cassette: %v, want ActEjectTape", act)
	}
}

func TestModel_InitprogAction(t *testing.T) {
	m := NewModel(nil)
	m.Toggle()
	idx := findMainIndex(m, "Init prog")
	if idx < 0 {
		t.Fatal("item 'Init prog' introuvable")
	}
	m.mainIndex = idx
	if act := m.Activate("/"); act != ActInitprog {
		t.Errorf("Activate sur Init prog: %v, want ActInitprog", act)
	}
}

func TestModel_ResumeClosesMenu(t *testing.T) {
	m := NewModel(nil)
	m.Toggle()
	m.mainIndex = findMainIndex(m, "Reprendre")
	if act := m.Activate("/"); act != ActResume {
		t.Fatalf("Activate Reprendre: %v, want ActResume", act)
	}
	if m.IsOpen() {
		t.Error("Reprendre doit fermer le menu")
	}
}

// TestModel_BrowseFiltersByExtension vérifie que le navigateur ne montre que les
// fichiers du bon type, et toujours « .. » en tête.
func TestModel_BrowseFiltersByExtension(t *testing.T) {
	fs := map[string][]Entry{
		"/sw": {
			{Name: "jeu.k7", IsDir: false},
			{Name: "doc.txt", IsDir: false},
			{Name: "image.fd", IsDir: false},
			{Name: "sousdossier", IsDir: true},
			{Name: ".cache", IsDir: true}, // caché → ignoré
		},
	}
	m := NewModel(fakeLister(fs))
	m.Toggle()
	m.mainIndex = findMainIndex(m, "Charger cassette")
	if act := m.Activate("/sw"); act != ActNone {
		t.Fatalf("ouvrir le navigateur ne doit pas produire d'action: %v", act)
	}
	if m.State() != StateBrowse {
		t.Fatalf("state=%v, want StateBrowse", m.State())
	}
	entries := m.Entries()
	// Attendu : "..", "sousdossier", "jeu.k7" — PAS doc.txt, image.fd, .cache.
	names := map[string]bool{}
	for _, e := range entries {
		names[e.Name] = true
	}
	if entries[0].Name != ".." {
		t.Errorf("première entrée = %q, want ..", entries[0].Name)
	}
	if !names["jeu.k7"] {
		t.Error("jeu.k7 (cassette) devrait être listé")
	}
	if names["doc.txt"] {
		t.Error("doc.txt ne devrait PAS être listé (mauvaise extension)")
	}
	if names["image.fd"] {
		t.Error("image.fd (disquette) ne devrait PAS apparaître en mode cassette")
	}
	if names[".cache"] {
		t.Error(".cache (caché) ne devrait PAS être listé")
	}
	if !names["sousdossier"] {
		t.Error("les dossiers doivent rester navigables")
	}
}

// TestModel_BrowseEntersDirAndChoosesFile vérifie la navigation dans un
// sous-dossier puis la sélection d'un fichier → ActMountChosen + chemin correct.
func TestModel_BrowseEntersDirAndChoosesFile(t *testing.T) {
	fs := map[string][]Entry{
		"/sw":       {{Name: "games", IsDir: true}},
		"/sw/games": {{Name: "arkanoid.k7", IsDir: false}},
	}
	m := NewModel(fakeLister(fs))
	m.Toggle()
	m.mainIndex = findMainIndex(m, "Charger cassette")
	m.Activate("/sw")

	// Sélectionner "games" (index 1, après "..") et entrer.
	m.browseIndex = 1
	if m.Entries()[1].Name != "games" {
		t.Fatalf("entrée 1 = %q, want games", m.Entries()[1].Name)
	}
	if act := m.Activate(""); act != ActNone {
		t.Fatalf("entrer dans un dossier ne produit pas d'action: %v", act)
	}
	if m.CurrentDir() != "/sw/games" {
		t.Fatalf("dir courant = %q, want /sw/games", m.CurrentDir())
	}
	// Choisir le fichier.
	m.browseIndex = 1 // après ".."
	if m.Entries()[1].Name != "arkanoid.k7" {
		t.Fatalf("entrée 1 = %q, want arkanoid.k7", m.Entries()[1].Name)
	}
	if act := m.Activate(""); act != ActMountChosen {
		t.Fatalf("choisir un fichier: %v, want ActMountChosen", act)
	}
	path, kind := m.Chosen()
	if path != "/sw/games/arkanoid.k7" {
		t.Errorf("chemin choisi = %q, want /sw/games/arkanoid.k7", path)
	}
	if kind != KindTape {
		t.Errorf("kind = %v, want KindTape", kind)
	}
	if m.IsOpen() {
		t.Error("choisir un fichier doit fermer le menu")
	}
}

// TestModel_BrowseBackToParent vérifie la remontée via « .. ».
func TestModel_BrowseBackToParent(t *testing.T) {
	fs := map[string][]Entry{
		"/sw/games": {{Name: "a.k7", IsDir: false}},
		"/sw":       {{Name: "games", IsDir: true}},
	}
	m := NewModel(fakeLister(fs))
	m.Toggle()
	m.mainIndex = findMainIndex(m, "Charger cassette")
	m.Activate("/sw/games")
	// browseIndex 0 = "..", l'activer remonte.
	m.browseIndex = 0
	m.Activate("")
	if m.CurrentDir() != "/sw" {
		t.Errorf("après '..': dir = %q, want /sw", m.CurrentDir())
	}
}

// TestModel_BackFromBrowseReturnsToMain vérifie Back() depuis le navigateur.
func TestModel_BackFromBrowseReturnsToMain(t *testing.T) {
	m := NewModel(fakeLister(map[string][]Entry{"/": nil}))
	m.Toggle()
	m.mainIndex = findMainIndex(m, "Charger disquette")
	m.Activate("/")
	if m.State() != StateBrowse {
		t.Fatalf("state=%v, want StateBrowse", m.State())
	}
	m.Back()
	if m.State() != StateMain {
		t.Errorf("après Back: state=%v, want StateMain", m.State())
	}
}

// TestModel_SetIndexClamps vérifie que les setters d'index (survol souris)
// ignorent les valeurs hors bornes sans corrompre l'état.
func TestModel_SetIndexClamps(t *testing.T) {
	m := NewModel(nil)
	m.Toggle()
	n := len(m.MainLabels())
	m.SetMainIndex(2)
	if m.MainIndex() != 2 {
		t.Errorf("SetMainIndex(2): index=%d, want 2", m.MainIndex())
	}
	m.SetMainIndex(-1) // ignoré
	m.SetMainIndex(n)  // ignoré (hors borne)
	if m.MainIndex() != 2 {
		t.Errorf("SetMainIndex hors borne a modifié l'index: %d, want 2", m.MainIndex())
	}
}

// TestModel_VisibleWindow vérifie le calcul de la fenêtre de scroll partagée
// entre rendu et hit-test souris.
func TestModel_VisibleWindow(t *testing.T) {
	fs := map[string][]Entry{"/d": {
		{Name: "a.k7"}, {Name: "b.k7"}, {Name: "c.k7"},
		{Name: "e.k7"}, {Name: "f.k7"}, {Name: "g.k7"},
	}}
	m := NewModel(fakeLister(fs))
	m.Toggle()
	m.mainIndex = findMainIndex(m, "Charger cassette")
	m.Activate("/d") // 7 entrées : ".." + 6 fichiers

	// Tout visible si maxVisible >= n.
	if first, count := m.VisibleWindow(10); first != 0 || count != 7 {
		t.Errorf("VisibleWindow(10) = (%d,%d), want (0,7)", first, count)
	}
	// Fenêtre de 3 centrée sur la sélection en fin de liste : doit coller au bas.
	m.SetBrowseIndex(6)
	first, count := m.VisibleWindow(3)
	if count != 3 || first != 4 {
		t.Errorf("VisibleWindow(3) sel=6 = (%d,%d), want (4,3)", first, count)
	}
	if first+count > len(m.Entries()) {
		t.Errorf("fenêtre déborde: first=%d count=%d n=%d", first, count, len(m.Entries()))
	}
}

// TestModel_CartExtensions vérifie que les trois extensions cartouche passent.
func TestModel_CartExtensions(t *testing.T) {
	fs := map[string][]Entry{
		"/c": {
			{Name: "a.rom", IsDir: false},
			{Name: "b.m5", IsDir: false},
			{Name: "c.memo5", IsDir: false},
			{Name: "d.k7", IsDir: false}, // pas une cartouche
		},
	}
	m := NewModel(fakeLister(fs))
	m.Toggle()
	m.mainIndex = findMainIndex(m, "Charger cartouche")
	m.Activate("/c")
	names := map[string]bool{}
	for _, e := range m.Entries() {
		names[e.Name] = true
	}
	for _, want := range []string{"a.rom", "b.m5", "c.memo5"} {
		if !names[want] {
			t.Errorf("%s devrait être listé comme cartouche", want)
		}
	}
	if names["d.k7"] {
		t.Error("d.k7 ne devrait PAS apparaître en mode cartouche")
	}
}
