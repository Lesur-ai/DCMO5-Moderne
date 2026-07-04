# DCMO5 Moderne

Portage moderne de l'émulateur Thomson MO5 [DCMO5 v11](http://dcmo5.free.fr/)
(C/SDL, 2007, © Daniel Coulom) vers **Go / Ebitengine**.

Ce projet est un logiciel libre sous licence **GNU GPL v3+**. Voir `LICENSE`
et `NOTICE`.

**Version : 2.1.0** — historique dans [`CHANGELOG.md`](CHANGELOG.md). Cette
version ajoute le socle **multi-machines** : MO5, TO8D et TO9+. Le TO8D est
utilisable (clavier AZERTY-FR complet, joystick clavier + gamepad standard,
changement de machine à chaud) et le TO9+ dispose d'un boot validé, du clavier
BASIC et du joystick clavier.

## Captures d'écran

| Jeu cassette (`.k7`) | DOS disquette (CD90-640) |
|:--:|:--:|
| ![Jeu MO5 chargé depuis cassette](screenshoot/mo5_aigle.png) | ![DOS MO5 depuis disquette](screenshoot/mo5_disk.png) |

---

## Périmètre historique v1 — MO5

### Fonctionnalités émulées

- **Vidéo** MO5 (framebuffer logique 336×216, palette Thomson, timing faisceau/IRQ 50 Hz)
- **Audio** mono (haut-parleur 1 bit, échantillonné à 48 kHz)
- **Clavier MO5** *layout-safe* (AZERTY/QWERTY) : les touches sont **maintenues**
  (jeux + répétition) ; mapping clavier hôte
- **Joysticks** émulés au clavier
- **Cassette** `.k7`, **disquette** `.fd` (densité variable + DOS contrôleur CD90-640),
  **cartouche** MEMO5 `.rom`
- **Menu de pilotage in-app** (touche `Échap`) : charger/éjecter cassette,
  disquette, cartouche ; Reset / Init prog
- **Médias à chaud** : montage/éjection sans relancer l'émulateur
- **Saisie programmée** `--exec` (taper une séquence au démarrage) et
  **copier-coller** depuis le presse-papier (`Cmd+V` / `Ctrl+V`)
- **Imprimante** parallèle vers fichier
- Préférences utilisateur portables (Windows / macOS / Linux)
- ROM système, ROM contrôleur CD90-640 et logiciels MO5 **inclus dans le dépôt**
  (sous réserve — voir [`DESIGN/LICENSING.md`](DESIGN/LICENSING.md))

### Limites connues

- **Crayon optique** : la routine bas niveau (souris → coordonnées MO5) est en
  place, mais la fonction BASIC `PEN(...)` **ne suit pas la souris**. La ROM MO5
  dérive la position d'un **handshake matériel du crayon optique** (synchro
  faisceau) que la version moderne n'émule pas — comportement **identique à
  dcmo5 v11**, qui ne fait pas non plus suivre la souris à `PEN`. Voir
  [issue #86](https://github.com/Lesur-ai/dcmo5/issues/86) (amélioration future).

### Exclusions explicites de la v1

Les extensions suivantes **ne sont pas émulées**, conformément au périmètre de
DCMO5 v11 :

- Nanoréseau Leanord
- Quick Disk Drive QD90-128
- Contrôleur IN57-001
- Contrôleur DI90-011
- Toute extension assimilée

---

## Version 2.1.0 — multi-machines (TO8D + TO9+)

La version 2.1.0 ajoute les **Thomson TO8D** et **TO9+** à côté du MO5.
Architecture de profils de machine + moteur d'émulation partagé, base
gate-array complète (mémoire/vidéo/timer/E/S + clavier + joystick), clavier
généralisé *data-driven* et IHM *data-driven* (ebitenui, cross-compilation
Windows `CGO_ENABLED=0`).

**Ce qui est utilisable côté TO8D** :
- Boot avec ROM, BASIC opérationnel, ratio d'affichage correct.
- Clavier français AZERTY complet (lettres + chiffres + symboles + accents
  directs `é è à ç ù`).
- **Joystick** : émulation au clavier (toggle dans l'overlay) et **gamepad
  standard** (Xbox/PS/Switch Pro, jusqu'à 2 manettes simultanées en hot-plug,
  bouton Start ouvre l'overlay).
- Changement de machine MO5 ↔ TO8D **à chaud** via l'overlay (`Échap` →
  bouton « Changer machine »), médias éjectés au switch et son préservé.
- Présélection via `dcmo5 --machine to8d` : le launcher s'ouvre sur le TO8D
  avec repli en cascade sur `rom/to8d.rom` si la ROM n'est pas mémorisée en
  config.

**Ce qui est validé côté TO9+** :
- Profil `to9p`, ROM `rom/to9p.rom` et clavier TO9+ ASCII distinct du chemin
  TO8D (`E7DE/E7DF`).
- Patchs ROM TO9+ appliqués en mémoire, alignés sur DCTO9P v11, pour détourner
  les routines cassette/disque/souris/crayon/clavier vers les traps émulés.
- Date de boot TO9+ injectée en mémoire au format `jj-mm-aa`, comme DCTO9P v11.
- Boot non-GUI couvert par un invariant déterministe sur la ROM réelle :
  progression depuis le vecteur reset `0xFDA0`, framebuffer rendu et signature
  FNV-1a `0xbe3a0985`.
- Smoke GUI borné disponible via le vrai chemin Ebitengine (`cmd` → `app.Run`
  → `Host` → `Draw`) :

  ```bash
  DCMO5_SMOKE_FRAMES=180 \
  DCMO5_SMOKE_SCREENSHOT=/tmp/dcmo5-to9p.png \
  go run ./cmd/dcmo5 --machine to9p --rom rom/to9p.rom --no-audio
  ```

- Smoke clavier minimal validé avec `--exec '1\n'` : le firmware quitte le menu
  TO9+ et arrive au prompt BASIC. Cette preuve valide une saisie bout-en-bout,
  sans prétendre certifier tous les logiciels souris/crayon.
- Joystick clavier TO9+ validé : maintenir une flèche ne bloque plus
  l'émulation, car les flèches ne sont plus envoyées simultanément au clavier
  firmware TO9+ quand le mode joystick clavier est actif.

**Switch machine et ROMs par défaut** :
- Le bouton « Changer machine » parcourt les profils disponibles (MO5, TO8D,
  TO9+) au lieu d'un aller-retour limité MO5 ↔ TO8D.
- Le launcher et l'overlay pré-remplissent les ROMs livrées quand elles
  existent : `rom/mo5-v1.1.rom`, `rom/to8d.rom`, `rom/to9p.rom`.

**Overlay de pilotage Échap** : remplace le menu v1. Carte `ebitenui`
superposée au framebuffer gelé, médias éditables + actions système (Reset
/ Init prog / Quitter / Changer machine / Key Joystk) + bouton « Appliquer
et reprendre ». `Start` du gamepad ouvre et ferme aussi l'overlay.

Le **MO5 (v1) décrit ci-dessus reste pleinement fonctionnel et inchangé** —
non-régression contrôlée par tests miroirs (parité bits joystick MO5/TO8D
figée par la suite CI).

Conception détaillée dans
[`DESIGN/MACHINE_PROFILES.md`](DESIGN/MACHINE_PROFILES.md) et
[`DESIGN/JOYSTICK.md`](DESIGN/JOYSTICK.md). Le détail
des évolutions est tenu dans [`CHANGELOG.md`](CHANGELOG.md) (section
`2.1.0`).

---

## Architecture

```
cmd/dcmo5
  └── internal/app        (Ebitengine : fenêtre, input, audio, prefs)
       └── internal/core  (machine MO5 : bus, RAM/ROM, ports, timing, IRQ)
            ├── internal/cpu6809  (Motorola 6809, pur Go, sans UI)
            ├── internal/media    (cassette, disquette, cartouche, imprimante)
            └── internal/spec     (constantes matérielles, adresses, codes touches)
```

Le cœur d'émulation (`core`, `cpu6809`, `media`, `spec`) ne dépend d'aucune
bibliothèque graphique, audio ou fichier. Ebitengine est limité à la couche
application. *(Schéma du cœur MO5 v1.)*

> **v2.1** : la généralisation multi-machines ajoute `internal/machine`
> (profils + registre), `internal/engine` (boucle d'émulation partagée),
> `internal/keyboard` (clavier *data-driven*) et `internal/uimodel` (IHM
> *data-driven*), avec la famille gate-array TO8D/TO9+ sous
> `internal/machine/gatearray`. La direction de dépendance (cœur sans UI) est
> préservée. Détails dans
> [`DESIGN/MACHINE_PROFILES.md`](DESIGN/MACHINE_PROFILES.md).

Voir [`DESIGN/ARCHITECTURE.md`](DESIGN/ARCHITECTURE.md) pour les décisions
structurantes.

---

## ROM et médias

Pour que l'émulateur soit **utilisable immédiatement**, ce dépôt inclut :

- `rom/` — ROM système **MO5** (`mo5-v1.1.rom`), ROM du contrôleur de disquette
  **CD90-640** (`cd90-640.rom`) et ROMs Thomson TO/MO utilisées par les profils
  multi-machines (`to8d.rom`, `to9p.rom`, etc.) ;
- `software/` — une sélection de **logiciels Thomson historiques** (`.k7`,
  `.fd`, `.rom`, `.EPROM`).

> **Provenance & droits.** Ces contenus proviennent du matériel et de l'écosystème
> Thomson MO5 (commercialisé en 1984) et de la communauté de préservation/émulation
> (notamment la distribution [DCMO5 v11](http://dcmo5.free.fr/) de Daniel Coulom).
> Compte tenu de l'ancienneté du matériel et de sa diffusion établie à des fins de
> préservation, le mainteneur les inclut comme raisonnablement redistribuables.
> **Ce n'est pas un avis juridique** et cela n'affirme pas un statut de domaine
> public établi. Tout ayant droit peut demander le retrait d'un contenu en
> **ouvrant une issue** sur le dépôt ; il sera retiré sans délai.

L'application peut aussi démarrer **sans ROM** (message « ROM manquante ») et
accepte l'import de vos propres fichiers. Détails : [`DESIGN/LICENSING.md`](DESIGN/LICENSING.md).

---

## Pré-requis

- **Go 1.26+** (voir `go.mod`)
- Plateformes de bureau supportées : **Windows 10/11**, **macOS** (arm64/amd64),
  **Linux** (amd64) — et plus largement toute cible supportée par Go et Ebitengine.

Le cœur est en Go pur et le rendu passe par **Ebitengine** (multi-plateforme) :
DCMO5 Moderne tourne **nativement** sur les trois OS de bureau.

### Windows — supporté nativement

Aucune dépendance système à installer (Ebitengine utilise l'API graphique native
de Windows). Avec [Go 1.26+](https://go.dev/dl/) :

```powershell
# Lancer depuis la racine du dépôt
go run ./cmd/dcmo5 -rom rom\mo5-v1.1.rom

# Ou construire un exécutable
go build -o dcmo5.exe ./cmd/dcmo5
dcmo5.exe -rom rom\mo5-v1.1.rom -tape software\yahtzee-mo5.k7
```

### macOS — supporté nativement

Aucune dépendance à installer ; `go run ./cmd/dcmo5 -rom rom/mo5-v1.1.rom`.

### Linux — dépendances système (Ebitengine)

Ebitengine requiert des bibliothèques graphiques système absentes des
environnements CI headless. Pour un build et des tests locaux sur Linux :

```bash
# Debian / Ubuntu
sudo apt-get install -y \
  libgl1-mesa-dev \
  libx11-dev \
  libxcursor-dev \
  libxi-dev \
  libxinerama-dev \
  libxrandr-dev \
  libxxf86vm-dev

# Fedora / RHEL
sudo dnf install -y \
  mesa-libGL-devel \
  libX11-devel \
  libXcursor-devel \
  libXi-devel \
  libXinerama-devel \
  libXrandr-devel \
  libXxf86vm-devel
```

> **CI headless :** `internal/app` initialise Ebitengine (GLFW) et n'est donc
> pas exécuté dans la suite headless — la CI lance `go test -race` sur tous les
> autres paquets, et ne teste de `internal/app` que ses fonctions pures.
> `go build ./...` requiert les libs ci-dessus sur Linux.

## Utilisation

### Démarrage rapide

La ROM et des logiciels étant inclus dans le dépôt, l'émulateur est utilisable
immédiatement (lancé depuis la racine du projet). Le launcher pré-remplit les
ROMs par machine, et les exemples CLI explicites restent possibles :

```bash
# BASIC MO5 avec la ROM livrée
go run ./cmd/dcmo5 -rom rom/mo5-v1.1.rom

# Launcher présélectionné TO8D, ROM rom/to8d.rom pré-remplie
go run ./cmd/dcmo5 --machine to8d

# Boot direct TO9+ avec la ROM livrée
go run ./cmd/dcmo5 --machine to9p --rom rom/to9p.rom --no-audio

# Charger un jeu cassette
go run ./cmd/dcmo5 -rom rom/mo5-v1.1.rom -tape software/yahtzee-mo5.k7

# Démarrer le DOS depuis une disquette (ROM contrôleur cd90-640.rom auto-détectée)
go run ./cmd/dcmo5 -rom rom/mo5-v1.1.rom -disk software/dos-5p25-mo5.fd

# Cartouche MEMO5
go run ./cmd/dcmo5 -rom rom/mo5-v1.1.rom -cart software/glouton-memo5.rom
```

> Le menu in-app (`Échap`) permet aussi de charger/éjecter les médias **à chaud**,
> sans relancer. Les chemins sont mémorisés dans la config utilisateur
> (`~/.config/dcmo5/config.json` sous Linux,
> `~/Library/Application Support/dcmo5/config.json` sous macOS).

### Options de ligne de commande

| Option | Description |
|--------|-------------|
| `-machine <id>` | Machine à présélectionner/lancer (`mo5`, `to8d`, `to9p`) |
| `-rom <fichier>` | ROM système de la machine sélectionnée (MO5 16 Ko ; TO9+ 80 Ko avec `--machine to9p`) |
| `-tape <fichier>` | Cassette `.k7` à monter |
| `-disk <fichier>` | Disquette `.fd` à monter |
| `-cart <fichier>` | Cartouche MEMO5 `.rom` à monter |
| `-disk-rom <fichier>` | ROM du contrôleur CD90-640 (auto-détectée à côté de la ROM système si absente) |
| `-exec "<séquence>"` | Tape une séquence de touches au démarrage (`\n` = ENTRÉE, `\t` = TAB) |
| `-exec-delay <s>` | Délai avant `--exec`, le temps que l'invite BASIC apparaisse (défaut 3 s) |
| `-no-audio` | Désactive la sortie audio |
| `-version` | Affiche la version du binaire et quitte |

### Raccourcis clavier (hôte)

| Touche | Action |
|--------|--------|
| `Échap` | Ouvrir le menu de pilotage / revenir en arrière |
| `F5` | Reset machine (efface la RAM) |
| `F3` | Pause / Reprise |
| `Cmd+V` / `Ctrl+V` | Coller le presse-papier (tapé dans le MO5) |
| Fermeture fenêtre | Quitter |

Dans le **menu** (`Échap`) : flèches pour naviguer, `Entrée` pour valider —
charger/éjecter cassette, disquette, cartouche ; `Init prog` (reset doux) ;
`Reset machine`.

### Saisie programmée (`--exec`) et copier-coller

`--exec` tape automatiquement une séquence après le boot (utile pour charger et
lancer un programme), et `Cmd+V`/`Ctrl+V` colle le presse-papier comme si vous
le tapiez :

```bash
# Taper puis exécuter un petit programme BASIC au démarrage
go run ./cmd/dcmo5 -rom rom/mo5-v1.1.rom -exec '10 CLS\n20 PRINT "BONJOUR"\nRUN\n'
```

### Tests

```bash
# Suite headless (exclut internal/app qui nécessite un affichage)
go test $(go list ./... | grep -v /internal/app)

# Tests longs avec la vraie ROM (boot BASIC, cassette, disquette…)
DCMO5_LONG_TESTS=1 go test ./internal/core/...
```

---

## Distribution

Des archives binaires pré-compilées sont disponibles dans les
[releases GitHub](https://github.com/Lesur-ai/dcmo5/releases) :

- **Windows amd64** : `dcmo5-windows-amd64.zip`
- **macOS** arm64 / amd64 : `dcmo5-darwin-{arm64,amd64}.tar.gz`
- **Linux amd64** : `dcmo5-linux-amd64.tar.gz`

```bash
# macOS / Linux
tar xzf dcmo5-darwin-arm64.tar.gz
./dcmo5-darwin-arm64 -rom /chemin/vers/mo5.rom
```

```powershell
# Windows : dézipper puis lancer
dcmo5-windows-amd64.exe -rom mo5-v1.1.rom
```

`dcmo5 -version` affiche la version du binaire.

Voir [`RELEASE.md`](RELEASE.md) pour la procédure de release complète.

## Contribuer

Workflow PR-only — tout merge vers `main` passe exclusivement par une Pull
Request GitHub. Le guide de contribution (`CONTRIBUTING.md`) sera ajouté dans
le milestone P0 (issue #12).

---

## Référence historique

Ce portage s'appuie sur DCMO5 v11 comme référence fonctionnelle et
documentaire. Le code C d'origine reste la référence ; il n'est pas une
dépendance runtime de la version moderne.
