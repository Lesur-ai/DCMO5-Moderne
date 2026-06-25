package app

import (
	"image"
	"testing"

	"github.com/ebitenui/ebitenui/widget"
)

// fakeSized implémente widget.PreferredSizeLocateableWidget SANS aucun widget ebitenui
// réel ni ebiten.NewImage : le test reste donc sûr en headless (`go test ./...`),
// contrairement à un test qui construirait le launcher (init Ebitengine/GLFW → hang/échec
// hors affichage, comme l'a confirmé la revue Codex). GetWidget/SetLocation ne sont pas
// appelés par fixedHeightLayout.PreferredSize.
type fakeSized struct{ w, h int }

func (f fakeSized) GetWidget() *widget.Widget     { return nil }
func (f fakeSized) PreferredSize() (int, int)     { return f.w, f.h }
func (f fakeSized) SetLocation(_ image.Rectangle) {}
func (f fakeSized) Validate()                     {}

// TestFixedHeightLayoutPreferredWidthNonZero verrouille la régression de DÉBORDEMENT du
// navigateur de fichiers : fixedHeightLayout DOIT annoncer une largeur > 0. Sinon
// RowLayout.PreferredSize (qui unionne les rects des enfants via image.Rectangle.Union)
// ignore le rect « vide » (largeur OU hauteur nulle), la carte ne réserve pas la hauteur
// de liste et celle-ci déborde sous le panneau. La hauteur reste la valeur fixe bornée.
func TestFixedHeightLayoutPreferredWidthNonZero(t *testing.T) {
	const h = browserListMaxPx
	lay := fixedHeightLayout{h: h}

	// Avec contenu : largeur = max des largeurs enfants (> 0) ; hauteur = valeur fixe.
	w, gotH := lay.PreferredSize([]widget.PreferredSizeLocateableWidget{
		fakeSized{w: 131, h: 9999},
	})
	if w <= 0 {
		t.Fatalf("largeur préférée=%d : DOIT être > 0 (sinon rect « vide » ignoré par Union → débordement)", w)
	}
	if w != 131 {
		t.Fatalf("largeur préférée=%d, attendu 131 (largeur du contenu)", w)
	}
	if gotH != h {
		t.Fatalf("hauteur préférée=%d, attendu hauteur fixe %d", gotH, h)
	}

	// Sans contenu (répertoire vide) : largeur plancher > 0, jamais « vide ».
	w0, gotH0 := lay.PreferredSize(nil)
	if w0 <= 0 {
		t.Fatalf("largeur préférée (vide)=%d : DOIT rester > 0", w0)
	}
	if gotH0 != h {
		t.Fatalf("hauteur préférée (vide)=%d, attendu %d", gotH0, h)
	}
}
