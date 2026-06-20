# Architecture multi-machines & IHM de pilotage (v2 / v3)

> Statut : proposition de conception — à valider.
> Date : 2026-06-20.
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
    SetKey(k Key, pressed bool)
    SetJoystick(j JoystickInput)
    SetPen(x, y int, pressed bool)

    // Vidéo — dimensions DYNAMIQUES (peuvent changer au runtime selon le mode)
    FrameSize() (w, h int)        // taille courante du framebuffer logique
    FramebufferInto(dst []uint32) // rend dans dst (len ≥ w*h courant)

    // Audio
    AudioSampleRate() int
    DrainAudio(dst []uint8) int

    // Médias à chaud
    MountTape(media.Tape); EjectTape()
    MountDisk(media.Disk); EjectDisk()
    MountCartridge(media.Cartridge); EjectCartridge()

    // Observabilité
    CPUSnapshot() cpu6809.Snapshot
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
    Key      string    // "ram","rom","tape","disk","video",...
    Label    string    // libellé affiché
    Kind     ParamKind // Enum | File | Bool | Int
    Default  any
    Options  []Option  // pour Enum (RAM 512 Ko / mode vidéo / variante ROM…)
    FileExt  []string  // pour File (".k7", ".fd", ".rom")
    Required bool
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
    cpu6809.Bus          // Read8/Write8 = carte mémoire de la machine
    Trap(code int)       // dispatch I/O (Entreesortie) propre à la machine
    OnCycles(n int)      // timing périphériques (6846, IRQ clavier…) ; MO5 = no-op
    SoundLevel() uint8   // niveau audio courant à échantillonner
    FrameSize() (w, h int)
    DecodeFrame(dst []uint32)
}
// Engine possède *cpu6809.CPU, l'accumulateur audio, le compteur de trame ;
// au bout de 312 lignes il lève cpu.IRQ() (commun à toutes les machines).
// Le Device tient une back-référence au CPU pour lever ses propres IRQ (6846).
```

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

- **TO8D et TO9+ (v2)** partagent ~95 % : un seul `Device` « gate-array »
  paramétré, dont la **seule vraie divergence est le clavier** (+ layout ROM et
  séquence reset). Voir les notes d'audit en mémoire.
- **MO6 (v3)** = gate-array « saveur MO » : il **réutilisera l'essentiel du
  `Device` gate-array de la v2** (vidéo en `0x0000`, clavier MO, mode compat MO5).
  Quasi gratuit une fois la v2 faite.
- **TO7 / TO7/70 (v3)** = le vrai travail neuf de v3 : génération antérieure
  (BASIC en cartouche, ROM décalée, banking différent). TO7 est le cas minimal.

## 8. Impacts sur l'hôte, l'UI et les entrées

L'abstraction force trois adaptations concrètes, toutes circonscrites :

1. **`emu.Host` dépend de `machine.Machine`** (au lieu de `*core.Machine`). Les
   appels sont déjà les bons ; seul le type change.
2. **Framebuffer à taille dynamique** : `Host` interroge `FrameSize()` (qui peut
   varier — ex. mode 80 colonnes `640×…` du TO8) et **réalloue** `fbFront`/`fbBack`
   si la taille change (rare → coût négligeable). On supprime l'allocation figée à
   `spec.FrameWidth*FrameHeight`. Le rendu `app` met à l'échelle la taille courante.
3. **Clavier multi-machines** : `KeyMax` varie (58 MO5 / 84 TO8-TO9). Deux sous-
   problèmes — (a) l'espace de touches logiques par machine ; (b) la traduction
   touche physique hôte → touche machine, aujourd'hui MO5-spécifique dans
   `internal/keyboard` (apprentissage layout-safe, politique modificateurs, table
   ASCII TO9 différente). Proposition : le `MachineProfile` expose un **modèle
   clavier** (tables + politique) et `internal/keyboard` se généralise pour le
   consommer. `emu.InputState` passe à une représentation non figée (slice ou max
   commun). **C'est un lot à part entière** (cf. §10), pas un détail.

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

Lots, dans l'ordre de dépendance. Chacun = une PR (workflow PR-only + revue) ;
chaque lot reste verrouillé par les tests de non-régression MO5.

1. **Contrat & registre** : paquet `internal/machine` (`Machine`, `MachineProfile`,
   `Param`, `Registry`). Aucun comportement modifié. *(Socle.)*
2. **Moteur partagé** : extraire `internal/engine` (boucle Step/audio/trame) +
   contrat `Device` (option B §5). *(Décision (A)/(B) à valider avant.)*
3. **MO5 derrière le contrat** : déplacer le MO5 en `machine/mo5` (Device),
   sortir ses constantes de `spec`. **Critère : fidélité identique.**
4. **Hôte & UI agnostiques** : `emu.Host` + `app` sur `machine.Machine` ;
   framebuffer dynamique ; CLI `--machine`.
5. **Clavier généralisé** : modèle clavier porté par le profil ; `internal/keyboard`
   data-driven ; `InputState` non figé.
6. **Device gate-array** : carte mémoire 512 K + banking `e7e4`–`e7e7`, ROM overlay.
7. **Vidéo gate-array** : 5 modes + palette programmable EF9369.
8. **6846 + PIA système** : timer/IRQ via `Device.OnCycles`.
9. **ROM & traps TO** : layout ROM (TO8D 2 blobs + patchs / TO9+ 1 blob),
   disque secteur `.fd`, cassette octet `.k7`, imprimante, crayon/souris.
10. **Clavier TO8D** (scancode + IRQ).
11. **Launcher ebitenui** (consomme les profils) + **overlay** de pilotage.
12. **Profil TO8D** : enregistrement, paramètres déclarés, intégration, validation
    utilisateur en GUI.

Incrément **v2.1** (après mise en service TO8D) : **variante clavier TO9+** (table
ASCII), **layout ROM TO9+** (1 blob `to9prom`), **profil TO9+**. Réutilise tout le
Device gate-array ci-dessus.

v3 (hors Epic v2) : **MO6/PC128** (réutilise 6–9), **TO7/TO7-70** (nouveau Device
famille TO7). Les ROMs de toutes ces machines sont récupérables dans Theodore
(`src/rom/*.inc`, GPLv3) — même prudence licence que les ROMs MO5 (cf. `LICENSING.md`).

## 11. Décisions à valider

1. ~~Moteur partagé (B) vs cœurs séparés (A)~~ — **validé : (B)** (2026-06-20, §5).
2. **Nommage des paquets** : `internal/machine` (contrat) + `internal/machine/<id>`
   (impls) + `internal/engine` (moteur). Alternative : `internal/profile`.
   *(Détail tranché en PR du lot 1.)*
3. **Sort de `internal/core`** : devient `machine/mo5` (option B) — ou reste un
   alias le temps de la transition ? *(Détail tranché en PR du lot 3.)*
4. **Représentation des touches** multi-machines (slice vs max commun) — §8.
   *(Détail tranché en PR du lot 5.)*
5. ~~Périmètre v2~~ — **validé : TO8D d'abord ; TO9+ = incrément v2.1** (2026-06-20).

## 12. Risques

- **Refactor sans régression (lots 2–3)** : la fidélité MO5 est le filet ; tout
  écart de checksum bloque le lot. Risque maîtrisé si les tests restent vert.
- **Framebuffer dynamique** : bien gérer le changement de taille en cours de
  session (mode 80 colonnes) sans glitch ni course avec le thread d'affichage.
- **Clavier** : la généralisation est le point le moins trivial (politique
  modificateurs, apprentissage layout-safe, table ASCII TO9). À ne pas sous-estimer.
- **ebitenui** : valider tôt le rendu + la cohabitation avec la boucle Ebitengine
  et le cross-compile Windows `CGO_ENABLED=0` (prototype jetable recommandé).

---

*Références : audit des sources `dcto8d.2009.05` / `dcto9p v11` et projet
[Theodore](https://github.com/Zlika/theodore) (core libretro GPLv3 unifiant les
9 machines Thomson à partir des mêmes sources Coulom). Voir notes d'audit v2/v3 en
mémoire de projet.*
