# Changelog

Toutes les évolutions notables de **DCMO5 Moderne** (portage Go/Ebitengine de
l'émulateur Thomson MO5 [DCMO5 v11](http://dcmo5.free.fr/)).

Format inspiré de [Keep a Changelog](https://keepachangelog.com/fr/1.1.0/) ;
versionnage [SemVer](https://semver.org/lang/fr/).

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
