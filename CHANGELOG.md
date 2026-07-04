# Changelog

Toutes les évolutions notables de **DCMO5 Moderne** (portage Go/Ebitengine de
l'émulateur Thomson MO5 [DCMO5 v11](http://dcmo5.free.fr/)).

Format inspiré de [Keep a Changelog](https://keepachangelog.com/fr/1.1.0/) ;
versionnage [SemVer](https://semver.org/lang/fr/).

## [Non publié] — v2, multi-machines (TO8D utilisable)

Généralisation **multi-machines** : émulation du **TO8D** en plus du MO5. Le
TO8D boote, se pilote au clavier français AZERTY, accepte les manettes
(clavier + gamepad standard), et permet le **changement de machine à chaud**
sans relancer l'émulateur. Le **MO5 (v1) reste pleinement fonctionnel** —
aucune régression côté MO5 (parité bits joystick figée par tests miroirs).

Conception : [`DESIGN/MACHINE_PROFILES.md`](DESIGN/MACHINE_PROFILES.md) +
[`DESIGN/JOYSTICK.md`](DESIGN/JOYSTICK.md).

### Ajouté

- **Architecture multi-machines** : profils de machine (`MachineProfile`) +
  registre, **moteur d'émulation partagé** (boucle CPU/IRQ/vidéo/audio factorisée),
  MO5 refactoré en *device* du moteur. Conception : [`DESIGN/MACHINE_PROFILES.md`](DESIGN/MACHINE_PROFILES.md).
- **Émulation TO8D complète** (gate-array) : mémoire 512 Ko + banking, vidéo 5 modes
  + palette EF9369, timer 6846 + lignes d'IRQ, traps d'E/S (cassette, disquette,
  crayon optique, souris, imprimante) + son, **clavier TO8D AZERTY-FR**
  (scancode + IRQ gate-array, CAPSLOCK, table complète chiffres + symboles +
  accents directs `é è à ç ù`), **joystick** sur les registres `0xE7CC/0xE7CD`
  avec mux `port[0x0E/0F]&4`.
- **TO8D *bootable*** : profil sélectionnable au launcher (présélectionnable
  via `--machine to8d`), intégration au moteur partagé, chargement de
  `rom/to8d.rom` (BASIC + moniteur, patchs *trap* en mémoire, tout-ou-rien)
  ([#118](https://github.com/Lesur-ai/dcmo5/issues/118) /
  [#146](https://github.com/Lesur-ai/dcmo5/pull/146)). Affichage au **bon
  ratio** via `DisplayGeometry` ([#147](https://github.com/Lesur-ai/dcmo5/issues/147)
  corrigé par [#152](https://github.com/Lesur-ai/dcmo5/pull/152)).
- **Profil TO9+ minimal** : profil `to9p`, découpage ROM 80 Ko
  (64 Ko BASIC/logiciels + 16 Ko moniteur), repli launcher/CLI sur
  `rom/to9p.rom` ([#186](https://github.com/Lesur-ai/dcmo5/issues/186) /
  [#187](https://github.com/Lesur-ai/dcmo5/pull/187)).
- **Patchs ROM TO9+ effectifs** : les copies mémoire BASIC/moniteur sont
  alignées sur DCTO9P v11 pour détourner cassette, disque, souris, crayon,
  imprimante et clavier vers les traps émulés. La date de boot est injectée au
  format `jj-mm-aa`, comme DCTO9P v11. Le patcher est tout-ou-rien, idempotent
  et refuse les variantes ROM inconnues
  ([#195](https://github.com/Lesur-ai/dcmo5/issues/195)).
- **Clavier TO9+** : modèle clavier `TO9PModel` et variante gate-array TO9+
  explicite. Les frappes TO9+ publient désormais le code ASCII attendu via
  `E7DE/E7DF` d'après DCTO9P v11, au lieu d'hériter du chemin clavier TO8D
  (`0x30F8`/`0x3125`).
- **Boot TO9+ non-GUI validé** : invariant déterministe sur `rom/to9p.rom`
  (PC reset `0xFDA0`, progression CPU, framebuffer rendu/non uniforme,
  déterminisme inter-instances et signature FNV-1a `0xbe3a0985` après patchs
  ROM et date fixe de test en mémoire).
- **Smoke GUI TO9+ borné** : variables de diagnostic `DCMO5_SMOKE_FRAMES` et
  `DCMO5_SMOKE_SCREENSHOT` pour lancer TO9+ par le vrai chemin Ebitengine,
  capturer un PNG après un nombre de frames rendues, puis quitter proprement.
  Cela valide le chemin application/rendu. Un smoke `--exec '1\n'` valide en
  plus la saisie clavier jusqu'au prompt BASIC, sans prétendre certifier tous
  les logiciels souris/crayon.
- **Assets logiciels TO/MEMO7** : ajout des disquettes `blueberry_to8.fd`,
  `bob-winner_moto.fd`, `lemmings_to8.fd`, `les-bd-1_to8.fd`,
  `les-bd-2_to8.fd`, `space-racer_to8.fd`, ainsi que des cartouches
  `compilation_memo7.rom`, `blitz_memo7.rom` et `autotest3_memo7.rom`
  dans `software/memo7/`
  ([#187](https://github.com/Lesur-ai/dcmo5/pull/187)).
- **Overlay de pilotage Échap** (lot [#117](https://github.com/Lesur-ai/dcmo5/issues/117),
  PRs [#148](https://github.com/Lesur-ai/dcmo5/pull/148)–[#161](https://github.com/Lesur-ai/dcmo5/pull/161)) :
  remplace le menu v1 par une carte `ebitenui` superposée au framebuffer
  gelé. Médias éditables (cassette, disquette, cartouche) + actions système
  (Reset / Init prog / Quitter / Changer machine / Key Joystk) + bouton
  « Appliquer et reprendre ». Capture clavier stricte ; aucune touche fantôme
  à la reprise.
- **Changement de machine à chaud** (PRs [#162](https://github.com/Lesur-ai/dcmo5/pull/162)
  / [#163](https://github.com/Lesur-ai/dcmo5/pull/163)) : bouton « Changer
  machine » dans l'overlay, MO5 ↔ TO8D, ordre `New → Stop ancien → close
  media → teardownAudio → attach → mount → applyWindowSize → initAudio →
  Start`. Validation pure `PrepareSwitch` AVANT arrêt (ROM absente → erreur
  affichée, session intacte). Éjection systématique des médias (familles
  incompatibles). Lancement direct via `--machine to8d` au CLI sans ROM
  pré-configurée : repli en cascade sur `rom/to8d.rom`
  ([#170](https://github.com/Lesur-ai/dcmo5/pull/170)).
- **Clavier TO8D AZERTY-FR complet** (PRs
  [#165](https://github.com/Lesur-ai/dcmo5/pull/165) Inc Kc /
  [#166](https://github.com/Lesur-ai/dcmo5/pull/166) Inc Ka /
  [#167](https://github.com/Lesur-ai/dcmo5/pull/167)+[#168](https://github.com/Lesur-ai/dcmo5/pull/168) Inc Kb) :
  - **Inc Kc** : ordre modificateurs (SHIFT/CNT/ACC) avant caractères dans
    `Host.tick`, par construction (méthode `Model.ModifierKeys()` data-driven,
    nécessaire au latching du gate-array TO8D).
  - **Inc Ka** : `Model.SpecialKeys` data-driven par machine (remplace la
    `var keyMapping` figée MO5). MO5 = 18 entrées verbatim, TO8D = 32
    entrées (modifs, édition, flèches, F1/F2/F4, 13 numpad). Fix critique :
    Enter → `0x46` ENT principale (au lieu de `0x34` = ESPACE TO8D — bug
    d'origine qui faisait taper un espace).
  - **Inc Kb** : table `charToTO8D` AZERTY complète (24 paires chiffres +
    symboles + accents directs). Convention « accent direct sans SHIFT,
    chiffre AVEC SHIFT » alignée sur `pckeycode[]` de référence Coulom.
- **Support joystick complet** (lot J0..J4b, PRs
  [#169](https://github.com/Lesur-ai/dcmo5/pull/169)–[#178](https://github.com/Lesur-ai/dcmo5/pull/178)
  + [#179](https://github.com/Lesur-ai/dcmo5/pull/179)) : convention bits
  LOGIQUE INVERSÉE (0 = appuyé, repos `{0xFF, 0xC0}`) figée par tests miroirs
  MO5/TO8D ; gate-array TO8D `SetJoystick` câblé sur `0xE7CC/0xE7CD` avec
  fix bug latent (lecture `joysAction | sound` au lieu de `sound` seul) ;
  pipeline `App.SetInput → Host.tick → machine.SetJoystick → port matériel →
  CPU 6809` validé bout-en-bout. Conception : [`DESIGN/JOYSTICK.md`](DESIGN/JOYSTICK.md).
  - **Joystick clavier** activable via bouton overlay « **Key Joystk : ON/OFF** ».
    J1 = flèches + RightShift fire ; J2 = WASD (= ZQSD visuel AZERTY-FR) +
    LeftShift fire. WASD exclus du clavier émulation quand ON (sinon BASIC
    pollué) ; OFF = WASD tapent normalement. Le toggle est persisté comme
    préférence utilisateur globale.
  - **Gamepad matériel** : standard layout Ebitengine, jusqu'à 2 manettes
    simultanées (J1 + J2) par ordre de connexion, hot-plug par réconciliation
    à chaque frame, deadzone 0.3, DPad OR stick gauche, fire = bouton A OR B.
    **Start/Menu** ouvre et ferme l'overlay (utilisation gamepad-only).
- **Clavier généralisé** *data-driven* : modèle de clavier par machine
  (`internal/keyboard.Model`), méthodes `ModifierKeys()` + `SpecialKeys`
  data-driven, injection des constantes `ebiten.Key` depuis `internal/app`
  pour préserver la pureté CI du paquet `keyboard`.
- **IHM *data-driven*** : couche pure `internal/uimodel` (descripteurs de
  widgets + composition joystick : `JoystickFromKeys`, `JoystickFromGamepad`,
  `MergeJoysticks`) + dépendance **ebitenui**, avec garde-fou CI de
  cross-compilation **Windows `CGO_ENABLED=0`**.
- **Suivi des ROM/cartouches Thomson** v2/v3 (firmwares TO8D/TO9/… + cartouches
  MEMO5) dans le dépôt, sous la même réserve de licence que la v1
  (cf. [`DESIGN/LICENSING.md`](DESIGN/LICENSING.md)).

### Corrigé

- **Clignotement écran palette TO8D/TO9+** ([#197](https://github.com/Lesur-ai/dcmo5/issues/197)) :
  la publication du framebuffer par le Host est désormais cadencée sur la trame
  vidéo Thomson (`64×312` cycles) au lieu d'un rythme hôte 60 Hz. Cela évite
  d'échantillonner des états intermédiaires visibles pendant les animations ou
  modifications de palette du firmware.
- **Joystick clavier après perte de focus** : la fenêtre publie désormais un
  joystick neutre quand Ebitengine perd le focus, évitant qu'une direction
  maintenue reste collée après un alt-tab.
- **Bug latent registre joystick TO8D `0xE7CD`** ([#171](https://github.com/Lesur-ai/dcmo5/pull/171)) :
  la lecture retournait `g.sound` seul au lieu de `g.joysAction | g.sound`
  (cf. ref C `dcto8demulation.c Mgetto8d`). Silencieux tant que `joysAction`
  était toujours 0, mais aurait masqué les boutons fire J1/J2 dès le câblage
  joystick.
- **Visuel overlay** ([#164](https://github.com/Lesur-ai/dcmo5/pull/164)) : champs
  « Aucun fichier » survolés trop saillants et asymétriques. Rééquilibrage
  de la palette (`colField`/`colFieldHi`) + padding droit + retrait du
  `BackgroundImage` redondant du Container porteur.
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
