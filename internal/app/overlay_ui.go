// overlay_ui.go — arbre ebitenui de l'overlay Échap (lot #117 Inc 3b, RENDU IMPUR,
// hors CI → validation visuelle owner). Calqué sur launcher.go : racine plein écran
// (AnchorLayout) TRANSPARENTE — le framebuffer gelé + le voile sont dessinés AVANT par
// App.drawOverlay, la carte flotte par-dessus — reconstruite (rebuild) à chaque
// changement d'état/ouverture. La logique de décision reste PURE (overlay.Model,
// uimodel.DescribeLive/LiveMediaOps) ; ici on ne fait que CÂBLER des widgets.
//
// Discipline ebitenui (cf. launcher) : uniquement Boutons/Text/List — pas de TextInput
// ni Checkbox (qui exigent un thème complet et paniquent). Glyphes limités à ceux que
// gofont couvre (×, », «, ⚠) ; pas d'emoji.
//
// 3b.2 (cet incrément) : vue Main = actions système (boutons) + liste des médias montés
// EN LECTURE. Le navigateur (Browse) et l'application des changements média (montage/
// éjection via LiveMediaOps) arrivent en 3b.3.
package app

import (
	"path/filepath"

	"github.com/ebitenui/ebitenui"
	"github.com/ebitenui/ebitenui/widget"

	"github.com/Lesur-ai/dcmo5/internal/machine"
	"github.com/Lesur-ai/dcmo5/internal/overlay"
	"github.com/Lesur-ai/dcmo5/internal/uimodel"
)

// overlayUI porte l'arbre ebitenui de l'overlay et les signaux d'action. Embarque
// *uiKit (polices/images/couleurs partagées avec le launcher) par promotion de champ.
type overlayUI struct {
	ui      *ebitenui.UI
	root    *widget.Container
	profile machine.MachineProfile // schéma consommé par DescribeLive
	live    machine.Config         // état média RÉELLEMENT monté (base d'affichage) ; rafraîchi à l'ouverture
	state   overlay.State          // miroir de overlay.Model.State() pour le rebuild
	errText string                 // erreurs (application média en 3b.3) ; affichées en colDanger

	// Signaux one-shot, lus puis remis à zéro par App.updateOverlay (pattern takeStart du
	// launcher) : découple totalement overlayUI du Host (aucun import emu ici).
	resume   bool
	reset    bool
	initprog bool
	quit     bool

	*uiKit
}

// newOverlayUI crée l'arbre (racine transparente). Le profil fige le schéma des médias
// affichés ; il est rafraîchi à chaque ouverture (open) pour rester cohérent avec la
// machine attachée.
func newOverlayUI(profile machine.MachineProfile, kit *uiKit) *overlayUI {
	o := &overlayUI{profile: profile, uiKit: kit}
	// Racine TRANSPARENTE (pas de BackgroundImage) : le framebuffer gelé + le voile
	// dessinés par App.drawOverlay restent visibles autour de la carte centrée.
	o.root = widget.NewContainer(
		widget.ContainerOpts.Layout(widget.NewAnchorLayout()),
	)
	o.ui = &ebitenui.UI{Container: o.root}
	return o
}

// open (ré)initialise la vue à l'ouverture de l'overlay : profil + état média courant
// (source de vérité dérivée, cf. App.CurrentConfig), efface les erreurs, reconstruit.
func (o *overlayUI) open(profile machine.MachineProfile, state overlay.State, live machine.Config) {
	o.profile = profile
	o.state = state
	o.live = live
	o.errText = ""
	o.rebuild()
}

// sync reflète un changement d'état (Back/GoMain/GoBrowse) puis reconstruit.
func (o *overlayUI) sync(state overlay.State) {
	o.state = state
	o.rebuild()
}

// takeResume/takeReset/takeInitprog consomment un signal one-shot (lu une fois).
func (o *overlayUI) takeResume() bool   { v := o.resume; o.resume = false; return v }
func (o *overlayUI) takeReset() bool    { v := o.reset; o.reset = false; return v }
func (o *overlayUI) takeInitprog() bool { v := o.initprog; o.initprog = false; return v }

// rebuild reconstruit l'arbre selon l'état. 3b.2 : seule la vue Main existe ; Browse
// (3b.3) et ConfirmSwitch (Inc 5) retombent sur Main en attendant (jamais atteints ici).
func (o *overlayUI) rebuild() {
	o.root.RemoveChildren()
	card := o.card()
	switch o.state {
	// case overlay.StateBrowse: o.buildBrowser(card) // 3b.3
	// case overlay.StateConfirmSwitch: ...            // Inc 5
	default:
		o.buildMain(card)
	}
	o.root.AddChild(card)
}

// buildMain rend la vue principale : en-tête, médias montés (lecture seule en 3b.2),
// actions système, et « Reprendre » en action primaire.
func (o *overlayUI) buildMain(card *widget.Container) {
	header := widget.NewContainer(
		widget.ContainerOpts.Layout(widget.NewRowLayout(
			widget.RowLayoutOpts.Direction(widget.DirectionVertical),
			widget.RowLayoutOpts.Spacing(2),
		)),
	)
	header.AddChild(widget.NewText(widget.TextOpts.Text("DCMO5 — Pilotage", o.faceTitle, colText)))
	header.AddChild(widget.NewText(widget.TextOpts.Text("Émulation en pause — Échap pour reprendre", o.faceLabel, colMuted)))
	card.AddChild(header)
	card.AddChild(o.separator())

	// Médias : liste data-driven des Params LiveMutable, EN LECTURE (3b.2). Le montage/
	// l'éjection (boutons » et ×) arrivent en 3b.3.
	card.AddChild(o.sectionLabel("Médias montés"))
	descs := uimodel.DescribeLive(o.profile, o.live)
	if len(descs) == 0 {
		card.AddChild(o.hint("Aucun média configurable"))
	} else {
		grid := widget.NewContainer(
			widget.ContainerOpts.WidgetOpts(stretchH()),
			widget.ContainerOpts.Layout(widget.NewGridLayout(
				widget.GridLayoutOpts.Columns(2),
				widget.GridLayoutOpts.Spacing(16, 8),
				widget.GridLayoutOpts.Stretch([]bool{false, true}, nil),
			)),
		)
		for _, d := range descs {
			grid.AddChild(widget.NewText(
				widget.TextOpts.Text(d.Label, o.faceLabel, colMuted),
				widget.TextOpts.Position(widget.TextPositionStart, widget.TextPositionCenter),
			))
			name, col := "Aucun", colMuted
			if s, _ := d.Value.(string); s != "" && s != "." {
				name, col = ellipsizeName(filepath.Base(s), maxFileNameRunes), colText
			}
			grid.AddChild(widget.NewText(
				widget.TextOpts.Text(name, o.faceLabel, col),
				widget.TextOpts.Position(widget.TextPositionStart, widget.TextPositionCenter),
			))
		}
		card.AddChild(grid)
	}

	card.AddChild(o.separator())
	card.AddChild(o.sectionLabel("Système"))
	card.AddChild(o.button("Réinitialiser (Reset)", o.btnImg, o.txtColor, func() { o.reset = true }))
	card.AddChild(o.button("Redémarrage doux (Init prog)", o.btnImg, o.txtColor, func() { o.initprog = true }))
	card.AddChild(o.button("Quitter", o.btnImg, o.txtColor, func() { o.quit = true }))

	if o.errText != "" {
		card.AddChild(widget.NewText(
			widget.TextOpts.Text("⚠  "+o.errText, o.faceLabel, colDanger),
			widget.TextOpts.MaxWidth(cardWidth-56),
		))
	}

	card.AddChild(o.separator())
	card.AddChild(o.primaryButton("Reprendre", func() { o.resume = true }))
}
