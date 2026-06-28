// overlay_ui.go — arbre ebitenui de l'overlay Échap (lot #117 Inc 3b, RENDU IMPUR,
// hors CI → validation visuelle owner). Calqué sur launcher.go : racine plein écran
// (AnchorLayout) TRANSPARENTE — le framebuffer gelé + le voile sont dessinés AVANT par
// App.drawOverlay, la carte flotte par-dessus — reconstruite (rebuild) à chaque
// changement d'état. La logique de décision reste PURE (overlay.Model, DescribeLive,
// LiveMediaOps) ; ici on ne fait que CÂBLER des widgets.
//
// Discipline ebitenui (cf. launcher) : uniquement Boutons/Text/List — pas de TextInput
// ni Checkbox (qui paniquent sans thème complet). Glyphes limités à gofont (×, », «, ⚠).
//
// 3b.4c : vues Main (médias éditables + actions système) et Browse (navigateur de
// fichiers). L'édition se fait dans une config de travail `next` (clone de l'état monté à
// l'ouverture) ; « Appliquer et reprendre » la confronte à l'état réel via
// uimodel.LiveMediaOps et l'App exécute les montages/éjections. Échap ANNULE (rejette next).
package app

import (
	"path/filepath"

	"github.com/ebitenui/ebitenui"
	eimage "github.com/ebitenui/ebitenui/image"
	"github.com/ebitenui/ebitenui/widget"

	"github.com/Lesur-ai/dcmo5/internal/machine"
	"github.com/Lesur-ai/dcmo5/internal/overlay"
	"github.com/Lesur-ai/dcmo5/internal/uimodel"
)

// overlayCardWidth : largeur de la carte de l'overlay. Plus étroite que celle du launcher
// (cardWidth=600, fenêtre 760×640) car l'overlay se superpose à la fenêtre ÉMULATEUR, plus
// petite (672×432 pour le MO5). Avec padding/espacements réduits (overlayCard), la carte
// tient avec des marges au lieu de déborder l'écran.
const overlayCardWidth = 540

// overlayBrowserListMaxPx : hauteur max de la liste du navigateur DANS l'overlay. Bien plus
// basse que celle du launcher (browserListMaxPx=360, fenêtre 760×640) : avec l'en-tête, le
// chemin, le bouton Annuler et le padding, la carte Browse doit tenir dans la fenêtre
// émulateur 672×432 (au-delà, défilement de la liste). ~5-6 entrées visibles, le reste défile.
const overlayBrowserListMaxPx = 220

// overlayUI porte l'arbre ebitenui de l'overlay et les signaux d'action. Embarque
// *uiKit (polices/images/couleurs partagées avec le launcher) par promotion de champ.
type overlayUI struct {
	ui    *ebitenui.UI
	root  *widget.Container
	model *overlay.Model // état (Main/Browse) ; partagé avec App.overlay (PUR, sans Host)

	profile  machine.MachineProfile   // schéma consommé par DescribeLive
	profiles []machine.MachineProfile // machines enregistrées (cible du bouton « Changer de machine »)
	lister   uimodel.Lister           // listage de répertoire (vue Browse)
	mediaDir string                   // répertoire de départ du navigateur

	// next : config de travail, clone de l'état RÉELLEMENT monté (App.CurrentConfig) au
	// moment de l'ouverture, éditée dans l'overlay (sélection / effacement de média). Elle
	// est confrontée à l'état réel par App.applyLiveOps (uimodel.LiveMediaOps) à l'application.
	next machine.Config

	// Navigateur de fichiers (vue Browse) : la clé éditée vient de model.BrowseKey().
	browseDir  string
	browseExt  []string
	browseList widget.Focuser // cible du focus clavier en vue Browse (cf. restoreFocus)

	// lastBuildBrowse : mode du dernier rebuild, pour décider du focus à restaurer.
	lastBuildBrowse bool

	errText string // erreurs d'application média (#5), affichées en colDanger

	// Signaux one-shot, lus puis remis à zéro par App.updateOverlay (pattern takeStart du
	// launcher) : découple totalement overlayUI du Host (aucun import emu ici).
	apply    bool
	reset    bool
	initprog bool
	quit     bool

	// Changement de machine (Inc 5) : la vue ConfirmSwitch arme switchTarget+switchArmed ;
	// App.updateOverlay consomme via takeSwitch et exécute le switch.
	switchTarget machine.MachineProfile
	switchArmed  bool

	*uiKit
}

// newOverlayUI crée l'arbre (racine transparente). model est l'état partagé avec l'App
// (overlay.Model, pur) ; lister sert au navigateur de fichiers ; profiles est la liste des
// machines enregistrées (pour le bouton « Changer de machine »).
func newOverlayUI(profile machine.MachineProfile, profiles []machine.MachineProfile, model *overlay.Model, lister uimodel.Lister, kit *uiKit) *overlayUI {
	o := &overlayUI{profile: profile, profiles: profiles, model: model, lister: lister, uiKit: kit}
	// Racine TRANSPARENTE (pas de BackgroundImage) : le framebuffer gelé + le voile
	// dessinés par App.drawOverlay restent visibles autour de la carte centrée.
	o.root = widget.NewContainer(
		widget.ContainerOpts.Layout(widget.NewAnchorLayout()),
	)
	o.ui = &ebitenui.UI{Container: o.root}
	return o
}

// open (ré)initialise la vue à l'ouverture : profil + config de travail = clone de l'état
// RÉELLEMENT monté (cur), répertoire média, efface les erreurs, reconstruit.
func (o *overlayUI) open(profile machine.MachineProfile, mediaDir string, cur machine.Config) {
	o.profile = profile
	o.mediaDir = mediaDir
	o.next = cloneConfig(cur)
	o.errText = ""
	o.rebuild()
}

// takeApply/takeReset/takeInitprog consomment un signal one-shot (lu une fois).
func (o *overlayUI) takeApply() bool    { v := o.apply; o.apply = false; return v }
func (o *overlayUI) takeReset() bool    { v := o.reset; o.reset = false; return v }
func (o *overlayUI) takeInitprog() bool { v := o.initprog; o.initprog = false; return v }

// takeSwitch consomme la demande de changement de machine (cible + drapeau, lus une fois).
func (o *overlayUI) takeSwitch() (machine.MachineProfile, bool) {
	if !o.switchArmed {
		return machine.MachineProfile{}, false
	}
	o.switchArmed = false
	return o.switchTarget, true
}

// overlayCard : variante COMPACTE de uiKit.card(), dimensionnée pour la fenêtre émulateur
// (≠ launcher). Padding et espacements réduits pour que la carte tienne avec des marges
// dans une fenêtre 672×432 (MO5) sans déborder. uiKit.card() reste inchangée (launcher).
func (o *overlayUI) overlayCard() *widget.Container {
	return widget.NewContainer(
		widget.ContainerOpts.BackgroundImage(eimage.NewNineSliceColor(colPanel)),
		widget.ContainerOpts.WidgetOpts(
			widget.WidgetOpts.MinSize(overlayCardWidth, 0),
			widget.WidgetOpts.LayoutData(widget.AnchorLayoutData{
				HorizontalPosition: widget.AnchorLayoutPositionCenter,
				VerticalPosition:   widget.AnchorLayoutPositionCenter,
			}),
		),
		widget.ContainerOpts.Layout(widget.NewRowLayout(
			widget.RowLayoutOpts.Direction(widget.DirectionVertical),
			widget.RowLayoutOpts.Padding(&widget.Insets{Top: 16, Bottom: 16, Left: 20, Right: 20}),
			widget.RowLayoutOpts.Spacing(9),
		)),
	)
}

// rebuild reconstruit l'arbre selon l'état (model.State()). Recrée TOUS les widgets : le
// focus clavier natif serait perdu → on capture le rang focalisé avant destruction et on le
// restaure après (restoreFocus), comme le launcher. ConfirmSwitch (Inc 5) retombe sur Main.
func (o *overlayUI) rebuild() {
	prevIdx := indexOfFocuser(o.root.GetFocusers(), o.ui.GetFocusedWidget())
	wasBrowse := o.lastBuildBrowse

	o.root.RemoveChildren()
	o.browseList = nil
	card := o.overlayCard()
	browse := o.model.State() == overlay.StateBrowse
	switch o.model.State() {
	case overlay.StateBrowse:
		o.buildBrowser(card)
	case overlay.StateConfirmSwitch:
		o.buildConfirmSwitch(card)
	default:
		o.buildMain(card)
	}
	o.root.AddChild(card)
	o.lastBuildBrowse = browse
	o.restoreFocus(browse, wasBrowse, prevIdx)
}

// restoreFocus pose un focus clavier cohérent après rebuild (cf. launcher.restoreFocus) :
// en Browse, la liste (flèches + Entrée immédiats) ; en Main, le contrôle de même rang
// qu'avant, réinitialisé au 1er au changement de vue.
func (o *overlayUI) restoreFocus(browse, wasBrowse bool, prevIdx int) {
	if browse {
		if o.browseList != nil {
			o.ui.SetFocusedWidget(o.browseList)
		}
		return
	}
	fs := o.root.GetFocusers()
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
	o.ui.SetFocusedWidget(fs[idx])
}

// buildMain rend la vue principale : en-tête, médias ÉDITABLES (champ + » parcourir + ×
// effacer, sur la config de travail next), actions système, et « Appliquer et reprendre ».
func (o *overlayUI) buildMain(card *widget.Container) {
	header := widget.NewContainer(
		widget.ContainerOpts.Layout(widget.NewRowLayout(
			widget.RowLayoutOpts.Direction(widget.DirectionVertical),
			widget.RowLayoutOpts.Spacing(2),
		)),
	)
	header.AddChild(widget.NewText(widget.TextOpts.Text("DCMO5 — Pilotage", o.faceTitle, colText)))
	header.AddChild(widget.NewText(widget.TextOpts.Text("Émulation en pause — Échap pour annuler", o.faceLabel, colMuted)))
	card.AddChild(header)
	card.AddChild(o.separator())

	card.AddChild(o.sectionLabel("Médias"))
	descs := uimodel.DescribeLive(o.profile, o.next)
	if len(descs) == 0 {
		card.AddChild(o.hint("Aucun média configurable"))
	} else {
		grid := widget.NewContainer(
			widget.ContainerOpts.WidgetOpts(stretchH()),
			widget.ContainerOpts.Layout(widget.NewGridLayout(
				widget.GridLayoutOpts.Columns(2),
				widget.GridLayoutOpts.Spacing(16, 6),
				widget.GridLayoutOpts.Stretch([]bool{false, true}, nil),
			)),
		)
		for _, d := range descs {
			if d.Kind != machine.ParamFile {
				continue // section Médias : on ne rend en champ fichier QUE les Params File
			}
			grid.AddChild(widget.NewText(
				widget.TextOpts.Text(d.Label, o.faceLabel, colMuted),
				widget.TextOpts.Position(widget.TextPositionStart, widget.TextPositionCenter),
			))
			grid.AddChild(o.mediaField(d))
		}
		card.AddChild(grid)
	}

	card.AddChild(o.separator())
	card.AddChild(o.sectionLabel("Système"))
	// Boutons système EN LIGNE (libellés courts) : compact pour tenir dans la fenêtre.
	sys := widget.NewContainer(
		widget.ContainerOpts.Layout(widget.NewRowLayout(
			widget.RowLayoutOpts.Direction(widget.DirectionHorizontal),
			widget.RowLayoutOpts.Spacing(8),
		)),
	)
	sys.AddChild(o.button("Reset", o.btnImg, o.txtColor, func() { o.reset = true }))
	sys.AddChild(o.button("Init prog", o.btnImg, o.txtColor, func() { o.initprog = true }))
	sys.AddChild(o.button("Quitter", o.btnImg, o.txtColor, func() { o.quit = true }))
	// Changement de machine : DANS la même rangée (compact — une section séparée ferait
	// déborder la carte au-delà de la fenêtre 672×432). Affiché seulement s'il existe une
	// AUTRE machine (overlay.NextProfile, pur). → vue de confirmation (ConfirmSwitch).
	if _, ok := overlay.NextProfile(o.profiles, o.profile.ID); ok {
		sys.AddChild(o.button("Changer machine", o.btnImg, o.txtColor, func() {
			o.model.GoConfirmSwitch()
			o.rebuild()
		}))
	}
	card.AddChild(sys)

	if o.errText != "" {
		card.AddChild(widget.NewText(
			widget.TextOpts.Text("⚠  "+o.errText, o.faceLabel, colDanger),
			widget.TextOpts.MaxWidth(overlayCardWidth-40),
		))
	}

	card.AddChild(o.separator())
	// Action primaire : applique les changements média de next (montage/éjection) puis
	// reprend l'émulation. Aucun changement → aucune op (LiveMediaOps), simple reprise.
	card.AddChild(o.primaryButton("Appliquer et reprendre", func() { o.apply = true }))
}

// buildConfirmSwitch rend la vue de confirmation du changement de machine : la cible
// (NextProfile, pur), un avertissement (les médias seront éjectés), puis Confirmer (arme
// le signal switch) / Annuler (retour Main). Si aucune cible (ne devrait pas arriver depuis
// le bouton), on retombe sur Main.
func (o *overlayUI) buildConfirmSwitch(card *widget.Container) {
	target, ok := overlay.NextProfile(o.profiles, o.profile.ID)
	if !ok {
		o.model.GoMain()
		o.buildMain(card)
		return
	}
	header := widget.NewContainer(
		widget.ContainerOpts.Layout(widget.NewRowLayout(
			widget.RowLayoutOpts.Direction(widget.DirectionVertical),
			widget.RowLayoutOpts.Spacing(2),
		)),
	)
	header.AddChild(widget.NewText(widget.TextOpts.Text("Changer de machine", o.faceTitle, colText)))
	header.AddChild(widget.NewText(widget.TextOpts.Text("Passer de "+o.profile.Name+" à "+target.Name, o.faceLabel, colMuted)))
	card.AddChild(header)
	card.AddChild(o.separator())
	card.AddChild(widget.NewText(
		widget.TextOpts.Text("La machine courante sera réinitialisée et les médias montés éjectés.", o.faceLabel, colMuted),
		widget.TextOpts.MaxWidth(overlayCardWidth-40),
	))
	if o.errText != "" {
		card.AddChild(widget.NewText(
			widget.TextOpts.Text("⚠  "+o.errText, o.faceLabel, colDanger),
			widget.TextOpts.MaxWidth(overlayCardWidth-40),
		))
	}
	card.AddChild(o.separator())
	actions := widget.NewContainer(
		widget.ContainerOpts.Layout(widget.NewRowLayout(
			widget.RowLayoutOpts.Direction(widget.DirectionHorizontal),
			widget.RowLayoutOpts.Spacing(8),
		)),
	)
	actions.AddChild(o.button("« Annuler", o.btnImg, o.txtColor, func() {
		o.model.GoMain()
		o.rebuild()
	}))
	card.AddChild(actions)
	// Action primaire : confirmer la bascule vers la cible.
	card.AddChild(o.primaryButton("Passer à "+target.Name, func() {
		o.switchTarget = target
		o.switchArmed = true
	}))
}

// mediaField rend un Param File média comme un champ éditable (calqué sur launcher.fileField,
// mais sur la config de travail next) : nom du fichier choisi à gauche (clic = parcourir),
// « × » pour effacer (= éjecter à l'application), « » » pour ouvrir le navigateur.
func (o *overlayUI) mediaField(d uimodel.WidgetDescriptor) *widget.Container {
	s, _ := d.Value.(string)
	name, nameCol := "Aucun fichier", colMuted
	if s != "" && s != "." {
		name, nameCol = ellipsizeName(filepath.Base(s), maxFileNameRunes), colText
	}
	browse := func() {
		o.browseExt = append([]string(nil), d.FileExt...)
		o.browseDir = o.mediaDir
		o.model.GoBrowse(d.Key)
		o.rebuild()
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
	field.AddChild(widget.NewButton(
		widget.ButtonOpts.Image(o.fieldImg),
		widget.ButtonOpts.Text(name, o.faceBtn, &widget.ButtonTextColor{Idle: nameCol, Hover: colWhite, Pressed: nameCol}),
		widget.ButtonOpts.TextPosition(widget.TextPositionStart, widget.TextPositionCenter),
		widget.ButtonOpts.TextPadding(&widget.Insets{Right: 8, Top: 6, Bottom: 6}),
		widget.ButtonOpts.WidgetOpts(widget.WidgetOpts.MinSize(0, 34)),
		widget.ButtonOpts.ClickedHandler(func(*widget.ButtonClickedEventArgs) { browse() }),
	))
	actions := widget.NewContainer(
		widget.ContainerOpts.Layout(widget.NewRowLayout(
			widget.RowLayoutOpts.Direction(widget.DirectionHorizontal),
			widget.RowLayoutOpts.Spacing(2),
		)),
	)
	if s != "" && s != "." {
		actions.AddChild(o.glyphButton("×", colMuted, func() {
			o.next[d.Key] = "" // "" = éjection à l'application (cf. LiveMediaOps)
			o.rebuild()
		}))
	}
	actions.AddChild(o.glyphButton("»", colAccent, browse))
	field.AddChild(actions)
	return field
}

// buildBrowser rend le navigateur de fichiers (vue Browse) pour la clé en cours
// (model.BrowseKey()), via la brique partagée uiKit.fileList (logique pure ListDir, testée CI).
func (o *overlayUI) buildBrowser(card *widget.Container) {
	card.AddChild(widget.NewText(widget.TextOpts.Text("Choisir un fichier", o.faceTitle, colText)))
	card.AddChild(widget.NewText(widget.TextOpts.Text(shortenPath(o.browseDir, maxPathRunes), o.faceLabel, colMuted)))
	card.AddChild(o.separator())
	card.AddChild(o.button("« Annuler", o.btnImg, o.txtColor, func() {
		o.model.GoMain()
		o.rebuild()
	}))

	entries := uimodel.ListDir(o.lister, o.browseDir, o.browseExt)
	viewport, focuser := o.uiKit.fileList(entries, overlayBrowserListMaxPx, func(ent uimodel.Entry) {
		target := filepath.Join(o.browseDir, ent.Name)
		if ent.IsDir {
			o.browseDir = filepath.Clean(target)
			o.rebuild()
			return
		}
		o.next[o.model.BrowseKey()] = target // sélection : enregistrée dans la config de travail
		o.mediaDir = filepath.Dir(target)
		o.model.GoMain()
		o.rebuild()
	})
	o.browseList = focuser
	card.AddChild(viewport)
}
