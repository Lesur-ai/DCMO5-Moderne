# Profil Setup GitHub - DCMO5

> Profil projet appliquant le template generique
> `DESIGN/GITHUB_PROJECT_SETUP.md` au portage moderne de DCMO5.
> Date de reference : 2026-06-04.

## Variables projet

```bash
OWNER="Lesur-ai"
REPO="dcmo5"
FULL_REPO="${OWNER}/${REPO}"
PROJECT_TITLE="DCMO5 Modern Port"
REPO_DESCRIPTION="Modern Go/Ebitengine port of the DCMO5 Thomson MO5 emulator for macOS and Linux."
DEFAULT_BRANCH="main"
VISIBILITY="private"
PROJECT_KIND="software"
PRIMARY_STACK="go"
CI_PROFILE="go"
LICENSE_PROFILE="sensitive"
```

## Positionnement

DCMO5 est un portage moderne de l'ancien emulateur DCMO5 v11.0. Le nouveau
projet vise une application desktop macOS et Linux en Go + Ebitengine, avec un
coeur d'emulation reecrit et teste. L'ancien code C reste une reference de
lecture, pas une fondation runtime permanente.

## Labels specifiques DCMO5

| Label | Couleur | Description |
|---|---|---|
| `area:core` | `FBCA04` | Coeur machine MO5, orchestration cycles et etat deterministe |
| `area:cpu` | `FBCA04` | CPU Motorola 6809, instructions, flags et cycles |
| `area:video` | `FBCA04` | Memoire video, framebuffer, rendu et cadence |
| `area:audio` | `FBCA04` | Generation sonore, buffer audio et synchronisation |
| `area:input` | `FBCA04` | Clavier, joystick clavier, souris et crayon optique |
| `area:media` | `FBCA04` | Cassette, disquette, cartouche ROM et imprimante fichier |
| `area:app` | `FBCA04` | Application Ebitengine, menus, preferences et UX desktop |
| `area:packaging` | `FBCA04` | Packaging macOS/Linux, ressources et distribution privee |

## Jalons specialises

| Milestone | Description |
|---|---|
| `P0 - Fondations projet` | Repository prive, architecture, setup GitHub, CI Go initiale, politique de contribution et garde-fous licence. |
| `P1 - Squelette Go/Ebitengine` | Module Go, application `cmd/dcmo5`, packages internes et fenetre affichant un framebuffer MO5 vide redimensionnable. |
| `P2 - CPU Motorola 6809` | CPU 6809 pur Go, instructions, flags, registres, adressages et comptage de cycles testes. |
| `P3 - Machine MO5 core` | Bus memoire, RAM, ROM utilisateur, banques MEMO5, ports, clavier, joystick et crayon dans un coeur sans UI. |
| `P4 - Video et entrees` | Framebuffer logique 336x216, rendu Ebitengine, scaling propre, cadence deterministe et mapping input. |
| `P5 - Media et persistance` | Support `.k7`, `.fd`, `.rom`, imprimante fichier, preferences utilisateur et chemins macOS/Linux portables. |
| `P6 - Desktop complet` | Chargement medias, reset, pause, preferences, mapping clavier/manettes, statut et workflows utilisateur v1. |
| `P7 - Fidelity suite` | Tests deterministes ROM + inputs, checksums RAM/framebuffer et comparaison fonctionnelle avec DCMO5 v11. |
| `P8 - Distribution privee` | Packaging macOS/Linux, verification ressources, absence de ROM/logicielles copyrightes embarques et documentation release. |

## Risques initiaux

- Ne pas embarquer les ROM MO5, CD90-640 ou logiciels MO5 copyrightes sans
  validation explicite.
- La fidelite CPU 6809 et le comptage de cycles sont le risque technique
  principal : les tests doivent etre discriminants, pas seulement structurels.
- L'audio et la video doivent etre decouples de la boucle UI pour rester
  portables et deterministes.
- Le vieux repertoire `dcmo5v11.0/` reste non tracke tant que l'audit
  licence/import n'est pas tranche.

## Execution

Le setup GitHub DCMO5 s'execute avec :

```bash
scripts/github/setup_dcmo5.sh
```

Pre-requis :

- `gh` authentifie avec les scopes necessaires au repository et a GitHub
  Projects ;
- un remote local absent ou compatible avec `Lesur-ai/dcmo5` ;
- branche locale `main` prete a etre poussee.

Limite outillage au 2026-06-04 : `gh project field-create` permet de creer
les champs `Priority`, `Size`, `Estimate`, `Start date` et `Target date`, mais
ne permet pas de creer directement les vues Project v2 ni un champ
`Iteration`. Ces elements restent a configurer dans l'interface GitHub si le
projet en a besoin immediatement.
