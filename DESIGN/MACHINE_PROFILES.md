# Architecture multi-machines & IHM de pilotage (v2 / v3)

> Statut : proposition de conception — revue Codex intégrée.
> Date : 2026-06-20 (révisé le jour même après revue Codex).
> Périmètre : faire évoluer DCMO5 d'un émulateur mono-machine (MO5, v1) vers une
> plateforme multi-machines Thomson, avec une IHM de pilotage professionnelle.
> Cibles v2 : **TO8D**, **TO9+**. Cibles v3 : **MO6/PC128**, **TO7**, **TO7/70**.
> Exigence structurante : l'abstraction doit **absorber les machines v3 sans
> refonte du cœur**.

---

## 1. Objectif

La v1 émule le MO5 de bout en bout. Le code est propre et bien découpé, mais
**figé sur une seule machine** : les dimensions vidéo, la palette, le nombre de
touches et la carte mémoire sont câblés dans `internal/spec` et dans un type
concret `core.Machine`. La v2 introduit d'autres machines Thomson ; il faut donc
une **abstraction de machine** sous laquelle MO5 se range sans changer de
comportement, et au-dessus de laquelle l'IHM (launcher + overlay) devient
**pilotée par les données** (chaque machine déclare ses paramètres).

Ce document définit ce contrat (`MachineProfile` + `Machine`), montre comment le
code actuel s'y adapte, et cartographie les trois familles matérielles à couvrir.

## 2. Ce qui est déjà réutilisable (acquis v1)

L'audit des sources de référence (`dcto8d.2009.05`, `dcto9p v11`, projet Theodore)
et la relecture du code v1 établissent :

- **CPU 6809 (`internal/cpu6809`)** : **totalement agnostique**. Il ne connaît que
  l'interface `Bus { Read8(uint16) uint8; Write8(uint16, uint8) }` et lit son
  vecteur reset en `0xFFFE`. Identique pour MO5/TO8D/TO9+/MO6/TO7. **Réutilisé tel
  quel.**
- **Timing raster** : 64 cycles/ligne × 312 lignes × 50 Hz, signaux `Initn`/`Iniln`
  — **identiques** sur MO5 et la famille TO. La structure de boucle `core.Step()`
  est commune à toutes les machines.
- **Modèle audio** : niveau de haut-parleur 6 bits échantillonné par accumulateur
  de cycles (`Step()` → `appendSample`). Le TO8/TO9 alimente ce même niveau via le
  port `e7cd` (`sound = c & 0x3f`). **Cadence d'échantillonnage commune.**
- **Hôte temps réel (`internal/emu.Host`)** : goroutine propriétaire de la machine,
  double-buffer vidéo, ring audio, commandes médias. **Découplé de l'UI** ; il pilote
  la machine par un jeu de méthodes (Step, SetKey, FramebufferInto, DrainAudio,
  Reset, Mount*…) — c'est déjà, de fait, une interface.
- **Médias (`internal/media`)** : interfaces `Tape`/`Disk`/`Cartridge`/`PrinterSink`
  sans dépendance OS. Réutilisables ; les formats TO (.fd secteur, .k7 octet)
  s'y logent.

## 3. Ce qui couple actuellement le code au MO5

Points précis à découpler (ce sont les seuls) :

1. **`core.Machine` est un type concret MO5** : carte mémoire (`Read8`/`Write8`),
   ports `0xA7Cx`, traps `Entreesortie`, décodage vidéo, tout est MO5.
2. **`emu.Host` et `app` dépendent du type concret `*core.Machine`**
   (`emu.New(m *core.Machine, …)`, `app.New(machine *core.Machine)`).
3. **`internal/spec` fige les dimensions MO5** utilisées partout :
   `FrameWidth=336`, `FrameHeight=216`, `KeyMax=58`, la palette 16 couleurs fixe,
   la carte mémoire 48 Ko. `emu.InputState.Keys` est un `[spec.KeyMax]bool` ;
   les framebuffers de `Host` sont alloués à `spec.FrameWidth*FrameHeight`.
4. **`cmd/dcmo5/main.go` construit un `core.Options` MO5** depuis les flags CLI.

## 4. Le contrat : `MachineProfile` (statique) + `Machine` (runtime)

Deux concepts distincts, dans un nouveau paquet `internal/machine` (agnostique) :

- **`MachineProfile`** — descriptif **statique** d'un modèle : identité, famille,
  **schéma de paramètres déclaratif**, et une **fabrique**. Consommé par le
  launcher et le registre. C'est l'équivalent Go idiomatique du couple
  `ThomsonModel` + `SystemRom` de Theodore.
- **`Machine`** — contrat **runtime** qu'un `MachineProfile.New(cfg)` produit, et
  que `emu.Host`/l'UI pilotent sans rien savoir du modèle.

```go
// internal/machine/machine.go
package machine

// Machine : contrat runtime piloté par l'hôte et l'UI, indépendant du modèle.
type Machine interface {
    // Exécution
    Step(cycles int) int   // avance d'au plus cycles, retourne les cycles consommés
    Reset()                // reset matériel (efface la RAM)
    Initprog()             // reset doux (RAM conservée)

    // Entrées (l'espace de touches est défini par la machine — cf. §8 clavier)
    // Entrées — état IDEMPOTENT réappliqué à chaque tick par l'hôte ; la machine
    // détecte elle-même les TRANSITIONS d'appui (indispensable au clavier TO8D qui
    // émet scancode + IRQ sur front, sinon rafale d'IRQ — revue Codex, bloquant).
    SetKey(k Key, pressed bool)
    SetJoystick(j JoystickInput)
    SetPointer(p PointerInput) // crayon ET souris (TO) : type, boutons, coords (X jusqu'à 640)

    // Vidéo — FrameSize est CONSTANT pour une instance de machine (336×216 MO5,
    // 672×216 famille TO). Les modes vidéo (ex. 80 colonnes) sont des résolutions de
    // DÉCODAGE dans cette frame logique, PAS un redimensionnement runtime (revue
    // Codex, majeur ; cf. dcto8dglobal.h XBITMAP=672). L'hôte dimensionne au New().
    FrameSize() (w, h int)        // taille du framebuffer logique (fixe par machine)
    FramebufferInto(dst []uint32) // rend dans dst (len ≥ w*h)

    // Audio
    AudioSampleRate() int
    DrainAudio(dst []uint8) int

    // Médias à chaud
    MountTape(media.Tape); EjectTape()
    MountDisk(media.Disk); EjectDisk()
    MountCartridge(media.Cartridge); EjectCartridge()
    MountPrinter(media.PrinterSink); EjectPrinter() // sortie imprimante (trap 0x51 ; MO5 et TO)

    // Observabilité
    CPUSnapshot() cpu6809.Snapshot
}
```

`PointerInput` unifie crayon optique et souris (les TO ont les deux ; X jusqu'à 640
en mode 80 colonnes). Le contrat `Machine` n'expose **pas** d'`IRQ()` : les lignes
d'interruption (trame 50 Hz, timer 6846, clavier) sont gérées **dans le moteur**
(cf. §5), pas pilotées par l'hôte.

```go
type PointerInput struct {
    Kind   PointerKind // Pen | Mouse
    X, Y   int         // repère écran actif de la machine
    Button bool        // bouton crayon / clic souris
}
```

```go
// internal/machine/profile.go
type Family int
const (
    FamilyMO         Family = iota // MO5, MO6, PC128 — vidéo 0x0000, clavier MO
    FamilyTOGateArray              // TO8, TO8D, TO9, TO9+ — vidéo 0x4000, gate array
    FamilyTO7                      // TO7, TO7/70 — BASIC cartouche, ROM décalée
)

// MachineProfile : descriptif statique d'un modèle émulable.
type MachineProfile struct {
    ID     string                          // "mo5","to8d","to9p","mo6","to7","to770"
    Name   string                          // "Thomson MO5"
    Family Family
    Params []Param                         // schéma déclaratif rendu par l'UI
    New    func(cfg Config) (Machine, error) // fabrique d'une instance runtime
}

// Param : un paramètre configurable, rendu GÉNÉRIQUEMENT par le launcher/overlay.
type Param struct {
    Key         string    // "ram","rom","tape","disk","video",...
    Label       string    // libellé affiché
    Kind        ParamKind // Enum | File | Bool | Int
    Default     any
    Options     []Option  // pour Enum (RAM 512 Ko / mode vidéo / variante ROM…)
    FileExt     []string  // pour File (".k7", ".fd", ".rom")
    Required    bool
    LiveMutable bool            // modifiable à chaud (overlay) vs boot-only (launcher) — revue Codex
    Validate    func(any) error // validation/coercition typée de Config[Key] (nil = aucune)
}

type ParamKind int
const ( ParamEnum ParamKind = iota; ParamFile; ParamBool; ParamInt )

type Option struct{ Value any; Label string }

// Config : valeurs saisies dans le launcher, passées à New().
type Config map[string]any
```

```go
// internal/machine/registry.go — registre peuplé par init() de chaque machine.
var registry []MachineProfile
func Register(p MachineProfile) { registry = append(registry, p) }
func Profiles() []MachineProfile { return registry }
func ByID(id string) (MachineProfile, bool) { /* … */ }
```

Chaque paquet machine s'enregistre :

```go
// internal/machine/mo5/mo5.go
func init() {
    machine.Register(machine.MachineProfile{
        ID: "mo5", Name: "Thomson MO5", Family: machine.FamilyMO,
        Params: paramsMO5, New: newMO5,
    })
}
```

**Conséquence clé** : ajouter le TO9+ (ou en v3 le MO6) = écrire un paquet qui
s'enregistre. **Aucune ligne d'UI, d'hôte ou de CLI à modifier** — le launcher
itère `machine.Profiles()` et rend `Params` ; le CLI résout `--machine <id>` via
`machine.ByID`.

## 5. Moteur partagé vs cœurs séparés (décision centrale)

Les machines partagent la boucle d'exécution (CPU, comptage de cycles,
échantillonnage audio, cadence de trame + IRQ 50 Hz) mais diffèrent par la carte
mémoire, le décodage vidéo, les traps et le **timing des périphériques** (le 6846
de la famille TO décrémente un timer à **chaque instruction** et lève une IRQ —
voir `dcto8demulation.c:Run()`). Deux options :

- **(A) Cœurs séparés** : chaque machine réimplémente sa boucle `Step`. Simple à
  isoler, mais **duplique** le timing/audio/IRQ (≈ identiques) → risque de dérive.
- **(B) Moteur partagé + `Device` injecté** (recommandé) : un paquet
  `internal/engine` possède la boucle commune (CPU, cycles, audio, trame) et
  appelle un `Device` fourni par la machine pour tout ce qui lui est propre.

```go
// internal/engine/engine.go (option B, recommandée)
type Device interface {
    cpu6809.Bus               // Read8/Write8 = carte mémoire de la machine
    Trap(code int)            // dispatch I/O (Entreesortie) propre à la machine
    OnInstructionCycles(c int, irq *IRQLines) // timing périph. (6846, IRQ clavier) ; MO5 = no-op
    SoundLevel() uint8        // niveau audio courant à échantillonner
    FrameSize() (w, h int)    // fixe par machine (cf. §4)
    DecodeFrame(dst []uint32)
}

// IRQLines : lignes d'interruption NIVEAU-déclenchées, détenues par l'engine et
// échantillonnées en frontière d'instruction (revue Codex, bloquant). La famille TO
// pilote une ligne IRQ composite (timer 6846 + clavier) avec durée d'impulsion et
// clear conditionnel — PAS un cpu.IRQ() ponctuel : une source asserte la ligne et la
// maintient jusqu'à acquittement, donc l'IRQ n'est pas perdue si I est masqué lors de
// l'assertion. MO5 n'utilise que la source « trame ».
type IRQLines struct{ /* sources : trame, timer, clavier… (assert/clear par source) */ }
func (l *IRQLines) Assert(src IRQSource)
func (l *IRQLines) Clear(src IRQSource)
```

Ordre par instruction (engine) : `CPU.Step` → trap éventuel (coût 64) → échantillon
audio + avance vidéo → `Device.OnInstructionCycles(c, &irq)` → l'engine présente au
CPU le niveau d'IRQ courant avant l'instruction suivante. La source « trame » (50 Hz)
est gérée par l'engine, commune à toutes les machines.

En (B), MO5 et la famille gate-array deviennent chacun un `Device` ; `Machine`
(le contrat runtime) est une fine enveloppe `engine + Device`. La boucle de
`core.Step()` actuelle migre **telle quelle** dans `engine`, avec deux points
d'extension : `dev.OnCycles(c)` (no-op pour MO5) et `dev.Trap(-c)`.

> **Décision validée (2026-06-20) : option (B).** Elle écrit le timing/audio une
> seule fois, colle au modèle paramétré éprouvé par Theodore, et réduit la v3 à de
> nouveaux `Device`. L'option (A) reste un repli si l'extraction du moteur s'avère
> risquée en cours de route.

## 6. MO5 derrière le contrat, sans changement de comportement

Le `core.Machine` actuel se scinde proprement :

- la **boucle `Step`**, l'**échantillonnage audio** et la **cadence de trame**
  → `internal/engine` (inchangés algorithmiquement) ;
- la **carte mémoire** (`Read8`/`Write8`), les **ports `0xA7Cx`**, les **traps**
  (`entreesortie`), le **décodage vidéo** (`FramebufferInto`/palette/`composeLine`)
  → `internal/machine/mo5` en tant que `Device` ;
- les **constantes MO5** (frame 336×216, `KeyMax=58`, palette) quittent `spec`
  pour `machine/mo5` ; `spec` ne garde que le **transverse** (`CPUClockHz`,
  vecteurs 6809, cadence ligne/trame).

Garde-fou : les **tests de fidélité** existants (checksums ROM/RAM, tests longs
ROM réelle, déterminisme) doivent passer **à l'identique** après ce déplacement —
c'est le critère d'acceptation du lot « abstraction » (refactor sans régression).

## 7. Les trois familles et leurs points de variation

Cartographie issue de l'audit (réf. Theodore `motoemulator.c`, sources Coulom) :

| Axe | Famille **MO** | Famille **TO gate-array** | Famille **TO7** |
|---|---|---|---|
| Machines | MO5 (v1), **MO6/PC128** (v3) | **TO8/TO8D/TO9/TO9+** (v2) | **TO7, TO7/70** (v3) |
| Base RAM vidéo | `0x0000` | `0x4000` | `0x4000` |
| ROM système | `0xF000` | `0xE000` | `0xE800` (décalée) |
| Banking | MO5 simple ; MO6 = gate-array pagé 512 K | gate-array 512 K (`e7e4`–`e7e7`) | TO7 aucun ; TO7/70 banques RAM |
| BASIC | en ROM | en ROM | **en cartouche** |
| Vidéo | MO5 : 1 mode, palette fixe ; MO6 : modes TO8 + compat MO5 | **5 modes** + palette programmable EF9369 | TO7 : 8 coul ; TO7/70 : 16 coul |
| Clavier | codé PB7 + bouton | scancode + IRQ (TO8) / table ASCII (TO9) | matrice 8×8 |
| Chips | 6821 | **6846** (timer+IRQ) + 6821 | 6821 |

Lecture stratégique :

- **TO8D et TO9+ (v2/v2.1)** partagent ~95 % : un seul `Device` « gate-array »
  paramétré, dont la divergence **principale est le clavier**, mais PAS la seule —
  aussi la séquence de reset, le patch/date ROM et la sémantique IRQ timer/clavier
  (revue Codex, mineur). Le `Device` gate-array expose donc une **table de points de
  variation** explicite (clavier, layout ROM, reset). Voir les notes d'audit en mémoire.
- **MO6 (v3)** = gate-array « saveur MO » : il **réutilisera l'essentiel du
  `Device` gate-array de la v2** (vidéo en `0x0000`, clavier MO, mode compat MO5).
  Quasi gratuit une fois la v2 faite.
- **TO7 / TO7/70 (v3)** = le vrai travail neuf de v3 : génération antérieure
  (BASIC en cartouche, ROM décalée, banking différent). TO7 est le cas minimal.

## 8. Impacts sur l'hôte, l'UI et les entrées

L'abstraction force des adaptations plus larges que le seul `emu.Host` (revue
Codex, majeur) :

1. **`emu.Host` ET `internal/app` sur `machine.Machine`** (au lieu de
   `*core.Machine`). Au-delà de `Host`, `app` câble aussi `ebiten.Image`, les buffers
   pixels/octets, le `Layout`, la taille fenêtre, le scaling et la conversion crayon
   sur `spec.FrameWidth/Height` : tout cela doit passer par `FrameSize()` de la machine.
2. **Framebuffer dimensionné PAR MACHINE, une fois.** `FrameSize()` est fixe pour une
   instance (cf. §4) : hôte et app allouent au `New()` de la machine, **sans**
   réallocation par changement de mode vidéo. Changer de machine (overlay) =
   réinstanciation → nouveaux buffers. Pas de course de redimensionnement en session.
3. **Entrées multi-machines.** `KeyMax` varie (58 MO5 / 84 TO8-TO9) → `emu.InputState`
   passe à une représentation non figée. La **sémantique** est arrêtée dès le contrat
   (§4) : `SetKey`/`SetPointer` publient un état idempotent, la machine détecte les
   transitions (sinon rafale d'IRQ TO8D). La traduction touche physique hôte → touche
   machine (aujourd'hui MO5-spécifique dans `internal/keyboard` : layout-safe,
   modificateurs, table ASCII TO9) est portée par un **modèle clavier** déclaré par le
   `MachineProfile` ; `internal/keyboard` se généralise. **Lot à part entière** (§10).

## 9. IHM de pilotage (ebitenui)

Décisions actées : **ebitenui** (widgets purs Go, préserve le binaire unique et la
release cross-compile `CGO_ENABLED=0`) + structure **launcher puis overlay**.

- **Launcher** (`internal/app/launcher`, ebitenui) : au démarrage, liste
  `machine.Profiles()` → sélecteur de machine ; pour la machine choisie, **rend
  `Params` génériquement** (Enum→liste, File→sélecteur fichier, Bool→case,
  Int→champ) ; bouton **Démarrer** → construit `Config` → `profile.New(cfg)` →
  `emu.New(m, …)` → boucle émulateur.
- **Overlay en session** (Échap) : même moteur de rendu de `Params` (reconfig à
  chaud quand c'est permis) + commandes de pilotage (reset, pause, montage/éjection
  médias, **changer de machine** = retour launcher / réinstanciation via le
  registre). Remplace le menu `basicfont` v1.
- **Pilotée par les données** : launcher et overlay ne contiennent **aucune
  connaissance d'un modèle précis**. Ajouter MO6/TO7 en v3 n'ajoute pas d'écran.

> Dépendance nouvelle : `github.com/ebitenui/ebitenui` (pur Go, compatible
> Ebitengine + `CGO_ENABLED=0` Windows). À acter dans `techContext`.

## 10. Découpage proposé (futur Epic v2)

**Périmètre Epic v2 = TO8D** (décision 2026-06-20). **TO9+ = incrément v2.1**
(son delta ≈ clavier table ASCII + layout ROM 1 blob), traité après mise en service
et validation utilisateur du TO8D.

Lots, dans l'ordre de dépendance RÉVISÉ après la revue Codex (poser MO5 derrière le
contrat AVANT d'extraire l'engine et le gate-array, pour éviter le big-bang). Chacun
= une PR (workflow PR-only + revue) ; chaque lot reste verrouillé par les tests de
non-régression MO5. Le n° d'issue GitHub est indiqué entre crochets.

1. **Contrat + adapter MO5** [#107] : paquet `internal/machine` (`Machine`,
   `MachineProfile`, `Param`, `Registry`) INCLUANT le **modèle de lignes d'IRQ**, la
   **sémantique d'entrées** (`SetKey` idempotent/transition, `SetPointer`),
   l'imprimante et le `Param` enrichi (`LiveMutable`/`Validate`). Le `core.Machine`
   existant est **adapté** derrière l'interface, **sans déplacement**. *(Socle.)*
2. **Hôte & app agnostiques** [#110] : `emu.Host` **et** `internal/app` sur
   `machine.Machine` ; framebuffer dimensionné par machine ; conversion pointeur ;
   CLI `--machine`. *(Avant toute extraction de moteur.)*
3. **Extraction `internal/engine` + `Device` sous MO5** [#108] : boucle
   Step/audio/trame + lignes d'IRQ migrent dans `engine` ; MO5 devient un `Device`.
4. **MO5 comme Device, sans régression** [#109] : constantes hors `spec` ; **critère
   BLOQUANT : fidélité identique** (checksums ROM/RAM, tests longs, déterminisme).
5. **Device gate-array** [#112] : mémoire 512 K + banking `e7e4`–`e7e7` + `e7c9` +
   recouvrement ROM `e7e6`.
6. **Vidéo gate-array** [#113] : 5 modes + palette programmable EF9369 (frame 672×216).
7. **Timer 6846 + PIA système** [#114] : timer/IRQ clavier via le **modèle de lignes
   d'IRQ** (`OnInstructionCycles`).
8. **ROM & traps TO8D** [#115] : layout ROM (2 blobs + patchs) ; disque secteur `.fd`,
   cassette octet `.k7`, imprimante, **crayon + souris** (`0x4b`/`0x4e`/`0x52`).
9. **Clavier — généralisation** [#111] : modèle clavier porté par le profil ;
   `internal/keyboard` data-driven ; `InputState` non figé.
10. **Clavier TO8D** [#116] : tables scancode + IRQ sur transition (via le lot 9).
11. **IHM ebitenui** [#117] : launcher (consomme les profils) + overlay de pilotage.
12. **Profil TO8D** [#118] : enregistrement, paramètres déclarés, intégration,
    validation utilisateur en GUI.

> Remaniement Codex (2026-06-20) : l'ancien ordre extrayait `internal/engine` avant de
> poser MO5 derrière le contrat — point de régression maximal. Le nouvel ordre adapte
> d'abord MO5 sans le déplacer (1), bascule l'hôte/app sur l'interface (2), puis
> n'extrait l'engine qu'ensuite (3-4), gate-array après.

Incrément **v2.1** (après mise en service TO8D) : **variante clavier TO9+** (table
ASCII), **layout ROM TO9+** (1 blob `to9prom`), **profil TO9+** et invariant de
boot non-GUI sur `rom/to9p.rom` (signature framebuffer FNV-1a `0xdfa2f5c5` après
progression depuis le vecteur reset `0xFDA0`). Réutilise tout le Device
gate-array ci-dessus. Le smoke GUI borné passe par le chemin réel Ebitengine via
`DCMO5_SMOKE_FRAMES` + `DCMO5_SMOKE_SCREENSHOT` et capture un PNG après rendu ;
il valide le boot/rendu application, mais la compatibilité interactive complète
reste une validation séparée avant de déclarer TO9+ pleinement utilisable.

v3 (hors Epic v2) : **MO6/PC128** (réutilise 6–9), **TO7/TO7-70** (nouveau Device
famille TO7). Les ROMs de toutes ces machines sont récupérables dans Theodore
(`src/rom/*.inc`, GPLv3) — même prudence licence que les ROMs MO5 (cf. `LICENSING.md`).

## 11. Décisions à valider

1. ~~Moteur partagé (B) vs cœurs séparés (A)~~ — **validé : (B)** (2026-06-20, §5).
2. **Nommage des paquets** : `internal/machine` (contrat) + `internal/machine/<id>`
   (impls) + `internal/engine` (moteur). Alternative : `internal/profile`.
   *(Détail tranché en PR du lot 1.)*
3. **Sort de `internal/core`** : adapté derrière l'interface au lot 1 (sans
   déplacement), puis déplacé en `machine/mo5` aux lots 3-4. *(Tranché en PR.)*
4. **Représentation des touches** multi-machines (slice vs max commun) — §8.
   *(Sémantique au lot 1 ; implémentation au lot 9.)*
5. ~~Périmètre v2~~ — **validé : TO8D d'abord ; TO9+ = incrément v2.1** (2026-06-20).
6. ~~Contrat interruptions / entrées / framebuffer~~ — **arrêté via la revue Codex
   du 2026-06-20** : lignes d'IRQ niveau, entrées idempotentes + transitions,
   `SetPointer`, framebuffer fixe par machine (cf. §4, §5, §8).

## 12. Risques

- **Refactor sans régression (lots 3–4)** : la fidélité MO5 est le filet ; tout
  écart de checksum bloque le lot. L'ordre révisé (adapter avant d'extraire) réduit
  ce risque.
- **Modèle d'interruptions** : les lignes d'IRQ niveau (timer 6846 + clavier) sont le
  point le plus délicat du gate-array ; à valider sur le boot moniteur TO8D.
- **Clavier** : généralisation non triviale (modificateurs, layout-safe, table ASCII
  TO9, détection de transitions pour éviter les rafales d'IRQ). À ne pas sous-estimer.
- **ebitenui** : valider tôt le rendu, la cohabitation avec la boucle Ebitengine et le
  cross-compile Windows `CGO_ENABLED=0` (prototype jetable recommandé).

---

*Références : audit des sources `dcto8d.2009.05` / `dcto9p v11` et projet
[Theodore](https://github.com/Zlika/theodore) (core libretro GPLv3 unifiant les
9 machines Thomson à partir des mêmes sources Coulom). Voir notes d'audit v2/v3 en
mémoire de projet.*
