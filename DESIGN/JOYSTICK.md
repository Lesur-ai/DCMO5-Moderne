# DESIGN — Support joystick (MO5 + TO8D)

Conception du pipeline joystick côté hôte (clavier + gamepad matériel) et de
son intégration dans le bus machine Thomson. Implémenté par les Inc J0..J4b
(PRs #169–#179) sur la branche v2 multi-machines.

> **Lecture liée** : [`MACHINE_PROFILES.md`](MACHINE_PROFILES.md) pour le
> contrat `machine.Machine` et la séparation cœur/hôte ;
> [`ARCHITECTURE.md`](ARCHITECTURE.md) pour la direction de dépendance.

## 1. Convention bits — logique inversée

Les deux machines Thomson partagent un même registre d'état joystick à 16 bits,
publié par l'hôte via `machine.JoystickInput{Position, Action uint8}`. La
**convention bits est LOGIQUE INVERSÉE** : un bit à `0` signifie touche
**appuyée**, un bit à `1` signifie touche **relâchée**. Au repos, tous les bits
sont à `1` — d'où `machine.NeutralJoystick = {Position: 0xFF, Action: 0xC0}`.

Source de vérité : `Joysemul` cases 0-9 dans `dcmo5emulation.c` (réf C v1.1),
identiques **byte par byte** à `dcto8demulation.c`. Cette parité MO5/TO8D est
figée par deux tests miroirs synchronisés
(`internal/core/bus_test.go::TestBus_Joystick_BitConvention_Inverted` côté MO5,
`internal/machine/gatearray/io_test.go::TestSetJoystick_BitConvention_TO8D`
côté TO8D, plus
`internal/uimodel/joystick_test.go::TestJoystickFromKeys_DirectionsBitConvention`
côté entrée hôte) — toute divergence d'un des trois est un bug détecté en CI.

Mapping des bits :

| Champ      | Bit | Signification (logique inversée, 0 = appuyé) |
| ---------- | --- | --------------------------------------------- |
| `Position` | 0   | J1 nord                                       |
| `Position` | 1   | J1 sud                                        |
| `Position` | 2   | J1 ouest                                      |
| `Position` | 3   | J1 est                                        |
| `Position` | 4   | J2 nord                                       |
| `Position` | 5   | J2 sud                                        |
| `Position` | 6   | J2 ouest                                      |
| `Position` | 7   | J2 est                                        |
| `Action`   | 6   | J1 bouton fire                                |
| `Action`   | 7   | J2 bouton fire                                |
| `Action`   | 0-5 | inutilisés côté joystick — recouverts par le canal son `sound` côté MO5/TO8D (OR'és par le hardware à la lecture `0xA7CD` / `0xE7CD` en mode musique). À `0` dans `NeutralJoystick` (`0xC0 = 0b1100_0000`). |

**Piège évité** : la zéro-value Go d'un struct `JoystickInput{}` vaut
`{0x00, 0x00}`, soit toutes les directions appuyées simultanément.
`machine.NeutralJoystick` est la **seule constante d'initialisation valide**
côté hôte ; un `InputState{}` zéro-value est rattrapé par `Host.New` qui
initialise `input.Joystick` explicitement.

## 2. Pipeline complet

```
HARDWARE                  ┌──────────────────────────────┐
(clavier physique +       │  internal/app (impur, hors CI)│
 gamepad USB/Bluetooth)   │                              │
       │                  │  joystick.go (clavier)       │
       │  Ebitengine      │  gamepad.go (Ebitengine glue)│
       │  ─────────►      │                              │
       │                  │      ▼          ▼            │
       └──────────────────►  KeyCode      Snapshot       │
                          │      │          │            │
                          └──────┼──────────┼────────────┘
                                 ▼          ▼
                          ┌──────────────────────────────┐
                          │  internal/uimodel (pur, CI)  │
                          │                              │
                          │  JoystickFromKeys(map, fn)   │
                          │  JoystickFromGamepad(snap)   │
                          │      │          │            │
                          │      └────┬─────┘            │
                          │           ▼                  │
                          │     MergeJoysticks(a, b)     │
                          │           │                  │
                          └───────────┼──────────────────┘
                                      ▼
                          ┌──────────────────────────────┐
                          │  internal/emu (impur, hors CI)│
                          │                              │
                          │  Host.SetInput(InputState)   │
                          │  Host.tick()                 │
                          │    machine.SetJoystick(j)    │
                          └──────────────┬───────────────┘
                                         ▼
                          ┌──────────────────────────────┐
                          │  internal/machine/<famille>  │
                          │                              │
                          │   MO5: core.SetJoystick      │
                          │        → port 0xA7CC/0xA7CD  │
                          │                              │
                          │   TO8D: gatearray.SetJoystick│
                          │        → port 0xE7CC/0xE7CD  │
                          │  (mux port[0x0E/0F]&4)       │
                          └──────────────┬───────────────┘
                                         ▼
                                    CPU 6809 émulé
                                    (lit le port via
                                     LDA $A7CC / LDA $E7CC)
```

Trois couches strictement séparées :

1. **`internal/uimodel`** (pur, testé en CI) — connaît la convention bits
   Thomson, ne connaît rien d'Ebitengine. Type neutre `GamepadSnapshot`,
   `KeyCode = int`, fonctions `JoystickFromKeys`, `JoystickFromGamepad`,
   `MergeJoysticks` sans aucun effet de bord. Tests `-race` sur 100 % du
   code de calcul.
2. **`internal/app`** (impur, hors CI headless) — fait l'aller-retour entre
   les API Ebitengine et `uimodel`. `internal/app/joystick.go` traduit
   `ebiten.IsKeyPressed` → `KeyCode bool` ; `internal/app/gamepad.go` lit
   le standard layout Ebitengine et remplit `GamepadSnapshot`. Aucune
   logique de calcul des bits : tout est délégué à `uimodel`.
3. **`internal/machine/*`** (cœur émulé, testé en CI) — reçoit le
   `JoystickInput` final via `machine.SetJoystick(j)` et le publie sur ses
   registres matériels. MO5 a `core.SetJoystick` (déjà câblé en v1) ;
   TO8D a `gatearray.SetJoystick` (Inc J1a).

## 3. Composition (MergeJoysticks = AND bitwise)

`MergeJoysticks(a, b)` combine deux états joystick :

```go
func MergeJoysticks(a, b machine.JoystickInput) machine.JoystickInput {
    return machine.JoystickInput{
        Position: a.Position & b.Position,
        Action:   a.Action & b.Action,
    }
}
```

En **logique inversée**, l'AND bitwise réalise l'OR logique des appuis : si
l'une des deux sources éteint un bit (= appui), le résultat l'a éteint
(= appui visible côté machine). La fonction est commutative, associative,
et `NeutralJoystick` est son élément neutre — propriétés ancrées par tests.

La couche app combine trois sources potentielles à chaque frame :

```go
keyboardJoy := joystickFromKeys(ebiten.IsKeyPressed, a.joystickKBEnabled)
gamepadJoy  := a.joystickFromGamepads()  // J1 + J2 internalement Merge'és
in.Joystick = uimodel.MergeJoysticks(keyboardJoy, gamepadJoy)
```

Si l'utilisateur a **à la fois** un gamepad et le clavier joystick activé, les
deux contribuent en OR (cas non bloquant : un appui sur une seule source
suffit).

## 4. Clavier joystick — toggle « Key Joystk »

L'émulation joystick au clavier n'est **pas active par défaut**. WASD tape
normalement Z/Q/S/D AZERTY-FR en BASIC. Un toggle dans l'overlay (rangée
Système, bouton « **Key Joystk : ON/OFF** ») permet de basculer en mode
joystick au clavier.

Mapping fixe v1 (configurabilité reportée v2) :

| Joueur | Directions       | Bouton fire   |
| ------ | ---------------- | ------------- |
| J1     | `↑ ↓ ← →`        | `RightShift`  |
| J2     | `W A S D` (physique = `Z Q S D` visuel AZERTY-FR) | `LeftShift` |

Le choix `RightShift` (initialement `AltGr`) tient à la non-capture d'`AltGr`
côté Ebitengine Mac. `WASD` côté physique tombe sur `ZQSD` AZERTY-FR : pas
d'ambiguïté pour un joueur AZERTY.

**Exclusions du clavier émulation** (`internal/app/joystick.go::joystickExclusiveKeys`) :
- Mode ON : `KeyW/A/S/D` excluse des chemins `learnLiveKeys` et `resolveKeys`
  (boucle SpecialKeys ET boucle `learned`), sinon WASD pollue le BASIC avec
  Z/Q/S/D à chaque mouvement J2.
- Flèches et `Shift` *gardent* leur fonction MO5/TO8D (double-input
  accepté) — retirer Shift du clavier serait trop intrusif. Cette exception
  exploite le fait que `SHIFT` n'a pas de sémantique de caractère en BASIC.

Mode OFF (défaut) : aucune exclusion, WASD tapent normalement, et
`joystickFromKeys` retourne `NeutralJoystick` (gamepad seul actif si
branché).

## 5. Gamepad matériel — slot management + hot-plug

Jusqu'à **deux gamepads simultanés** (J1 + J2). Mapping côté Ebitengine
**standard layout** uniquement (`IsStandardGamepadLayoutAvailable`) ;
fallback bas niveau différé v2.

| Joystick Thomson | Gamepad standard                                              |
| ---------------- | ------------------------------------------------------------- |
| Direction        | `LeftTop/Bottom/Left/Right` (DPad) OR stick gauche (deadzone 0.3) |
| Bouton fire      | `RightBottom` (A) OR `RightRight` (B)                         |
| Ouvre l'overlay  | `CenterRight` (Start/Menu, just-pressed)                      |

**OR DPad + stick** : l'utilisateur choisit librement entre la croix
directionnelle et le stick gauche. `Fire = A OR B` couvre Xbox/PS/Switch
Pro sans avoir à connaître le constructeur (les conventions A/B sont
parfois inversées).

**Slot management par ordre de connexion** (`gamepadSlots [2]gamepadSlot`).
À chaque frame, `updateGamepadSlots` *réconcilie* les slots avec la liste
courante des gamepads (`ebiten.AppendGamepadIDs`) :

- Slot dont l'ID a disparu de la liste → libéré (= `Connected=false` →
  `NeutralJoystick`, retour au repos immédiat).
- ID nouvellement présent dans la liste, pas encore dans un slot → premier
  slot libre.
- Au-delà de 2 gamepads connectés, les supplémentaires sont **ignorés**
  jusqu'à libération d'un slot.

Cette stratégie *par réconciliation* couvre :
- Une manette **déjà connectée au démarrage** du programme (que
  `inpututil.AppendJustConnectedGamepadIDs` seul ne signalerait pas).
- Le **hot-plug** pendant l'émulation (branchement / débranchement en
  cours de partie).
- Le **mode overlay** : la réconciliation est appelée *en tête* de
  `App.Update`, donc une manette branchée pendant l'overlay est détectée
  immédiatement et son `Start` ouvre/ferme l'overlay au tick suivant.

**Bouton Start gamepad** (`StandardGamepadButtonCenterRight`) : équivalent
de `KeyEscape` côté clavier — ouvre l'overlay quand l'émulation tourne,
ferme l'overlay quand il est ouvert. Permet un usage *gamepad-only*
(reset, switch machine, etc.) sans toucher au clavier.

## 6. Permissions macOS

La première détection d'un gamepad sur macOS récent peut déclencher une
demande système « Input Monitoring ». Si l'utilisateur refuse,
`ebiten.AppendGamepadIDs` retourne une liste vide — non détectable
côté code. Si aucun gamepad n'apparaît alors qu'une manette est
branchée, vérifier *Réglages → Confidentialité et sécurité → Input
Monitoring → dcmo5* coché.

## 7. Validation et tests

- **Tests purs** (`internal/uimodel`, `internal/machine/*`, `internal/core`,
  `internal/emu`) : ancrent la convention bits sur les trois couches (clavier,
  gamepad, machine MO5, machine TO8D) avec des tables strictement identiques.
  Toute divergence d'une des quatre fait échouer la CI.
- **Test bout-en-bout CPU 6809** (`internal/emu/host_joystick_test.go`,
  J2a.T3) : programme 6809 minimal (`STA $A7CE` pour activer le mux,
  `LDA $A7CC` pour lire le joystick, `STA $2000` pour stocker en RAM)
  charge dans une vraie machine MO5, `Host.SetInput` publie un état
  joystick custom, et le test vérifie que `RAM[$2000]` contient la
  valeur publiée. Justifie tout le harness CPU de J0.
- **Validation owner** (hors CI, à l'œil) : prévue à chaque tranche
  *impure* — J3a (clavier joystick), J4b (gamepad réel branché au Mac),
  jeu réel sur cassette/disquette. Couvre les régressions visuelles, le
  timing perçu, et les pièges OS (capture des touches, permissions).

## 8. Limitations connues et travail différé

- **Configurabilité du mapping** : v1 a un mapping fixe en dur. Une
  configuration utilisateur (par machine ou globale) est reportée v2.
- **Échange J1↔J2 gamepad** : si la première manette branchée n'est pas
  celle voulue pour J1, l'utilisateur doit débrancher/rebrancher dans
  l'ordre souhaité. Un bouton « Échanger J1↔J2 » dans l'overlay est
  prévu (lot J5, non implémenté).
- **Persistance gamepad** : l'ordre d'attribution des slots est volatile
  (non persisté entre boots). Reporté v2 ; nécessitera un identifiant
  stable côté Ebitengine (GUID), non disponible nativement.
- **Touches mortes `^¨`** : non mappées en rune côté `charToTO8D`. Pour
  taper `^` isolé sur TO8D, séquence ACC + SHIFT + touche manuelle.
- **Majuscules accentuées** (`É À Ç Ù`) : non produisibles depuis le
  clavier — limitation matérielle TO8D (pas de touche dédiée).
- **Layout AZERTY autre que France-Windows** : non couvert v1. AZERTY
  belge/suisse ont des positions différentes pour quelques symboles —
  hors scope.
