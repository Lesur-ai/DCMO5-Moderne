// launcher.go — écran de lancement ebitenui (lot #117, PR-C2). RENDU NON TESTABLE
// EN CI (init graphique) : la logique pure vit dans internal/uimodel (Describe,
// BuildConfig, MediaMounts, InitialValues, ListDir), testée en CI. Ici on ne fait
// que CÂBLER ces fonctions à des widgets ebitenui, sans aucune connaissance d'un
// modèle précis : la liste des machines et leurs paramètres sont rendus
// génériquement depuis machine.MachineProfile (data-driven, DESIGN §9).
//
// Choix de robustesse : tous les contrôles sont des BOUTONS (Bool=bascule,
// Enum=cycle, Int=±, File=navigateur). On évite ainsi TextInput/Checkbox d'ebitenui,
// qui exigent un thème complet et paniquent sur un paramètre manquant (revue de plan
// Codex). Cela rend néanmoins les 4 ParamKind visibles, ce que valide l'owner.
//
// Présentation : carte centrée sur fond sombre, typographie vectorielle (goregular /
// gobold via text/v2), en-tête, sélecteur de machines (état sélectionné en accent),
// grille « libellé : contrôle » alignée, séparateurs, et action « Démarrer » primaire
// pleine largeur. La structure visuelle est portée ICI ; le schéma reste data-driven.
package app

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"strings"

	"github.com/ebitenui/ebitenui"
	eimage "github.com/ebitenui/ebitenui/image"
	"github.com/ebitenui/ebitenui/widget"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"

	"github.com/Lesur-ai/dcmo5/internal/machine"
	"github.com/Lesur-ai/dcmo5/internal/uimodel"
)

const (
	launcherWidth  = 760
	launcherHeight = 640
	cardWidth      = 600 // largeur stable de la carte (sinon elle « danse » avec le contenu)

	// Limites de troncature (purement visuelles, dépendantes de cardWidth) pour
	// éviter tout débordement hors de la carte.
	maxFileNameRunes = 30 // nom de fichier affiché dans un champ
	maxPathRunes     = 58 // chemin du répertoire courant dans le navigateur

	// Dimensions du navigateur de fichiers : au-delà de browserListMaxPx, la liste
	// défile à la molette au lieu de déborder hors de la fenêtre.
	browserItemPx    = 38  // hauteur approx. d'une entrée (bouton 34 + espacement 4)
	browserListMaxPx = 360 // hauteur max de la liste avant défilement
)

// Palette : thème sombre cohérent + un accent bleu pour les actions/états primaires.
var (
	colBG       = color.NRGBA{R: 0x12, G: 0x14, B: 0x1c, A: 0xff}
	colPanel    = color.NRGBA{R: 0x1f, G: 0x22, B: 0x30, A: 0xff}
	colBorder   = color.NRGBA{R: 0x34, G: 0x39, B: 0x52, A: 0xff}
	colText     = color.NRGBA{R: 0xe9, G: 0xec, B: 0xf5, A: 0xff}
	colMuted    = color.NRGBA{R: 0x96, G: 0x9c, B: 0xb4, A: 0xff}
	colAccent   = color.NRGBA{R: 0x5b, G: 0x8c, B: 0xff, A: 0xff}
	colAccentHi = color.NRGBA{R: 0x78, G: 0xa2, B: 0xff, A: 0xff}
	colAccentLo = color.NRGBA{R: 0x46, G: 0x70, B: 0xd8, A: 0xff}
	colBtn      = color.NRGBA{R: 0x2b, G: 0x2f, B: 0x44, A: 0xff}
	colBtnHi    = color.NRGBA{R: 0x3a, G: 0x3f, B: 0x5c, A: 0xff}
	colBtnLo    = color.NRGBA{R: 0x22, G: 0x25, B: 0x36, A: 0xff}
	colField    = color.NRGBA{R: 0x16, G: 0x18, B: 0x22, A: 0xff} // fond de champ (inset)
	colFieldHi  = color.NRGBA{R: 0x23, G: 0x27, B: 0x38, A: 0xff} // survol d'une zone de champ
	colDanger   = color.NRGBA{R: 0xff, G: 0x6b, B: 0x6b, A: 0xff}
	colWhite    = color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
)

// startRequest : profil + valeurs saisies, transmis à l'App à l'action « Démarrer ».
// L'App fait ensuite BuildConfig + profile.New (et peut renvoyer une erreur affichée).
type startRequest struct {
	profile machine.MachineProfile
	values  machine.Config
}

// launcher porte l'état du rendu : profils proposés, profil sélectionné, valeurs
// courantes, état du navigateur de fichiers, et le signal « Démarrer ». La fonction
// rebuild() reconstruit l'arbre de widgets selon cet état.
type launcher struct {
	ui       *ebitenui.UI
	root     *widget.Container
	profiles []machine.MachineProfile
	selected int
	values   machine.Config
	mediaDir string
	lister   uimodel.Lister

	// Navigateur de fichiers actif si browseKey != "" (clé du Param File en cours).
	browseKey  string
	browseDir  string
	browseExt  []string
	browseList widget.Focuser // widget List du navigateur, cible du focus clavier en mode navigateur

	// lastBuildBrowse mémorise le mode du dernier rebuild (navigateur vs principal). Sert à
	// décider, après reconstruction, s'il faut réinitialiser le focus (changement de mode)
	// ou restaurer le contrôle de même rang (même mode) — cf. restoreFocus.
	lastBuildBrowse bool

	errText string

	start    bool
	startReq startRequest

	// Ressources de rendu partagées (polices text/v2, images de bouton, couleurs).
	// ebitenui attend des *text.Face (pointeur sur l'interface).
	faceTitle *text.Face
	faceLabel *text.Face
	faceBtn   *text.Face
	btnImg    *widget.ButtonImage // bouton standard
	btnSel    *widget.ButtonImage // accent : machine sélectionnée / action primaire
	fieldImg  *widget.ButtonImage // zone de champ « plate » (nom de fichier, chevron, croix)
	txtColor  *widget.ButtonTextColor
	txtOnSel  *widget.ButtonTextColor
}

// loadFace charge une police vectorielle TTF embarquée dans golang.org/x/image (Go
// fonts, BSD — ce ne sont PAS des assets Thomson sous réserve). En cas d'échec de
// parsing (ne devrait jamais arriver), on retombe sur la police bitmap basicfont
// plutôt que de paniquer : l'UI reste affichée, juste plus laide.
func loadFace(ttf []byte, size float64) *text.Face {
	var f text.Face
	if src, err := text.NewGoTextFaceSource(bytes.NewReader(ttf)); err != nil {
		fmt.Fprintf(os.Stderr, "launcher: police vectorielle indisponible (%v), repli bitmap\n", err)
		f = text.NewGoXFace(basicfont.Face7x13)
	} else {
		f = &text.GoTextFace{Source: src, Size: size}
	}
	return &f
}

// osListerUI liste un répertoire réel pour le navigateur du launcher (uimodel.Lister).
func osListerUI(dir string) ([]uimodel.Entry, error) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	out := make([]uimodel.Entry, 0, len(ents))
	for _, e := range ents {
		out = append(out, uimodel.Entry{Name: e.Name(), IsDir: e.IsDir()})
	}
	return out, nil
}

// newLauncher construit l'UI ebitenui à partir des profils. initial pré-remplit les
// valeurs du profil sélectionné par défaut (ex. chemin ROM mémorisé en config).
func newLauncher(profiles []machine.MachineProfile, mediaDir string, lister uimodel.Lister, initial machine.Config) *launcher {
	l := &launcher{
		profiles:  profiles,
		mediaDir:  mediaDir,
		lister:    lister,
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
	// Valeurs initiales du profil par défaut + surcharge depuis la config (initial).
	if len(profiles) > 0 {
		l.values = uimodel.InitialValues(profiles[0])
		for k, v := range initial {
			l.values[k] = v
		}
	} else {
		l.values = machine.Config{}
	}

	// Racine plein écran : fond sombre + ancrage centré de la carte.
	l.root = widget.NewContainer(
		widget.ContainerOpts.BackgroundImage(eimage.NewNineSliceColor(colBG)),
		widget.ContainerOpts.Layout(widget.NewAnchorLayout()),
	)
	l.ui = &ebitenui.UI{Container: l.root}
	l.rebuild()
	return l
}

// currentProfile retourne le profil sélectionné (ou un profil vide si la liste est
// vide — cas dégénéré : aucun profil enregistré).
func (l *launcher) currentProfile() machine.MachineProfile {
	if l.selected < 0 || l.selected >= len(l.profiles) {
		return machine.MachineProfile{}
	}
	return l.profiles[l.selected]
}

// takeStart consomme le signal « Démarrer » (une seule fois).
func (l *launcher) takeStart() (startRequest, bool) {
	if !l.start {
		return startRequest{}, false
	}
	l.start = false
	return l.startReq, true
}

// setError affiche un message d'erreur (échec BuildConfig/New) et reste sur le launcher.
func (l *launcher) setError(err error) {
	l.errText = err.Error()
	l.rebuild()
}

// stretchH étire un widget sur toute la largeur dans un RowLayout vertical (l'axe
// transverse). ebitenui n'expose pas d'option Stretch globale sur RowLayout : c'est
// une donnée de placement portée par CHAQUE enfant.
func stretchH() widget.WidgetOpt {
	return widget.WidgetOpts.LayoutData(widget.RowLayoutData{Stretch: true})
}

// card construit le conteneur « carte » centré (panneau sombre, padding, colonne
// verticale ; les enfants pleine largeur portent stretchH()).
func (l *launcher) card() *widget.Container {
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

// rebuild reconstruit l'arbre de widgets selon l'état (vue principale ou navigateur).
// rebuild recrée TOUS les widgets : le focus clavier natif (ebitenui) serait donc perdu
// à chaque interaction. On capture le rang du widget focalisé AVANT destruction et on le
// restaure après (cf. restoreFocus) pour garder le launcher pilotable au clavier.
func (l *launcher) rebuild() {
	prevFocusers := l.root.GetFocusers()
	prevIdx := indexOfFocuser(prevFocusers, l.ui.GetFocusedWidget())
	wasBrowse := l.lastBuildBrowse

	l.root.RemoveChildren()
	l.browseList = nil
	card := l.card()
	browse := l.browseKey != ""
	if browse {
		l.buildBrowser(card)
	} else {
		l.buildMain(card)
	}
	l.root.AddChild(card)
	l.lastBuildBrowse = browse

	l.restoreFocus(browse, wasBrowse, prevIdx)
}

// restoreFocus pose un focus clavier cohérent après rebuild : en mode navigateur, la
// liste (flèches + Enter immédiats) ; en vue principale, le contrôle de même rang qu'avant
// (l'ordre/le nombre de focusables est stable tant qu'on bascule un paramètre), réinitialisé
// au 1ᵉʳ contrôle lors d'un changement de mode (rangs incomparables).
func (l *launcher) restoreFocus(browse, wasBrowse bool, prevIdx int) {
	if browse {
		if l.browseList != nil {
			l.ui.SetFocusedWidget(l.browseList)
		}
		return
	}
	fs := l.root.GetFocusers()
	if len(fs) == 0 {
		return
	}
	idx := prevIdx
	if wasBrowse || idx < 0 {
		idx = 0
	}
	if idx >= len(fs) {
		idx = len(fs) - 1
	}
	l.ui.SetFocusedWidget(fs[idx])
}

// indexOfFocuser retourne le rang de target dans fs (identité de pointeur), ou -1.
func indexOfFocuser(fs []widget.Focuser, target widget.Focuser) int {
	if target == nil {
		return -1
	}
	for i, f := range fs {
		if f == target {
			return i
		}
	}
	return -1
}

// escapePressed traite ÉCHAP : ferme le navigateur de fichiers s'il est ouvert (retour à
// la vue principale) et renvoie true ; sinon false (l'appelant — updateLauncher — quitte
// l'application). ebitenui ne gère pas ÉCHAP nativement.
func (l *launcher) escapePressed() bool {
	if l.browseKey != "" {
		l.browseKey = ""
		l.rebuild()
		return true
	}
	return false
}

// buildMain rend la vue principale : en-tête + sélecteur de machine + paramètres +
// action « Démarrer ».
func (l *launcher) buildMain(card *widget.Container) {
	// En-tête : titre + sous-titre.
	header := widget.NewContainer(
		widget.ContainerOpts.Layout(widget.NewRowLayout(
			widget.RowLayoutOpts.Direction(widget.DirectionVertical),
			widget.RowLayoutOpts.Spacing(2),
		)),
	)
	header.AddChild(widget.NewText(widget.TextOpts.Text("DCMO5", l.faceTitle, colText)))
	header.AddChild(widget.NewText(widget.TextOpts.Text("Émulateur Thomson — choix de la machine", l.faceLabel, colMuted)))
	card.AddChild(header)
	card.AddChild(l.separator())

	// Sélecteur de machine : un bouton par profil (accent si sélectionné).
	card.AddChild(l.sectionLabel("Machine"))
	machines := widget.NewContainer(
		widget.ContainerOpts.Layout(widget.NewRowLayout(
			widget.RowLayoutOpts.Direction(widget.DirectionHorizontal),
			widget.RowLayoutOpts.Spacing(8),
		)),
	)
	for i, p := range l.profiles {
		idx, name := i, p.Name
		img, txt := l.btnImg, l.txtColor
		if i == l.selected {
			img, txt = l.btnSel, l.txtOnSel
		}
		machines.AddChild(l.button(name, img, txt, func() {
			if idx == l.selected {
				return
			}
			l.selected = idx
			l.values = uimodel.InitialValues(l.profiles[idx]) // reset : pas de fuite inter-profils
			l.errText = ""
			l.rebuild()
		}))
	}
	card.AddChild(machines)

	// Paramètres du profil sélectionné, rendus génériquement via uimodel.Describe,
	// dans une grille 2 colonnes « libellé : contrôle » alignée.
	prof := l.currentProfile()
	descs := uimodel.Describe(prof, l.values)
	if len(descs) > 0 {
		card.AddChild(l.separator())
		card.AddChild(l.sectionLabel("Paramètres"))
		grid := widget.NewContainer(
			widget.ContainerOpts.WidgetOpts(stretchH()),
			widget.ContainerOpts.Layout(widget.NewGridLayout(
				widget.GridLayoutOpts.Columns(2),
				widget.GridLayoutOpts.Spacing(16, 10),
				widget.GridLayoutOpts.Stretch([]bool{false, true}, nil),
			)),
		)
		for _, d := range descs {
			l.addParam(grid, d)
		}
		card.AddChild(grid)
		card.AddChild(l.hint("*  paramètre requis"))
	}

	card.AddChild(l.separator())

	if l.errText != "" {
		card.AddChild(widget.NewText(
			widget.TextOpts.Text("⚠  "+l.errText, l.faceLabel, colDanger),
			widget.TextOpts.MaxWidth(cardWidth-56),
		))
	}

	// Action primaire : pleine largeur (étirée), accent.
	card.AddChild(l.primaryButton("Démarrer", func() {
		l.startReq = startRequest{profile: l.currentProfile(), values: cloneConfig(l.values)}
		l.start = true
	}))
}

// addParam ajoute à la grille la paire (libellé, contrôle) d'un descripteur, selon
// son Kind. Le libellé occupe la colonne gauche, le contrôle la colonne droite.
func (l *launcher) addParam(grid *widget.Container, d uimodel.WidgetDescriptor) {
	label := d.Label
	if d.Required {
		label += "  *"
	}
	grid.AddChild(widget.NewText(
		widget.TextOpts.Text(label, l.faceLabel, colMuted),
		widget.TextOpts.Position(widget.TextPositionStart, widget.TextPositionCenter),
	))

	switch d.Kind {
	case machine.ParamFile:
		grid.AddChild(l.fileField(d))
	case machine.ParamBool:
		on, _ := d.Value.(bool)
		img, txt := l.btnImg, l.txtColor
		if on {
			img, txt = l.btnSel, l.txtOnSel
		}
		grid.AddChild(l.button(boolLabel(on), img, txt, func() {
			l.values[d.Key] = !on
			l.rebuild()
		}))
	case machine.ParamEnum:
		grid.AddChild(l.button(enumLabel(d)+"   »", l.btnImg, l.txtColor, func() {
			l.values[d.Key] = nextEnum(d)
			l.rebuild()
		}))
	case machine.ParamInt:
		cur, _ := d.Value.(int)
		ctrl := widget.NewContainer(
			widget.ContainerOpts.Layout(widget.NewRowLayout(
				widget.RowLayoutOpts.Direction(widget.DirectionHorizontal),
				widget.RowLayoutOpts.Spacing(8),
			)),
		)
		ctrl.AddChild(l.squareButton("−", func() { l.values[d.Key] = cur - 1; l.rebuild() }))
		ctrl.AddChild(widget.NewText(
			widget.TextOpts.Text(fmt.Sprintf("%d", cur), l.faceBtn, colText),
			widget.TextOpts.Position(widget.TextPositionCenter, widget.TextPositionCenter),
		))
		ctrl.AddChild(l.squareButton("+", func() { l.values[d.Key] = cur + 1; l.rebuild() }))
		grid.AddChild(ctrl)
	}
}

// buildBrowser rend le navigateur de fichiers pour le Param File en cours (browseKey),
// alimenté par uimodel.ListDir (logique pure, testée en CI).
func (l *launcher) buildBrowser(card *widget.Container) {
	card.AddChild(widget.NewText(widget.TextOpts.Text("Choisir un fichier", l.faceTitle, colText)))
	// Chemin courant tronqué par la gauche (« …/dossier ») : un chemin sans espace ne
	// se coupe pas tout seul et déborderait sinon de la carte.
	card.AddChild(widget.NewText(widget.TextOpts.Text(shortenPath(l.browseDir, maxPathRunes), l.faceLabel, colMuted)))
	card.AddChild(l.separator())
	card.AddChild(l.button("« Annuler", l.btnImg, l.txtColor, func() {
		l.browseKey = ""
		l.rebuild()
	}))

	entries := uimodel.ListDir(l.lister, l.browseDir, l.browseExt)
	card.AddChild(l.fileList(entries))
}

// fileList rend les entrées du répertoire courant dans un widget List ebitenui. Le
// widget gère NATIVEMENT le défilement (molette + ascenseur) : on lui délègue tout le
// scroll au lieu de recâbler un slider à la main — l'ancien layout maison ne câblait
// PAS la molette (finding revue Codex).
//
// La hauteur reste BORNÉE par fixedHeightLayout : un List placé dans le RowLayout
// vertical de la carte réclamerait sinon la hauteur de TOUT le contenu
// (ScrollContainer.PreferredSize remonte la taille du contenu, pas celle du viewport).
// Au-delà de browserListMaxPx, le List clippe et défile.
func (l *launcher) fileList(entries []uimodel.Entry) *widget.Container {
	items := make([]any, len(entries))
	for i, e := range entries {
		items[i] = e
	}

	list := widget.NewList(
		widget.ListOpts.Entries(items),
		widget.ListOpts.EntryFontFace(l.faceBtn),
		// Dossiers distingués par un suffixe « / » (gofont n'a pas de glyphe dossier ; le
		// List applique un style uniforme — plus d'accent par entrée comme avant).
		widget.ListOpts.EntryLabelFunc(func(e any) string {
			ent := e.(uimodel.Entry)
			if ent.IsDir {
				return ent.Name + "/"
			}
			return ent.Name
		}),
		widget.ListOpts.EntryColor(&widget.ListEntryColor{
			Unselected:                 colText,
			Selected:                   colWhite,
			DisabledUnselected:         colMuted,
			DisabledSelected:           colMuted,
			SelectingBackground:        colAccent,
			SelectedBackground:         colAccent,
			FocusedBackground:          colAccentLo, // entrée focalisée clavier : bleu net (navigation aux flèches)
			SelectingFocusedBackground: colAccent,
			SelectedFocusedBackground:  colAccent,
			DisabledSelectedBackground: colBtnLo,
		}),
		widget.ListOpts.EntryTextPosition(widget.TextPositionStart, widget.TextPositionCenter),
		widget.ListOpts.EntryTextPadding(&widget.Insets{Top: 8, Bottom: 8, Left: 14, Right: 14}),
		widget.ListOpts.ScrollContainerImage(&widget.ScrollContainerImage{
			Idle: eimage.NewNineSliceColor(colField),
			Mask: eimage.NewNineSliceColor(colWhite),
		}),
		widget.ListOpts.SliderParams(&widget.SliderParams{
			TrackImage: &widget.SliderTrackImage{
				Idle:  eimage.NewNineSliceColor(colBtnLo),
				Hover: eimage.NewNineSliceColor(colBtnLo),
			},
			HandleImage: &widget.ButtonImage{
				Idle:    eimage.NewNineSliceColor(colBtn),
				Hover:   eimage.NewNineSliceColor(colBtnHi),
				Pressed: eimage.NewNineSliceColor(colAccent),
			},
		}),
		widget.ListOpts.HideHorizontalSlider(),
		widget.ListOpts.EntrySelectedHandler(func(args *widget.ListEntrySelectedEventArgs) {
			ent := args.Entry.(uimodel.Entry)
			target := filepath.Join(l.browseDir, ent.Name)
			if ent.IsDir {
				l.browseDir = filepath.Clean(target)
				l.rebuild()
				return
			}
			l.values[l.browseKey] = target
			l.mediaDir = filepath.Dir(target)
			l.browseKey = ""
			l.rebuild()
		}),
	)
	l.browseList = list // cible du focus clavier (cf. restoreFocus)

	// Hauteur bornée (cf. en-tête) : au moins une entrée, au plus browserListMaxPx.
	h := len(entries) * browserItemPx
	if h > browserListMaxPx {
		h = browserListMaxPx
	}
	if h < browserItemPx {
		h = browserItemPx
	}
	viewport := widget.NewContainer(
		widget.ContainerOpts.WidgetOpts(stretchH()),
		widget.ContainerOpts.Layout(fixedHeightLayout{h: h}),
	)
	viewport.AddChild(list)
	return viewport
}

// ── Helpers de rendu et de libellé ─────────────────────────────────────────────

// separator : fine ligne horizontale (1px) remplissant la largeur de la carte.
func (l *launcher) separator() *widget.Container {
	return widget.NewContainer(
		widget.ContainerOpts.BackgroundImage(eimage.NewNineSliceColor(colBorder)),
		widget.ContainerOpts.WidgetOpts(stretchH(), widget.WidgetOpts.MinSize(0, 1)),
	)
}

// sectionLabel : intitulé de section discret (gris).
func (l *launcher) sectionLabel(s string) *widget.Text {
	return widget.NewText(widget.TextOpts.Text(s, l.faceLabel, colMuted))
}

// hint : note d'aide discrète sous un groupe.
func (l *launcher) hint(s string) *widget.Text {
	return widget.NewText(widget.TextOpts.Text(s, l.faceLabel, colMuted))
}

// button : bouton standard (image + couleur de texte fournies), hauteur stable.
func (l *launcher) button(label string, img *widget.ButtonImage, txt *widget.ButtonTextColor, onClick func()) *widget.Button {
	return widget.NewButton(
		widget.ButtonOpts.Image(img),
		widget.ButtonOpts.Text(label, l.faceBtn, txt),
		widget.ButtonOpts.TextPadding(&widget.Insets{Top: 8, Bottom: 8, Left: 14, Right: 14}),
		widget.ButtonOpts.WidgetOpts(widget.WidgetOpts.MinSize(0, 34)),
		widget.ButtonOpts.ClickedHandler(func(*widget.ButtonClickedEventArgs) { onClick() }),
	)
}

// squareButton : petit bouton carré (incréments Int).
func (l *launcher) squareButton(label string, onClick func()) *widget.Button {
	return widget.NewButton(
		widget.ButtonOpts.Image(l.btnImg),
		widget.ButtonOpts.Text(label, l.faceBtn, l.txtColor),
		widget.ButtonOpts.WidgetOpts(widget.WidgetOpts.MinSize(34, 34)),
		widget.ButtonOpts.ClickedHandler(func(*widget.ButtonClickedEventArgs) { onClick() }),
	)
}

// primaryButton : action principale en accent, pleine largeur, plus haute.
func (l *launcher) primaryButton(label string, onClick func()) *widget.Button {
	return widget.NewButton(
		widget.ButtonOpts.Image(l.btnSel),
		widget.ButtonOpts.Text(label, l.faceBtn, l.txtOnSel),
		widget.ButtonOpts.TextPadding(&widget.Insets{Top: 11, Bottom: 11, Left: 14, Right: 14}),
		widget.ButtonOpts.WidgetOpts(stretchH(), widget.WidgetOpts.MinSize(0, 42)),
		widget.ButtonOpts.ClickedHandler(func(*widget.ButtonClickedEventArgs) { onClick() }),
	)
}

// fileField rend un paramètre fichier comme un CHAMP : nom de base à gauche (ou
// « Aucun fichier » en gris si vide), chevron « » » à droite pour ouvrir le
// navigateur, et croix « × » pour effacer si un fichier est posé. Toute la zone du
// nom est cliquable (= parcourir). Remplace l'ancien bouton « (parcourir…) » centré.
func (l *launcher) fileField(d uimodel.WidgetDescriptor) *widget.Container {
	s, _ := d.Value.(string)
	name, nameCol := "Aucun fichier", colMuted
	if s != "" {
		name, nameCol = ellipsizeName(filepath.Base(s), maxFileNameRunes), colText
	}
	browse := func() {
		l.browseKey = d.Key
		l.browseExt = append([]string(nil), d.FileExt...)
		l.browseDir = l.mediaDir
		l.rebuild()
	}

	field := widget.NewContainer(
		widget.ContainerOpts.BackgroundImage(eimage.NewNineSliceColor(colField)),
		widget.ContainerOpts.WidgetOpts(widget.WidgetOpts.MinSize(0, 34)),
		widget.ContainerOpts.Layout(widget.NewGridLayout(
			widget.GridLayoutOpts.Columns(2),
			widget.GridLayoutOpts.Stretch([]bool{true, false}, []bool{true}),
			widget.GridLayoutOpts.Padding(&widget.Insets{Left: 12, Right: 6}),
			widget.GridLayoutOpts.Spacing(4, 0),
		)),
	)
	// Colonne 1 (étirée) : nom du fichier, bouton plat aligné à gauche, clic = parcourir.
	field.AddChild(widget.NewButton(
		widget.ButtonOpts.Image(l.fieldImg),
		widget.ButtonOpts.Text(name, l.faceBtn, &widget.ButtonTextColor{Idle: nameCol, Hover: colWhite, Pressed: nameCol}),
		widget.ButtonOpts.TextPosition(widget.TextPositionStart, widget.TextPositionCenter),
		widget.ButtonOpts.TextPadding(&widget.Insets{Right: 8, Top: 6, Bottom: 6}),
		widget.ButtonOpts.WidgetOpts(widget.WidgetOpts.MinSize(0, 34)),
		widget.ButtonOpts.ClickedHandler(func(*widget.ButtonClickedEventArgs) { browse() }),
	))
	// Colonne 2 : actions à droite — « × » (effacer, si fichier) puis « » » (parcourir).
	actions := widget.NewContainer(
		widget.ContainerOpts.Layout(widget.NewRowLayout(
			widget.RowLayoutOpts.Direction(widget.DirectionHorizontal),
			widget.RowLayoutOpts.Spacing(2),
		)),
	)
	if s != "" {
		actions.AddChild(l.glyphButton("×", colMuted, func() {
			delete(l.values, d.Key)
			l.rebuild()
		}))
	}
	actions.AddChild(l.glyphButton("»", colAccent, browse))
	field.AddChild(actions)
	return field
}

// glyphButton : petit bouton « plat » (fond de champ) portant un glyphe (× ou »).
func (l *launcher) glyphButton(glyph string, col color.Color, onClick func()) *widget.Button {
	return widget.NewButton(
		widget.ButtonOpts.Image(l.fieldImg),
		widget.ButtonOpts.Text(glyph, l.faceBtn, &widget.ButtonTextColor{Idle: col, Hover: colWhite, Pressed: col}),
		widget.ButtonOpts.WidgetOpts(widget.WidgetOpts.MinSize(28, 34)),
		widget.ButtonOpts.ClickedHandler(func(*widget.ButtonClickedEventArgs) { onClick() }),
	)
}

// ellipsizeName tronque un nom de fichier trop long en préservant le DÉBUT et la FIN
// (donc l'extension) : « longnomdefichi…age.rom ».
func ellipsizeName(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	const tail = 8
	head := max - 1 - tail
	if head < 1 {
		head = 1
	}
	return string(r[:head]) + "…" + string(r[len(r)-tail:])
}

// shortenPath raccourcit un chemin trop long par la GAUCHE, en coupant sur les
// séparateurs : « …/parent/dossier ». Garantit l'absence de débordement hors carte.
func shortenPath(p string, max int) string {
	if len([]rune(p)) <= max {
		return p
	}
	sep := string(os.PathSeparator)
	parts := strings.Split(p, sep)
	tail := parts[len(parts)-1]
	for i := len(parts) - 2; i >= 0; i-- {
		cand := parts[i] + sep + tail
		if len([]rune("…"+sep+cand)) > max {
			break
		}
		tail = cand
	}
	return "…" + sep + tail
}

func boolLabel(on bool) string {
	if on {
		return "Oui"
	}
	return "Non"
}

// enumLabel affiche le libellé de l'Option dont la Value == valeur courante, sinon
// la valeur brute.
func enumLabel(d uimodel.WidgetDescriptor) string {
	for _, o := range d.Options {
		if o.Value == d.Value {
			return o.Label
		}
	}
	return fmt.Sprintf("%v", d.Value)
}

// nextEnum retourne la Value de l'Option suivante (cyclique) après la valeur courante.
func nextEnum(d uimodel.WidgetDescriptor) any {
	if len(d.Options) == 0 {
		return d.Value
	}
	for i, o := range d.Options {
		if o.Value == d.Value {
			return d.Options[(i+1)%len(d.Options)].Value
		}
	}
	return d.Options[0].Value
}

// cloneConfig copie une Config (la transmission à l'App ne doit pas partager la map
// que le launcher continue de muter si l'instanciation échoue).
func cloneConfig(c machine.Config) machine.Config {
	out := machine.Config{}
	for k, v := range c {
		out[k] = v
	}
	return out
}

// fixedHeightLayout est un layout ebitenui minimal qui annonce une hauteur FIXE
// (indépendante du contenu) et place ses enfants dans l'intégralité du rect reçu.
// Il sert à borner la hauteur du widget List du navigateur de fichiers : sans lui, la
// taille préférée remonterait celle de tout le contenu (cf. fileList).
//
// La largeur préférée DOIT être > 0 : RowLayout.PreferredSize unionne les rects des
// enfants, et image.Rectangle.Union ignore tout rect de largeur (ou hauteur) nulle
// (réputé « vide »). Renvoyer une largeur 0 faisait donc disparaître la liste du calcul
// de hauteur de la carte → la carte ne réservait pas les f.h pixels et la liste
// débordait sous le panneau. On remonte donc la largeur préférée du contenu (la largeur
// effective reste pilotée par l'étirement du parent via stretchH()).
type fixedHeightLayout struct{ h int }

func (f fixedHeightLayout) PreferredSize(widgets []widget.PreferredSizeLocateableWidget) (int, int) {
	w := 0
	for _, wd := range widgets {
		if cw, _ := wd.PreferredSize(); cw > w {
			w = cw
		}
	}
	if w < 1 {
		w = 1 // garde-fou : jamais un rect « vide » ignoré par Union (cf. ci-dessus)
	}
	return w, f.h
}

func (f fixedHeightLayout) Layout(widgets []widget.PreferredSizeLocateableWidget, rect image.Rectangle) {
	for _, w := range widgets {
		w.SetLocation(rect)
	}
}
