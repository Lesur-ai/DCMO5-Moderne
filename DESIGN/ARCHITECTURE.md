# Architecture DCMO5 Moderne

> Statut : document de cadrage initial.
> Date : 2026-06-04.
> Perimetre : portage moderne de `dcmo5v11.0` vers une application desktop
> macOS et Linux, avec reecriture progressive et testee en Go.

## 1. Objectif

DCMO5 Moderne est une nouvelle implementation de l'emulateur Thomson MO5
historique DCMO5 v11. Le code C d'origine reste la reference fonctionnelle et
documentaire, mais il ne doit pas devenir une dependance runtime permanente.

La premiere version vise un emulateur desktop complet :

- rendu video MO5 ;
- audio mono ;
- clavier MO5 et mapping clavier hote ;
- joysticks emules au clavier ;
- crayon optique via souris ;
- chargement cassette `.k7`, disquette `.fd`, cartouche MEMO5 `.rom` ;
- imprimante parallele vers fichier ;
- preferences utilisateur portables macOS/Linux.

Les extensions explicitement non emulees par DCMO5 v11 restent hors perimetre
initial : nanoreseau Leanord, Quick Disk Drive QD90-128, IN57-001, DI90-011 et
extensions assimilees.

## 2. Decisions structurantes

### Langage et runtime

Le projet cible Go avec Ebitengine.

Raisons :

- Go donne un coeur testable, portable et simple a distribuer ;
- Ebitengine correspond mieux a une application temps reel 2D/audio qu'un
  toolkit bureautique classique ;
- macOS et Linux restent des cibles de premier ordre ;
- la reecriture force a sortir de la dette globale du C historique.

Le choix est volontairement plus ambitieux qu'un portage SDL minimal. Il impose
des tests de fidelite rigoureux, sinon le risque principal devient une regression
silencieuse de l'emulation.

### Politique de dependances

Le coeur d'emulation ne depend d'aucune bibliotheque graphique, audio ou fichier.
Ebitengine est limite a la couche application.

Les packages internes doivent donc respecter cette direction :

```text
cmd/dcmo5
  -> internal/app
      -> internal/core
          -> internal/cpu6809
          -> internal/media
          -> internal/spec
```

Les imports inverses sont interdits : `internal/core`, `internal/cpu6809`,
`internal/media` et `internal/spec` ne doivent jamais importer `internal/app`,
Ebitengine, ni des packages d'UI.

### Ressources et droits

Le code DCMO5 v11 est GPLv3+. Les bibliotheques SDL historiques sont sous LGPL,
mais elles ne sont pas retenues comme dependance de la nouvelle version.

Point bloquant : les ROM MO5 et CD90-640 sont signalees dans la licence
historique comme des contenus soumis a copyright. Les logiciels MO5 fournis dans
`dcmo5v11.0/software` doivent etre traites avec la meme prudence.

Decision initiale :

- ne pas embarquer de ROM ni logiciel MO5 copyrightes dans l'application moderne
  sans validation explicite ;
- fournir un mecanisme d'import utilisateur ;
- documenter les chemins attendus ;
- garder les donnees de test libres ou generees dans le repo moderne.

## 3. Architecture cible

### Couche `internal/spec`

`spec` centralise les constantes materielles et les dimensions publiques :

- horloge CPU nominale ;
- taille framebuffer logique ;
- dimensions RAM, ROM, ports et cartouches ;
- adresses memoire significatives ;
- codes de touches MO5 ;
- parametres disque et cassette.

Cette couche ne contient pas de logique mutable.

### Couche `internal/cpu6809`

`cpu6809` implemente le Motorola 6809 avec types explicites :

- registres 8 bits en `uint8` ;
- registres 16 bits en `uint16` ;
- flags exposes via constantes nommees ;
- bus memoire fourni par interface ;
- execution deterministe instruction par instruction.

Interface de principe :

```go
type Bus interface {
    Read8(addr uint16) uint8
    Write8(addr uint16, value uint8)
}

type CPU struct { ... }

func (c *CPU) Reset(vector uint16)
func (c *CPU) Step() int
func (c *CPU) Snapshot() Snapshot
```

`Step` retourne le nombre de cycles consommes par l'instruction. Les operations
illegales du 6809 doivent etre representees explicitement, pas cachees dans une
valeur negative ambigue comme dans le code C historique.

### Couche `internal/media`

`media` fournit les formats et peripheriques persistants, sans connaitre les
chemins OS :

```go
type Tape interface {
    ReadByte() (byte, error)
    WriteByte(byte) error
    Rewind() error
    Position() int64
}

type Disk interface {
    ReadSector(unit, track, sector int) ([256]byte, error)
    WriteSector(unit, track, sector int, data [256]byte) error
    FormatUnit(unit int) error
}

type Cartridge interface {
    Bytes() []byte
}

type PrinterSink interface {
    WriteByte(byte) error
}
```

Les implementations fichiers seront dans ce package, mais le coeur les recevra
deja ouvertes ou construites par la couche application.

### Couche `internal/core`

`core` represente la machine MO5 complete :

- CPU 6809 ;
- RAM video et RAM utilisateur ;
- ROM systeme et ROM controleur disque ;
- cartouche et banques MEMO5 ;
- ports d'entree/sortie ;
- clavier, joystick, crayon ;
- timing video et IRQ ;
- generation framebuffer et echantillons audio.

Interface publique initiale :

```go
type Machine struct { ... }

func NewMachine(options Options) (*Machine, error)
func (m *Machine) Reset()
func (m *Machine) Step(cycles int) int
func (m *Machine) Framebuffer() []uint32
func (m *Machine) SetKey(key Key, pressed bool)
func (m *Machine) SetJoystick(input JoystickInput)
func (m *Machine) SetPen(x, y int, pressed bool)
```

`Step` execute au plus le nombre de cycles demande et retourne les cycles
effectivement consommes ou l'excedent selon la convention retenue par les tests.
Cette convention doit etre documentee au moment de l'implementation.

Le coeur ne lit jamais `./software`, ne cree jamais `dcmo5.ini` et n'ecrit jamais
directement `dcmo5-printer.txt`.

### Couche `internal/app`

`app` adapte le coeur au desktop :

- boucle Ebitengine ;
- rendu du framebuffer logique ;
- scaling entier ou preserve-ratio ;
- entree clavier/souris ;
- audio bufferise ;
- menus et statut ;
- preferences ;
- selection de fichiers ;
- chemins utilisateur macOS/Linux.

La configuration utilisateur doit utiliser les emplacements OS standard via une
abstraction `ConfigStore`, pas le repertoire courant.

Interface de principe :

```go
type ConfigStore interface {
    Load() (Config, error)
    Save(Config) error
    DataDir() string
}
```

### Couche `cmd/dcmo5`

`cmd/dcmo5` est le point d'entree executable. Il doit rester mince :

- chargement configuration ;
- creation de l'application ;
- lancement Ebitengine ;
- traduction des erreurs fatales en message utilisateur ou sortie console.

## 4. Analyse du code historique

Les fichiers C d'origine se repartissent ainsi :

| Fichier | Role historique | Implication pour le portage |
|---|---|---|
| `dc6809emul.c` | CPU Motorola 6809 | A porter avec tests instructionnels stricts. |
| `dcmo5emulation.c` | Bus MO5, RAM/ROM, ports, timing, IRQ | A porter dans `internal/core`. |
| `dcmo5video.c` | Palette, composition ligne, rendu SDL | Garder la logique de palette/composition, separer le rendu. |
| `dcmo5devices.c` | Cassette, disque, cartouche, imprimante | Separer format, stockage fichier et coeur. |
| `dcmo5main.c` | SDL init, audio callback, event loop | Remplace par `cmd/dcmo5` + `internal/app`. |
| `dcmo5dialog.c` | UI maison, menus, listes fichiers | Remplace par UI Ebitengine moderne. |
| `dcmo5keyb.c` | Mapping clavier/joystick | Reprendre le modele fonctionnel, pas les scancodes SDL 1.2. |
| `dcmo5options.c` | Preferences binaires `dcmo5.ini` | Remplacer par config versionnee et portable. |
| `dcmo5boutons.*` | Widgets bitmap maison | Hors coeur ; a remplacer par composants UI modernes. |
| `dcmo5msg.h` | Textes FR/EN | A transformer en ressources i18n simples. |

Dette critique observee :

- etat global massif ;
- `extern` entre modules ;
- couplage audio -> `Run()` -> CPU/video/peripheriques ;
- `char` signe et `short` utilises pour de l'emulation bas niveau ;
- chemins relatifs en dur ;
- fichier de preferences binaire dependant de tailles C ;
- ROM et police incorporees comme tableaux C.

## 5. Strategie de portage

Le portage se fait par tranches verifiables :

1. Mettre en place module Go, architecture de packages et CI minimale.
2. Porter les constantes `spec` et etablir les premiers tests.
3. Porter CPU 6809 instruction par instruction avec snapshots.
4. Porter bus MO5 et reset machine.
5. Porter video framebuffer sans UI avancee.
6. Porter cassette, disquette, cartouche, imprimante.
7. Brancher Ebitengine pour fenetre, input, audio, rendu.
8. Ajouter preferences, menus, chargement fichiers et packaging.

Chaque tranche doit livrer un comportement testable. Les tests ne doivent pas
simplement verifier les getters/setters : ils doivent capturer des invariants
materiels ou des regressions observables.

## 6. Tests de reference

Les tests minimaux attendus :

- CPU : opcodes representatifs, flags, PC, pile, adressages, cycles ;
- bus : mapping RAM video, RAM utilisateur, ROM systeme, ROM bank, ports ;
- reset : PC issu du vecteur reset, RAM initialisee comme l'ancien code ;
- video : checksum framebuffer apres RAM video controlee ;
- media : lecture/ecriture `.k7`, secteurs `.fd`, banques `.rom` ;
- determinisme : meme sequence d'entrees -> meme checksum RAM/framebuffer ;
- stockage : aucune dependance au repertoire courant.

Une suite de golden tests pourra comparer la nouvelle implementation a des
captures generees depuis le code C historique, mais ces captures devront etre
stockees comme donnees de test explicites et documentees.

## 7. Packaging et exploitation locale

Cibles initiales :

- macOS arm64 ;
- macOS amd64 si necessaire ;
- Linux amd64 ;
- Linux arm64 en objectif secondaire.

Le packaging ne doit pas supposer que les ROM ou logiciels MO5 sont inclus.
L'application doit demarrer sans ROM utilisateur avec un etat explicite :
"ROM manquante, importer une ROM".

Les donnees utilisateur doivent etre separees :

- configuration ;
- ROM importees ;
- medias utilisateur ;
- sorties imprimante ;
- logs eventuels.

## 8. Discipline GitHub

Le fichier `DESIGN/GITHUB_PROJECT_SETUP.md` a ete relu. Il documente un setup
observe pour `Lesur-ai/portal`, pas un setup DCMO5 pret a appliquer.

Pour DCMO5, on retient seulement les principes :

- repository prive au depart ;
- branche par defaut `main` ;
- integration via Pull Request ;
- issues pour cadrer le travail ;
- CI avant les premieres PR d'implementation ;
- labels/milestones a recreer pour les phases DCMO5, pas a copier depuis
  Portal.

Les labels et milestones DCMO5 seront definis dans un document separe avant
creation massive d'objets GitHub.

## 9. Risques ouverts

| Risque | Impact | Reponse |
|---|---|---|
| Fidelite CPU 6809 insuffisante | Programmes MO5 instables | Tests instructionnels et golden tests. |
| Timing audio/video incorrect | Son hache ou vitesse fausse | Scheduler deterministe dans le coeur. |
| Droits ROM/logiciels | Blocage distribution | Import utilisateur, pas d'embedding non valide. |
| UI Ebitengine trop custom | Ergonomie mediocre | Garder UI simple v1, priorite a la fidelite. |
| Portage trop large d'un coup | Effet tunnel | Tranches courtes avec tests et demos. |

## 10. Definition de termine initiale

Le socle d'architecture est considere pose quand :

- le module Go existe ;
- `cmd/dcmo5` lance une fenetre Ebitengine vide ou un etat ROM manquante ;
- `internal/spec`, `internal/cpu6809`, `internal/core`, `internal/media` et
  `internal/app` existent ;
- les premiers tests `spec` et `cpu6809` passent ;
- la documentation d'architecture et de setup GitHub DCMO5 est presente ;
- aucun fichier runtime moderne ne depend de `dcmo5v11.0` comme bibliotheque.
