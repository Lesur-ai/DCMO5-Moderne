package machine

// Family regroupe les machines partageant l'essentiel de leur matériel (cf.
// DESIGN/MACHINE_PROFILES.md §7).
type Family int

const (
	FamilyMO          Family = iota // MO5 (v1), MO6/PC128 (v3) — vidéo 0x0000, clavier MO
	FamilyTOGateArray               // TO8/TO8D/TO9/TO9+ (v2) — vidéo 0x4000, gate array
	FamilyTO7                       // TO7, TO7/70 (v3) — BASIC cartouche, ROM décalée
)

// MachineProfile est le descripteur STATIQUE d'un modèle émulable : identité,
// famille, schéma de paramètres déclaratif (consommé génériquement par le launcher
// et l'overlay), et fabrique d'une instance runtime.
type MachineProfile struct {
	ID     string                            // "mo5","to8d","to9p","mo6","to7","to770"
	Name   string                            // libellé affiché, ex. "Thomson MO5"
	Family Family                            //
	Params []Param                           // schéma déclaratif rendu par l'UI
	New    func(cfg Config) (Machine, error) // fabrique d'une instance runtime
}

// ParamKind est le type d'un paramètre, qui détermine son rendu dans l'UI.
type ParamKind int

const (
	ParamEnum ParamKind = iota // choix parmi Options
	ParamFile                  // sélecteur de fichier (FileExt)
	ParamBool                  // case à cocher
	ParamInt                   // champ entier
)

// Option est un choix d'un paramètre ParamEnum.
type Option struct {
	Value any
	Label string
}

// Param décrit UN paramètre configurable, rendu GÉNÉRIQUEMENT par le launcher et
// l'overlay. LiveMutable et Validate (revue Codex) permettent de distinguer un
// réglage modifiable à chaud d'un réglage boot-only, et de valider/coercer la valeur.
// Clés de paramètres CONVENTIONNELLES reconnues par la couche app (internal/app) :
// la ROM système (boot-only, consommée par MachineProfile.New) et les médias montés
// à chaud après New. Un profil qui veut qu'un média soit effectivement monté par l'UI
// doit réutiliser ces clés ; une autre clé serait rendue dans le launcher mais jamais
// montée. Contrat non typé, partagé MO5/TO8D (#118).
const (
	KeyROM     = "rom"      // ROM système (File, Required, boot-only)
	KeyDiskROM = "disk-rom" // ROM contrôleur disquette (File, boot-only)
	KeyTape    = "tape"     // cassette .k7 (File, LiveMutable)
	KeyDisk    = "disk"     // disquette .fd (File, LiveMutable)
	KeyCart    = "cart"     // cartouche .rom (File, LiveMutable)
)

type Param struct {
	Key         string          // "ram","rom","tape","disk","video",... (cf. Key* conventionnels)
	Label       string          // libellé affiché
	Kind        ParamKind       //
	Default     any             // valeur par défaut
	Options     []Option        // pour ParamEnum
	FileExt     []string        // pour ParamFile (".k7", ".fd", ".rom")
	Required    bool            //
	LiveMutable bool            // modifiable à chaud (overlay) vs boot-only (launcher)
	Validate    func(any) error // validation/coercition de Config[Key] (nil = aucune)
}

// Config porte les valeurs saisies dans le launcher, passées à MachineProfile.New.
type Config map[string]any
