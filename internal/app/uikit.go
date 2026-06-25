// uikit.go — couche de rendu partagée (lot #117, Inc 0). Thème (polices, images de
// bouton, couleurs de texte) et primitives de widgets ebitenui mutualisées entre le
// launcher ET l'overlay Échap. Extrait du launcher SANS changement de comportement :
// la logique de contrôle (addParam, fileField, fileList, navigation) reste propre à
// chaque écran car elle capture son état ; seules les briques visuelles neutres
// vivent ici. La palette (colBG, colAccent, …) et les constantes de mise en page
// restent déclarées dans launcher.go (mêmes vars de paquet, accessibles ici).
package app

import (
	"bytes"
	"fmt"
	"image/color"
	"os"

	eimage "github.com/ebitenui/ebitenui/image"
	"github.com/ebitenui/ebitenui/widget"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"
)

// uiKit porte les ressources de rendu partagées (polices text/v2, images de bouton,
// couleurs de texte) et les primitives de widgets communes. Les écrans (launcher,
// overlay) l'EMBARQUENT : les appels `l.button(...)`, `l.card()`, `l.faceTitle`
// restent inchangés par promotion de champ Go.
type uiKit struct {
	faceTitle *text.Face
	faceLabel *text.Face
	faceBtn   *text.Face
	btnImg    *widget.ButtonImage // bouton standard
	btnSel    *widget.ButtonImage // accent : sélection / action primaire
	fieldImg  *widget.ButtonImage // zone de champ « plate » (nom de fichier, chevron, croix)
	txtColor  *widget.ButtonTextColor
	txtOnSel  *widget.ButtonTextColor
}

// newUIKit construit les ressources de rendu partagées (polices + images + couleurs).
func newUIKit() *uiKit {
	return &uiKit{
		faceTitle: loadFace(gobold.TTF, 26),
		faceLabel: loadFace(goregular.TTF, 15),
		faceBtn:   loadFace(goregular.TTF, 15),
		btnImg: &widget.ButtonImage{
			Idle:    eimage.NewNineSliceColor(colBtn),
			Hover:   eimage.NewNineSliceColor(colBtnHi),
			Pressed: eimage.NewNineSliceColor(colBtnLo),
		},
		btnSel: &widget.ButtonImage{
			Idle:    eimage.NewNineSliceColor(colAccent),
			Hover:   eimage.NewNineSliceColor(colAccentHi),
			Pressed: eimage.NewNineSliceColor(colAccentLo),
		},
		fieldImg: &widget.ButtonImage{
			Idle:    eimage.NewNineSliceColor(colField),
			Hover:   eimage.NewNineSliceColor(colFieldHi),
			Pressed: eimage.NewNineSliceColor(colFieldHi),
		},
		txtColor: &widget.ButtonTextColor{Idle: colText, Hover: colWhite, Pressed: colText},
		txtOnSel: &widget.ButtonTextColor{Idle: colWhite, Hover: colWhite, Pressed: colWhite},
	}
}

// loadFace charge une police vectorielle TTF embarquée dans golang.org/x/image (Go
// fonts, BSD — ce ne sont PAS des assets Thomson sous réserve). En cas d'échec de
// parsing (ne devrait jamais arriver), on retombe sur la police bitmap basicfont
// plutôt que de paniquer : l'UI reste affichée, juste plus laide.
func loadFace(ttf []byte, size float64) *text.Face {
	var f text.Face
	if src, err := text.NewGoTextFaceSource(bytes.NewReader(ttf)); err != nil {
		fmt.Fprintf(os.Stderr, "ui: police vectorielle indisponible (%v), repli bitmap\n", err)
		f = text.NewGoXFace(basicfont.Face7x13)
	} else {
		f = &text.GoTextFace{Source: src, Size: size}
	}
	return &f
}

// card construit le conteneur « carte » centré (panneau sombre, padding, colonne
// verticale ; les enfants pleine largeur portent stretchH()).
func (k *uiKit) card() *widget.Container {
	return widget.NewContainer(
		widget.ContainerOpts.BackgroundImage(eimage.NewNineSliceColor(colPanel)),
		widget.ContainerOpts.WidgetOpts(
			widget.WidgetOpts.MinSize(cardWidth, 0),
			widget.WidgetOpts.LayoutData(widget.AnchorLayoutData{
				HorizontalPosition: widget.AnchorLayoutPositionCenter,
				VerticalPosition:   widget.AnchorLayoutPositionCenter,
			}),
		),
		widget.ContainerOpts.Layout(widget.NewRowLayout(
			widget.RowLayoutOpts.Direction(widget.DirectionVertical),
			widget.RowLayoutOpts.Padding(&widget.Insets{Top: 26, Bottom: 26, Left: 28, Right: 28}),
			widget.RowLayoutOpts.Spacing(14),
		)),
	)
}

// separator : fine ligne horizontale (1px) remplissant la largeur de la carte.
func (k *uiKit) separator() *widget.Container {
	return widget.NewContainer(
		widget.ContainerOpts.BackgroundImage(eimage.NewNineSliceColor(colBorder)),
		widget.ContainerOpts.WidgetOpts(stretchH(), widget.WidgetOpts.MinSize(0, 1)),
	)
}

// sectionLabel : intitulé de section discret (gris).
func (k *uiKit) sectionLabel(s string) *widget.Text {
	return widget.NewText(widget.TextOpts.Text(s, k.faceLabel, colMuted))
}

// hint : note d'aide discrète sous un groupe.
func (k *uiKit) hint(s string) *widget.Text {
	return widget.NewText(widget.TextOpts.Text(s, k.faceLabel, colMuted))
}

// button : bouton standard (image + couleur de texte fournies), hauteur stable.
func (k *uiKit) button(label string, img *widget.ButtonImage, txt *widget.ButtonTextColor, onClick func()) *widget.Button {
	return widget.NewButton(
		widget.ButtonOpts.Image(img),
		widget.ButtonOpts.Text(label, k.faceBtn, txt),
		widget.ButtonOpts.TextPadding(&widget.Insets{Top: 8, Bottom: 8, Left: 14, Right: 14}),
		widget.ButtonOpts.WidgetOpts(widget.WidgetOpts.MinSize(0, 34)),
		widget.ButtonOpts.ClickedHandler(func(*widget.ButtonClickedEventArgs) { onClick() }),
	)
}

// squareButton : petit bouton carré (incréments Int).
func (k *uiKit) squareButton(label string, onClick func()) *widget.Button {
	return widget.NewButton(
		widget.ButtonOpts.Image(k.btnImg),
		widget.ButtonOpts.Text(label, k.faceBtn, k.txtColor),
		widget.ButtonOpts.WidgetOpts(widget.WidgetOpts.MinSize(34, 34)),
		widget.ButtonOpts.ClickedHandler(func(*widget.ButtonClickedEventArgs) { onClick() }),
	)
}

// primaryButton : action principale en accent, pleine largeur, plus haute.
func (k *uiKit) primaryButton(label string, onClick func()) *widget.Button {
	return widget.NewButton(
		widget.ButtonOpts.Image(k.btnSel),
		widget.ButtonOpts.Text(label, k.faceBtn, k.txtOnSel),
		widget.ButtonOpts.TextPadding(&widget.Insets{Top: 11, Bottom: 11, Left: 14, Right: 14}),
		widget.ButtonOpts.WidgetOpts(stretchH(), widget.WidgetOpts.MinSize(0, 42)),
		widget.ButtonOpts.ClickedHandler(func(*widget.ButtonClickedEventArgs) { onClick() }),
	)
}

// glyphButton : petit bouton « plat » (fond de champ) portant un glyphe (× ou »).
func (k *uiKit) glyphButton(glyph string, col color.Color, onClick func()) *widget.Button {
	return widget.NewButton(
		widget.ButtonOpts.Image(k.fieldImg),
		widget.ButtonOpts.Text(glyph, k.faceBtn, &widget.ButtonTextColor{Idle: col, Hover: colWhite, Pressed: col}),
		widget.ButtonOpts.WidgetOpts(widget.WidgetOpts.MinSize(28, 34)),
		widget.ButtonOpts.ClickedHandler(func(*widget.ButtonClickedEventArgs) { onClick() }),
	)
}
