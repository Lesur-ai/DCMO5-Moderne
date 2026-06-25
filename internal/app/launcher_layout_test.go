package app

import (
	"fmt"
	"testing"

	"github.com/Lesur-ai/dcmo5/internal/uimodel"
)

// NOTE CI : le paquet internal/app est exclu du job `go test -race` de la CI (init
// graphique Ebitengine au Render/RunGame). Ce test est donc un GARDE-FOU LOCAL
// (`go test ./internal/app/`). Il reste rigoureux : il reproduit et verrouille la
// régression de DÉBORDEMENT du navigateur de fichiers — la carte ne réservait pas la
// hauteur de la liste car fixedHeightLayout renvoyait une largeur 0, or
// image.Rectangle.Union (utilisé par RowLayout.PreferredSize) ignore tout rect « vide »
// (largeur OU hauteur nulle). PreferredSize est purement CPU (pas de Render) → s'exécute
// sans contexte graphique.
func TestBrowserCardReservesBoundedListHeight(t *testing.T) {
	lister := func(string) ([]uimodel.Entry, error) {
		ents := make([]uimodel.Entry, 30)
		for i := range ents {
			ents[i] = uimodel.Entry{Name: fmt.Sprintf("fichier_%02d.k7", i)}
		}
		return ents, nil
	}
	l := newLauncher(nil, "/tmp", lister, nil)
	l.browseKey = "rom"
	l.browseDir = "/tmp"
	l.browseExt = nil

	// 1) La liste seule est bornée à browserListMaxPx (30+ entrées → cap atteint).
	entries := uimodel.ListDir(l.lister, l.browseDir, l.browseExt)
	if len(entries) <= browserListMaxPx/browserItemPx {
		t.Fatalf("cas de test invalide : %d entrées ne dépassent pas le seuil de défilement", len(entries))
	}
	_, listH := l.fileList(entries).PreferredSize()
	if listH > browserListMaxPx {
		t.Fatalf("liste non bornée : hauteur=%d > cap=%d", listH, browserListMaxPx)
	}

	// 2) La carte complète du navigateur DOIT réserver la hauteur de la liste (avant le
	//    correctif elle valait ~178px et ignorait les 360px → la liste débordait sous le
	//    panneau) ET tenir dans la fenêtre.
	card := l.card()
	l.buildBrowser(card)
	_, cardH := card.PreferredSize()
	if cardH < listH {
		t.Fatalf("la carte (%d px) ne réserve pas la hauteur de liste (%d px) — débordement", cardH, listH)
	}
	if cardH > launcherHeight {
		t.Fatalf("la carte (%d px) déborde la fenêtre (%d px)", cardH, launcherHeight)
	}
}

// TestLauncherKeyboardFocus vérifie le pilotage clavier (même garde-fou local que
// ci-dessus) : l'ouverture du navigateur pose le focus sur la LISTE (flèches/Enter
// immédiats sans souris), et ÉCHAP referme le navigateur en re-focalisant un contrôle de
// la vue principale. SetFocusedWidget/Focus n'effectuent que de la logique d'événement
// (pas de Render) → s'exécutent sans contexte graphique.
func TestLauncherKeyboardFocus(t *testing.T) {
	lister := func(string) ([]uimodel.Entry, error) {
		return []uimodel.Entry{{Name: "a.k7"}, {Name: "b.k7"}, {Name: "c.k7"}}, nil
	}
	l := newLauncher(nil, "/tmp", lister, nil)

	// Au démarrage (vue principale), un contrôle doit être focalisé (pilotable d'emblée).
	if l.ui.GetFocusedWidget() == nil {
		t.Fatal("vue principale : aucun widget focalisé au démarrage (clavier inutilisable sans souris)")
	}

	// Ouverture du navigateur → focus sur la liste (navigation flèches/Enter immédiate).
	l.browseKey = "rom"
	l.browseDir = "/tmp"
	l.browseExt = nil
	l.rebuild()
	if l.browseList == nil {
		t.Fatal("navigateur : référence de liste non capturée")
	}
	if l.ui.GetFocusedWidget() != l.browseList {
		t.Fatalf("navigateur : le focus clavier doit porter sur la liste, pas %v", l.ui.GetFocusedWidget())
	}

	// ÉCHAP en navigateur → retour vue principale + focus sur un contrôle principal.
	if !l.escapePressed() {
		t.Fatal("ÉCHAP en navigateur doit être traité (retour), pas propagé en quit")
	}
	if l.browseKey != "" {
		t.Fatal("ÉCHAP doit fermer le navigateur")
	}
	if l.ui.GetFocusedWidget() == nil {
		t.Fatal("après retour, un contrôle de la vue principale doit rester focalisé")
	}

	// ÉCHAP en vue principale → non traité (l'appelant quittera l'application).
	if l.escapePressed() {
		t.Fatal("ÉCHAP en vue principale ne doit PAS être consommé (doit déclencher le quit)")
	}
}
