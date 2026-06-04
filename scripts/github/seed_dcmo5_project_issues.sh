#!/usr/bin/env bash
set -euo pipefail

GH_BIN="${GH_BIN:-/opt/homebrew/bin/gh}"
OWNER="${OWNER:-Lesur-ai}"
REPO="${REPO:-dcmo5}"
FULL_REPO="${FULL_REPO:-${OWNER}/${REPO}}"
PROJECT_NUMBER="${PROJECT_NUMBER:-8}"
ASSIGNEE="${ASSIGNEE:-@me}"

info() {
  printf '==> %s\n' "$1"
}

die() {
  printf 'ERROR: %s\n' "$1" >&2
  exit 1
}

require_tools() {
  if [ ! -x "$GH_BIN" ]; then
    GH_BIN="$(command -v gh || true)"
  fi

  [ -n "$GH_BIN" ] && [ -x "$GH_BIN" ] || die "gh is required"
}

require_auth() {
  "$GH_BIN" auth status >/dev/null 2>&1 || {
    "$GH_BIN" auth status || true
    die "GitHub CLI authentication is not valid. Run: gh auth login -h github.com -s repo,project,workflow"
  }
}

find_issue_number_by_title() {
  title="$1"

  "$GH_BIN" issue list \
    --repo "$FULL_REPO" \
    --state all \
    --search "$title in:title" \
    --limit 100 \
    --json number,title \
    --jq "map(select(.title == \"${title}\")) | .[0].number // empty"
}

issue_url() {
  issue_number="$1"

  "$GH_BIN" issue view "$issue_number" \
    --repo "$FULL_REPO" \
    --json url \
    --jq ".url"
}

add_issue_to_project() {
  issue_number="$1"
  url="$(issue_url "$issue_number")"

  "$GH_BIN" project item-add "$PROJECT_NUMBER" \
    --owner "$OWNER" \
    --url "$url" >/dev/null 2>&1 || true
}

ensure_issue() {
  title="$1"
  milestone="$2"
  labels="$3"
  body_file="$(mktemp)"

  cat >"$body_file"

  issue_number="$(find_issue_number_by_title "$title")"
  if [ -n "$issue_number" ]; then
    info "Issue #${issue_number} already exists: ${title}"
    add_issue_to_project "$issue_number"
    rm -f "$body_file"
    return
  fi

  info "Creating issue: ${title}"
  issue_create_output="$(
    "$GH_BIN" issue create \
      --repo "$FULL_REPO" \
      --title "$title" \
      --body-file "$body_file" \
      --milestone "$milestone" \
      --label "$labels" \
      --assignee "$ASSIGNEE"
  )"
  issue_number="${issue_create_output##*/}"

  add_issue_to_project "$issue_number"
  rm -f "$body_file"
}

main() {
  require_tools
  require_auth

  ensure_issue \
    "P0 - Finaliser cadrage, licences et CI initiale" \
    "P0 - Fondations projet" \
    "phase-0,area:architecture,area:legal,area:ci,gate" <<'BODY'
Objectif:
Finaliser les fondations du portage moderne DCMO5 avant d'ouvrir les travaux d'implementation.

Travaux attendus:
- confirmer le perimetre v1 et les exclusions;
- analyser les licences de l'ancien DCMO5 v11.0 et des assets;
- definir ce qui peut etre importe, reference ou exclu;
- ajouter la CI Go initiale;
- documenter la discipline PR-only et les controles minimum.

Critere d'acceptation:
- les decisions de licence/import sont explicites;
- la CI initiale est presente ou son absence est documentee comme risque;
- le prochain travail peut demarrer sans ambiguite de repository.
BODY

  ensure_issue \
    "P1 - Initialiser le squelette Go/Ebitengine" \
    "P1 - Squelette Go/Ebitengine" \
    "phase-1,area:app,area:architecture,area:tests" <<'BODY'
Objectif:
Creer le socle executable Go + Ebitengine du nouvel emulateur.

Travaux attendus:
- initialiser le module Go;
- creer cmd/dcmo5 et les packages internes;
- ouvrir une fenetre Ebitengine redimensionnable;
- afficher un framebuffer MO5 vide;
- ajouter les premiers tests et commandes de verification.

Critere d'acceptation:
- l'application se lance localement;
- le framebuffer logique existe;
- le squelette reste portable macOS/Linux.
BODY

  ensure_issue \
    "P2 - Porter et tester le CPU Motorola 6809" \
    "P2 - CPU Motorola 6809" \
    "phase-2,area:cpu,area:tests,gate" <<'BODY'
Objectif:
Reecrire le CPU Motorola 6809 en Go deterministe.

Travaux attendus:
- modeliser registres, flags, PC, SP et modes d'adressage;
- porter les instructions prioritaires avec comptage de cycles;
- construire des tests discriminants par instruction;
- documenter les ecarts ou zones incertaines par rapport a la reference C.

Critere d'acceptation:
- les instructions implementees sont couvertes par tests;
- les cycles et flags sont verifies;
- le CPU peut etre branche au bus MO5.
BODY

  ensure_issue \
    "P3 - Implementer le coeur machine MO5" \
    "P3 - Machine MO5 core" \
    "phase-3,area:core,area:cpu,area:input,area:tests" <<'BODY'
Objectif:
Construire le coeur MO5 sans dependance UI ni chemin fichier.

Travaux attendus:
- definir Machine.Reset, Machine.Step, Machine.Framebuffer et Machine.SetKey;
- implementer bus memoire, RAM, ROM utilisateur, banques MEMO5 et ports;
- connecter clavier, joystick clavier et crayon optique cote coeur;
- ajouter tests bus/memoire et determinisme de base.

Critere d'acceptation:
- le coeur peut avancer de N cycles de facon deterministe;
- les mappings memoire et ports critiques sont testes;
- aucune dependance Ebitengine ou filesystem ne fuit dans le coeur.
BODY

  ensure_issue \
    "P4 - Porter video et entrees utilisateur" \
    "P4 - Video et entrees" \
    "phase-4,area:video,area:input,area:app" <<'BODY'
Objectif:
Brancher rendu video et entrees utilisateur dans l'application desktop.

Travaux attendus:
- produire le framebuffer logique 336x216;
- implementer scaling propre et cadence deterministe;
- mapper clavier, joystick clavier, souris et crayon;
- verifier l'absence de deformation ou de clipping sur macOS/Linux.

Critere d'acceptation:
- l'image logique est stable et inspectable;
- les entrees principales sont transmises au coeur;
- le rendu reste separe de la logique d'emulation.
BODY

  ensure_issue \
    "P5 - Porter medias, preferences et persistance" \
    "P5 - Media et persistance" \
    "phase-5,area:media,area:app,area:legal,area:tests" <<'BODY'
Objectif:
Ajouter les medias et la configuration utilisateur portable.

Travaux attendus:
- supporter les formats .k7, .fd et .rom;
- gerer l'import ROM utilisateur sans embarquer de ROM copyright;
- implementer PrinterSink fichier;
- stocker preferences et bibliotheque utilisateur dans les dossiers OS corrects.

Critere d'acceptation:
- les medias sont charges via interfaces decouplees du coeur;
- les chemins relatifs fragiles sont evites;
- les contraintes legales restent visibles dans l'UX et la documentation.
BODY

  ensure_issue \
    "P6 - Construire l'application desktop complete" \
    "P6 - Desktop complet" \
    "phase-6,area:app,area:input,area:media,area:packaging" <<'BODY'
Objectif:
Livrer une experience desktop v1 coherentement utilisable.

Travaux attendus:
- ajouter chargement medias, reset, pause, preferences et statut;
- exposer les mappings clavier/manettes;
- stabiliser les workflows utilisateur courants;
- preparer les commandes de lancement et packaging local.

Critere d'acceptation:
- un utilisateur peut configurer la ROM, charger un media et piloter l'emulateur;
- l'application ne depend pas du repertoire legacy;
- les workflows principaux sont documentes.
BODY

  ensure_issue \
    "P7 - Mettre en place la fidelity suite" \
    "P7 - Fidelity suite" \
    "phase-7,area:tests,area:core,area:video,area:audio,gate" <<'BODY'
Objectif:
Mettre en place une suite de non-regression deterministe.

Travaux attendus:
- definir les scenarios ROM + inputs;
- produire checksums RAM/framebuffer apres N frames;
- comparer les resultats utiles avec DCMO5 v11 lorsque pertinent;
- documenter les ecarts acceptes et les regressions bloquantes.

Critere d'acceptation:
- une regression coeur/video/audio est detectee automatiquement;
- les tests ne sont pas des tests de complaisance;
- les scenarios restent reproductibles sur macOS et Linux.
BODY

  ensure_issue \
    "P8 - Preparer distribution privee macOS/Linux" \
    "P8 - Distribution privee" \
    "phase-8,area:packaging,area:docs,area:legal,area:ci" <<'BODY'
Objectif:
Preparer une distribution privee testable sur macOS et Linux.

Travaux attendus:
- definir packaging macOS et Linux;
- verifier ressources, chemins et configuration utilisateur;
- documenter installation, limitations et contraintes ROM/logicielles;
- produire une checklist release privee.

Critere d'acceptation:
- les builds se lancent sur les plateformes ciblees;
- aucun payload copyright sensible n'est embarque sans validation;
- la release privee est reproductible.
BODY
}

main "$@"
