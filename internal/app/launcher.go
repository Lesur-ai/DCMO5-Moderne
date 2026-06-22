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
package app

import (
	"fmt"
	"image/color"
	"os"
	"path/filepath"

	"github.com/ebitenui/ebitenui"
	eimage "github.com/ebitenui/ebitenui/image"
	"github.com/ebitenui/ebitenui/widget"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"golang.org/x/image/font/basicfont"

	"github.com/Lesur-ai/dcmo5/internal/machine"
	"github.com/Lesur-ai/dcmo5/internal/uimodel"
)

const (
	launcherWidth  = 720
	launcherHeight = 520
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
	browseKey string
	browseDir string
	browseExt []string

	errText string

	start    bool
	startReq startRequest

	// Ressources de rendu partagées (police text/v2, images de bouton, couleurs).
	face     *text.Face
	btnImg   *widget.ButtonImage
	txtColor *widget.ButtonTextColor
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
	var face text.Face = text.NewGoXFace(basicfont.Face7x13)
	l := &launcher{
		profiles: profiles,
		mediaDir: mediaDir,
		lister:   lister,
		face:     &face,
		btnImg: &widget.ButtonImage{
			Idle:    eimage.NewNineSliceColor(color.NRGBA{R: 0x3a, G: 0x3a, B: 0x4a, A: 0xff}),
			Hover:   eimage.NewNineSliceColor(color.NRGBA{R: 0x4a, G: 0x4a, B: 0x60, A: 0xff}),
			Pressed: eimage.NewNineSliceColor(color.NRGBA{R: 0x2a, G: 0x2a, B: 0x36, A: 0xff}),
		},
		txtColor: &widget.ButtonTextColor{Idle: color.White, Hover: color.White, Pressed: color.White},
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

	l.root = widget.NewContainer(
		widget.ContainerOpts.BackgroundImage(eimage.NewNineSliceColor(color.NRGBA{R: 0x1e, G: 0x1e, B: 0x28, A: 0xff})),
		widget.ContainerOpts.Layout(widget.NewRowLayout(
			widget.RowLayoutOpts.Direction(widget.DirectionVertical),
			widget.RowLayoutOpts.Padding(widget.NewInsetsSimple(16)),
			widget.RowLayoutOpts.Spacing(8),
		)),
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

// rebuild reconstruit l'arbre de widgets selon l'état (vue principale ou navigateur).
func (l *launcher) rebuild() {
	l.root.RemoveChildren()
	if l.browseKey != "" {
		l.buildBrowser()
		return
	}
	l.buildMain()
}

// buildMain rend la vue principale : sélecteur de machine + paramètres + Démarrer.
func (l *launcher) buildMain() {
	l.root.AddChild(l.text("DCMO5 Moderne — choix de la machine"))

	// Sélecteur de machine : un bouton par profil (surligné si sélectionné).
	machines := widget.NewContainer(
		widget.ContainerOpts.Layout(widget.NewRowLayout(
			widget.RowLayoutOpts.Direction(widget.DirectionHorizontal),
			widget.RowLayoutOpts.Spacing(6),
		)),
	)
	for i, p := range l.profiles {
		idx, name := i, p.Name
		if i == l.selected {
			name = "▶ " + name
		}
		machines.AddChild(l.button(name, func() {
			if idx == l.selected {
				return
			}
			l.selected = idx
			l.values = uimodel.InitialValues(l.profiles[idx]) // reset : pas de fuite inter-profils
			l.errText = ""
			l.rebuild()
		}))
	}
	l.root.AddChild(machines)

	// Paramètres du profil sélectionné, rendus génériquement via uimodel.Describe.
	prof := l.currentProfile()
	for _, d := range uimodel.Describe(prof, l.values) {
		l.root.AddChild(l.paramRow(d))
	}

	l.root.AddChild(l.button("Démarrer", func() {
		l.startReq = startRequest{profile: l.currentProfile(), values: cloneConfig(l.values)}
		l.start = true
	}))

	if l.errText != "" {
		l.root.AddChild(widget.NewText(widget.TextOpts.Text("⚠ "+l.errText, l.face, color.NRGBA{R: 0xff, G: 0x80, B: 0x80, A: 0xff})))
	}
}

// paramRow rend une ligne « libellé : [contrôle] » pour un descripteur, selon son Kind.
func (l *launcher) paramRow(d uimodel.WidgetDescriptor) *widget.Container {
	row := widget.NewContainer(
		widget.ContainerOpts.Layout(widget.NewRowLayout(
			widget.RowLayoutOpts.Direction(widget.DirectionHorizontal),
			widget.RowLayoutOpts.Spacing(8),
		)),
	)
	label := d.Label
	if d.Required {
		label += " *"
	}
	row.AddChild(l.text(label + " :"))

	switch d.Kind {
	case machine.ParamFile:
		row.AddChild(l.button(fileLabel(d.Value), func() {
			l.browseKey = d.Key
			l.browseExt = append([]string(nil), d.FileExt...)
			l.browseDir = l.mediaDir
			l.rebuild()
		}))
	case machine.ParamBool:
		on, _ := d.Value.(bool)
		row.AddChild(l.button(boolLabel(on), func() {
			l.values[d.Key] = !on
			l.rebuild()
		}))
	case machine.ParamEnum:
		row.AddChild(l.button(enumLabel(d), func() {
			l.values[d.Key] = nextEnum(d)
			l.rebuild()
		}))
	case machine.ParamInt:
		cur, _ := d.Value.(int)
		row.AddChild(l.button("-", func() { l.values[d.Key] = cur - 1; l.rebuild() }))
		row.AddChild(l.text(fmt.Sprintf("%d", cur)))
		row.AddChild(l.button("+", func() { l.values[d.Key] = cur + 1; l.rebuild() }))
	}
	return row
}

// buildBrowser rend le navigateur de fichiers pour le Param File en cours (browseKey),
// alimenté par uimodel.ListDir (logique pure, testée en CI).
func (l *launcher) buildBrowser() {
	l.root.AddChild(l.text("Choisir un fichier — " + l.browseDir))
	l.root.AddChild(l.button("← Annuler", func() {
		l.browseKey = ""
		l.rebuild()
	}))
	for _, e := range uimodel.ListDir(l.lister, l.browseDir, l.browseExt) {
		entry := e
		name := entry.Name
		if entry.IsDir {
			name += "/"
		}
		l.root.AddChild(l.button(name, func() {
			target := filepath.Join(l.browseDir, entry.Name)
			if entry.IsDir {
				l.browseDir = filepath.Clean(target)
				l.rebuild()
				return
			}
			l.values[l.browseKey] = target
			l.mediaDir = filepath.Dir(target)
			l.browseKey = ""
			l.rebuild()
		}))
	}
}

// ── Helpers de rendu et de libellé ─────────────────────────────────────────────

func (l *launcher) text(s string) *widget.Text {
	return widget.NewText(widget.TextOpts.Text(s, l.face, color.White))
}

func (l *launcher) button(label string, onClick func()) *widget.Button {
	return widget.NewButton(
		widget.ButtonOpts.Image(l.btnImg),
		widget.ButtonOpts.Text(label, l.face, l.txtColor),
		widget.ButtonOpts.TextPadding(widget.NewInsetsSimple(6)),
		widget.ButtonOpts.ClickedHandler(func(*widget.ButtonClickedEventArgs) { onClick() }),
	)
}

// fileLabel affiche le nom de base du fichier choisi, ou « (parcourir…) » si vide.
func fileLabel(v any) string {
	if s, _ := v.(string); s != "" {
		return filepath.Base(s)
	}
	return "(parcourir…)"
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
