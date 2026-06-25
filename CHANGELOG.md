# Changelog

Toutes les évolutions notables de **DCMO5 Moderne** (portage Go/Ebitengine de
l'émulateur Thomson MO5 [DCMO5 v11](http://dcmo5.free.fr/)).

Format inspiré de [Keep a Changelog](https://keepachangelog.com/fr/1.1.0/) ;
versionnage [SemVer](https://semver.org/lang/fr/).

## [Non publié] — v2, multi-machines (en développement)

Chantier de généralisation **multi-machines** pour émuler d'autres machines
Thomson au-delà du MO5, première cible le **TO8D**.

> ⚠️ **En cours de finition** : le **TO8D boote** et se sélectionne au launcher
> (présélectionnable via `--machine to8d`), mais l'affichage n'est pas encore au bon ratio
> ([#147](https://github.com/Lesur-ai/dcmo5/issues/147)) et la finition (overlay
> d'options en jeu, validation complète des médias) reste à faire (épopée
> [#106](https://github.com/Lesur-ai/dcmo5/issues/106)). Le **MO5 (v1) reste
> pleinement fonctionnel**. Conception : [`DESIGN/MACHINE_PROFILES.md`](DESIGN/MACHINE_PROFILES.md).

### Ajouté

- **Architecture multi-machines** : profils de machine (`MachineProfile`) +
  registre, **moteur d'émulation partagé** (boucle CPU/IRQ/vidéo/audio factorisée),
  MO5 refactoré en *device* du moteur.
- **Base d'émulation TO8D** (gate-array) : mémoire 512 Ko + banking, vidéo 5 modes
  + palette EF9369, timer 6846 + lignes d'IRQ, traps d'E/S (cassette, disquette,
  crayon optique, souris, imprimante) + son, **clavier TO8D** (scancode + IRQ
  gate-array, CAPSLOCK).
- **TO8D *bootable*** : profil TO8D sélectionnable au launcher (présélectionnable
  via `--machine to8d`), intégration au moteur partagé (synchronisation du faisceau vidéo *gate-array* ;
  IRQ de fin de trame 50 Hz neutralisée pour la famille *gate-array*, l'interruption
  provenant du timer 6846) et chargement de `rom/to8d.rom` (BASIC + moniteur, patchs
  *trap* appliqués en mémoire, tout-ou-rien et idempotents) — le **moniteur TO8D
  démarre à l'écran** ([#118](https://github.com/Lesur-ai/dcmo5/issues/118) /
  [#146](https://github.com/Lesur-ai/dcmo5/pull/146)). Le ratio d'affichage reste
  à corriger ([#147](https://github.com/Lesur-ai/dcmo5/issues/147)).
- **Clavier généralisé** *data-driven* : modèle de clavier par machine, état
  d'entrée non figé.
- **IHM *data-driven*** : couche pure `internal/uimodel` (descripteurs de widgets
  dérivés des paramètres de profil) + dépendance **ebitenui**, avec garde-fou CI
  de cross-compilation **Windows `CGO_ENABLED=0`**.
- **Suivi des ROM/cartouches Thomson** v2/v3 (firmwares TO8D/TO9/… + cartouches
  MEMO5) dans le dépôt, sous la même réserve de licence que la v1
  (cf. [`DESIGN/LICENSING.md`](DESIGN/LICENSING.md)).

### Corrigé

- **Montage de cartouche fidèle à la réf C `Loadmemo()`** : `MountCartridge`
  effectue désormais « RAZ RAM + `Initprog()` » au lieu d'un *hard reset* complet —
  préservant ports d'E/S, cadençage vidéo et crayon optique — pour le gate-array
  **TO8D** ([#132](https://github.com/Lesur-ai/dcmo5/issues/132) /
  [#134](https://github.com/Lesur-ai/dcmo5/pull/134)) **et** le cœur **MO5**
  ([#138](https://github.com/Lesur-ai/dcmo5/issues/138) /
  [#139](https://github.com/Lesur-ai/dcmo5/pull/139)). Une cartouche nil/vide
  désactive le banc (sémantique `Loadmemo(name="")`).

## [1.0.0] — 2026-06-07

Première version fonctionnelle : un MO5 utilisable de bout en bout (BASIC,
cassette, disquette/DOS, cartouche, clavier, son), avec ROM et logiciels inclus.

### Ajouté

- **CPU Motorola 6809** complet (registres, ALU, branchements, interruptions,
  modes d'adressage), validé par golden tests.
- **Machine MO5** : bus mémoire, RAM/ROM, ports d'E/S, bancs MEMO5, timing
  vidéo (64 cycles/ligne, 312 lignes/trame, IRQ 50 Hz).
- **Vidéo** : framebuffer logique 336×216, palette Thomson, correction gamma.
- **Audio** mono (haut-parleur 1 bit) échantillonné à 48 kHz, architecture
  *audio-driven* (goroutine dédiée, ring FIFO, sans verrou partagé sur le cœur).
- **Clavier MO5** *layout-safe* (AZERTY/QWERTY), touches **maintenues** (jeux +
  répétition) ; **joysticks** émulés au clavier.
- **Médias** : cassette `.k7`, disquette `.fd` (densité variable + DOS contrôleur
  **CD90-640**), cartouche MEMO5 `.rom`, imprimante parallèle vers fichier.
- **Menu de pilotage in-app** (`Échap`) : charger/éjecter cassette, disquette,
  cartouche ; `Init prog` ; `Reset`. **Montage/éjection à chaud**.
- **Saisie programmée** `--exec` (séquence tapée au démarrage) et **copier-coller**
  du presse-papier (`Cmd+V` / `Ctrl+V`).
- **CLI** : `-rom`, `-tape`, `-disk`, `-cart`, `-disk-rom`, `-exec`,
  `-exec-delay`, `-no-audio` ; préférences utilisateur persistées (macOS/Linux).
- **Assets inclus** dans le dépôt (sous réserve, cf. `DESIGN/LICENSING.md`) :
  ROM système MO5, ROM contrôleur CD90-640, sélection de logiciels MO5.
- **Distribution** : workflow de release CI (archives macOS arm64/amd64 +
  Linux amd64) ; suite de tests déterministes et tests longs sur ROM réelle.

### Corrigé

- **Prompt BASIC READY** : opcodes direct-page manquants du 6809 corrigés
  (faux traps d'E/S qui désynchronisaient le PC).
- **Cassette** : `LOAD"` lit désormais la `.k7`. La vraie ROM pilotait la
  cassette par bit-bang matériel non émulé ; alignement sur le modèle *trap* de
  dcmo5 v11 via un **patch ROM en mémoire** (le fichier ROM n'est jamais modifié).
- **Disquette** : acceptation des `.fd` de densité variable (1 face / 2 faces,
  40/80 pistes) avec bornage dynamique ; **mapping de la ROM contrôleur
  CD90-640** (amorçage DOS) ; sémantique d'erreur `Diskerror` alignée réf C.
- **Clavier** : les touches-caractères sont maintenues en continu (auparavant
  jouées en impulsions → injouable pour les jeux).
- **Menu** : le navigateur de fichiers démarre au répertoire courant.

### Limites connues

- **Crayon optique** : la fonction BASIC `PEN(...)` ne suit pas la souris (la ROM
  dérive la position d'un handshake matériel du crayon non émulé) — comportement
  identique à dcmo5 v11. Voir
  [issue #86](https://github.com/Lesur-ai/dcmo5/issues/86).
- Extensions hors périmètre v1 (Nanoréseau, QD90-128, IN57-001, DI90-011).

[1.0.0]: https://github.com/Lesur-ai/dcmo5/releases/tag/v1.0.0
