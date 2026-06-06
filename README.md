# DCMO5 Moderne

Portage moderne de l'émulateur Thomson MO5 [DCMO5 v11](http://dcmo5.free.fr/)
(C/SDL, 2007, © Daniel Coulom) vers **Go / Ebitengine**.

Ce projet est un logiciel libre sous licence **GNU GPL v3+**. Voir `LICENSE`
et `NOTICE`.

---

## Périmètre v1

### Fonctionnalités émulées

- Rendu vidéo MO5 (framebuffer logique 336×216, palette Thomson)
- Audio mono
- Clavier MO5 + mapping clavier hôte
- Joysticks émulés au clavier
- Crayon optique via souris
- Chargement cassette `.k7`, disquette `.fd`, cartouche MEMO5 `.rom`
- Imprimante parallèle vers fichier
- Préférences utilisateur portables macOS / Linux

### Exclusions explicites de la v1

Les extensions suivantes **ne sont pas émulées**, conformément au périmètre de
DCMO5 v11 :

- Nanoréseau Leanord
- Quick Disk Drive QD90-128
- Contrôleur IN57-001
- Contrôleur DI90-011
- Toute extension assimilée

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
application.

Voir [`DESIGN/ARCHITECTURE.md`](DESIGN/ARCHITECTURE.md) pour les décisions
structurantes.

---

## ROM et médias

Pour que l'émulateur soit **utilisable immédiatement**, ce dépôt inclut :

- `rom/` — ROM système **MO5** (`mo5-v1.1.rom`) et ROM du contrôleur de disquette
  **CD90-640** (`cd90-640.rom`) ;
- `software/` — une sélection de **logiciels MO5 historiques** (`.k7`, `.fd`, `.rom`).

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

- Go 1.26+ (voir `go.mod`)
- macOS arm64 / amd64 ou Linux amd64 (cibles de premier ordre)

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

> **CI headless :** `go test -race ./...` est sûr car `internal/app` expose
> uniquement des fonctions pures testables sans initialiser Ebitengine.
> `go build ./...` requiert les libs ci-dessus sur Linux.

## Utilisation

### Lancer l'émulateur

```bash
# Sans ROM (état indéfini, avertissement affiché)
go run ./cmd/dcmo5

# Avec ROM (recommandé)
go run ./cmd/dcmo5 -rom /chemin/vers/mo5.rom

# Avec ROM + cassette
go run ./cmd/dcmo5 -rom mo5.rom -tape jeu.k7

# Avec ROM + disquette
go run ./cmd/dcmo5 -rom mo5.rom -disk dos.fd

# Avec cartouche MEMO5
go run ./cmd/dcmo5 -rom mo5.rom -cart memo5.rom
```

Les chemins sont mémorisés dans la configuration utilisateur
(`~/.config/dcmo5/config.json` sur Linux, `~/Library/Application Support/dcmo5/config.json` sur macOS)
et réutilisés au prochain lancement sans argument.

### Raccourcis clavier

| Touche | Action |
|--------|--------|
| `F5` | Reset machine |
| `F3` | Pause / Reprise |
| `Escape` | Quitter |

### Tests

```bash
go test ./...
```

---

## Distribution

Des archives binaires pré-compilées (macOS arm64/amd64, Linux amd64) sont
disponibles dans les [releases GitHub](https://github.com/Lesur-ai/dcmo5/releases).

```bash
tar xzf dcmo5-darwin-arm64.tar.gz
./dcmo5-darwin-arm64 -rom /chemin/vers/mo5.rom
```

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
