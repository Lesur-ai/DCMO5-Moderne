#!/usr/bin/env bash
set -euo pipefail

GH_BIN="${GH_BIN:-/opt/homebrew/bin/gh}"
OWNER="${OWNER:-Lesur-ai}"
REPO="${REPO:-dcmo5}"
FULL_REPO="${OWNER}/${REPO}"
PROJECT_TITLE="${PROJECT_TITLE:-DCMO5 Modern Port}"
REPO_DESCRIPTION="${REPO_DESCRIPTION:-Modern Go/Ebitengine port of the DCMO5 Thomson MO5 emulator for macOS and Linux.}"
DEFAULT_BRANCH="${DEFAULT_BRANCH:-main}"
VISIBILITY="${VISIBILITY:-private}"

info() {
  printf '==> %s\n' "$1"
}

warn() {
  printf 'WARN: %s\n' "$1" >&2
}

die() {
  printf 'ERROR: %s\n' "$1" >&2
  exit 1
}

require_tools() {
  command -v git >/dev/null 2>&1 || die "git is required"

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

ensure_repo() {
  if "$GH_BIN" repo view "$FULL_REPO" >/dev/null 2>&1; then
    info "Repository ${FULL_REPO} already exists"
    return
  fi

  info "Creating ${VISIBILITY} repository ${FULL_REPO}"
  case "$VISIBILITY" in
    private)
      "$GH_BIN" repo create "$FULL_REPO" --private --description "$REPO_DESCRIPTION"
      ;;
    public)
      "$GH_BIN" repo create "$FULL_REPO" --public --description "$REPO_DESCRIPTION"
      ;;
    internal)
      "$GH_BIN" repo create "$FULL_REPO" --internal --description "$REPO_DESCRIPTION"
      ;;
    *)
      die "Unsupported VISIBILITY=${VISIBILITY}"
      ;;
  esac
}

configure_repo() {
  info "Configuring repository options"
  "$GH_BIN" api -X PATCH "repos/${FULL_REPO}" \
    -F has_issues=true \
    -F has_projects=true \
    -F has_wiki=true \
    -F has_discussions=false \
    -F allow_squash_merge=true \
    -F allow_merge_commit=true \
    -F allow_rebase_merge=true \
    -F allow_auto_merge=false \
    -F delete_branch_on_merge=false \
    -F allow_update_branch=false >/dev/null

  info "Configuring Actions permissions"
  "$GH_BIN" api -X PUT "repos/${FULL_REPO}/actions/permissions" \
    -F enabled=true \
    -f allowed_actions=all >/dev/null

  "$GH_BIN" api -X PUT "repos/${FULL_REPO}/actions/permissions/workflow" \
    -f default_workflow_permissions=read \
    -F can_approve_pull_request_reviews=false >/dev/null
}

upsert_label() {
  label_name="$1"
  label_color="$2"
  label_description="$3"

  if "$GH_BIN" label create "$label_name" \
    --repo "$FULL_REPO" \
    --color "$label_color" \
    --description "$label_description" >/dev/null 2>&1; then
    return
  fi

  "$GH_BIN" label edit "$label_name" \
    --repo "$FULL_REPO" \
    --color "$label_color" \
    --description "$label_description" >/dev/null
}

configure_labels() {
  info "Creating or updating labels"
  while IFS='|' read -r label_name label_color label_description; do
    [ -n "$label_name" ] || continue
    upsert_label "$label_name" "$label_color" "$label_description"
  done <<'LABELS'
phase-0|0E8A16|Phase 0 - cadrage, repository, architecture, CI
phase-1|1D76DB|Phase 1 - socle produit ou technique initial
phase-2|5319E7|Phase 2 - premiere tranche fonctionnelle majeure
phase-3|0052CC|Phase 3 - integration et stabilisation
phase-4|8957E5|Phase 4 - experience utilisateur ou exploitation
phase-5|D93F0B|Phase 5 - durcissement, gouvernance, observabilite
phase-6|006B75|Phase 6 - extension fonctionnelle ou scale
phase-7|BFD4F2|Phase 7 - compatibilite, migration, documentation avancee
phase-8|C2E0C6|Phase 8 - release, distribution, maintenance
area:architecture|FBCA04|Architecture et decisions structurantes
area:product|FBCA04|Produit, cadrage fonctionnel, UX
area:backend|FBCA04|Backend, services, logique serveur
area:frontend|FBCA04|Frontend, UI, experience utilisateur
area:infra|FBCA04|Infrastructure, runtime, deploiement
area:data|FBCA04|Donnees, schemas, migrations, corpus
area:api|FBCA04|API, contrats, compatibilite externe
area:security|FBCA04|Securite, confidentialite, durcissement
area:ci|FBCA04|CI, outillage, automatisation
area:docs|FBCA04|Documentation, runbooks, guides
area:legal|FBCA04|Licences, conformite, contraintes legales
area:tests|FBCA04|Tests, qualite, non-regression
area:core|FBCA04|Coeur machine MO5, orchestration cycles et etat deterministe
area:cpu|FBCA04|CPU Motorola 6809, instructions, flags et cycles
area:video|FBCA04|Memoire video, framebuffer, rendu et cadence
area:audio|FBCA04|Generation sonore, buffer audio et synchronisation
area:input|FBCA04|Clavier, joystick clavier, souris et crayon optique
area:media|FBCA04|Cassette, disquette, cartouche ROM et imprimante fichier
area:app|FBCA04|Application Ebitengine, menus, preferences et UX desktop
area:packaging|FBCA04|Packaging macOS/Linux, ressources et distribution privee
debt|5319E7|Dette technique ou documentaire
gate|B60205|Verification bloquante
risk|B60205|Risque produit, technique, legal ou operationnel
status:in-progress|FBCA04|Label informatif seulement, ne remplace pas Project Status
opus|6F42C1|Issue a traiter par un modele Opus
sonnet|030E98|Issue a traiter par un modele Sonnet
gpt5-5|B60205|Issue a traiter par un modele GPT-5.5
gpt_5-5-pro|B60205|Modele recommande : GPT-5.5 Pro
opus_4-8|6F42C1|Modele recommande : Opus 4.8
sonnet_4-6|030E98|Modele recommande : Sonnet 4.6
LABELS
}

milestone_exists() {
  milestone_title="$1"
  "$GH_BIN" api "repos/${FULL_REPO}/milestones?state=all&per_page=100" \
    --jq ".[].title" | grep -Fx "$milestone_title" >/dev/null
}

create_milestone() {
  milestone_title="$1"
  milestone_description="$2"

  if milestone_exists "$milestone_title"; then
    return
  fi

  "$GH_BIN" api -X POST "repos/${FULL_REPO}/milestones" \
    -f title="$milestone_title" \
    -f description="$milestone_description" >/dev/null
}

configure_milestones() {
  info "Creating milestones"
  while IFS='|' read -r milestone_title milestone_description; do
    [ -n "$milestone_title" ] || continue
    create_milestone "$milestone_title" "$milestone_description"
  done <<'MILESTONES'
P0 - Fondations projet|Repository prive, architecture, setup GitHub, CI Go initiale, politique de contribution et garde-fous licence.
P1 - Squelette Go/Ebitengine|Module Go, application cmd/dcmo5, packages internes et fenetre affichant un framebuffer MO5 vide redimensionnable.
P2 - CPU Motorola 6809|CPU 6809 pur Go, instructions, flags, registres, adressages et comptage de cycles testes.
P3 - Machine MO5 core|Bus memoire, RAM, ROM utilisateur, banques MEMO5, ports, clavier, joystick et crayon dans un coeur sans UI.
P4 - Video et entrees|Framebuffer logique 336x216, rendu Ebitengine, scaling propre, cadence deterministe et mapping input.
P5 - Media et persistance|Support .k7, .fd, .rom, imprimante fichier, preferences utilisateur et chemins macOS/Linux portables.
P6 - Desktop complet|Chargement medias, reset, pause, preferences, mapping clavier/manettes, statut et workflows utilisateur v1.
P7 - Fidelity suite|Tests deterministes ROM + inputs, checksums RAM/framebuffer et comparaison fonctionnelle avec DCMO5 v11.
P8 - Distribution privee|Packaging macOS/Linux, verification ressources, absence de ROM/logicielles copyrightes embarques et documentation release.
MILESTONES
}

ensure_project() {
  if ! "$GH_BIN" project list --owner "$OWNER" --format json >/dev/null 2>&1; then
    warn "Cannot list GitHub Projects for ${OWNER}; check that gh auth has the project scope."
    return
  fi

  project_number="$("$GH_BIN" project list --owner "$OWNER" --format json \
    --jq ".projects[] | select(.title == \"${PROJECT_TITLE}\") | .number" | head -n 1)"

  if [ -n "$project_number" ]; then
    info "Project v2 '${PROJECT_TITLE}' already exists as #${project_number}"
    return
  fi

  info "Creating Project v2 '${PROJECT_TITLE}'"
  "$GH_BIN" project create --owner "$OWNER" --title "$PROJECT_TITLE" >/dev/null
}

ensure_remote_and_push() {
  remote_url="https://github.com/${FULL_REPO}.git"

  if git remote get-url origin >/dev/null 2>&1; then
    current_remote="$(git remote get-url origin)"
    if [ "$current_remote" != "$remote_url" ]; then
      die "origin already points to ${current_remote}, expected ${remote_url}"
    fi
  else
    info "Adding origin ${remote_url}"
    git remote add origin "$remote_url"
  fi

  current_branch="$(git branch --show-current)"
  [ "$current_branch" = "$DEFAULT_BRANCH" ] || die "Current branch is ${current_branch}, expected ${DEFAULT_BRANCH}"

  info "Pushing ${DEFAULT_BRANCH} to origin"
  git push -u origin "$DEFAULT_BRANCH"
}

verify_setup() {
  info "Repository"
  "$GH_BIN" repo view "$FULL_REPO" --json nameWithOwner,visibility,defaultBranchRef

  info "Actions permissions"
  "$GH_BIN" api "repos/${FULL_REPO}/actions/permissions"
  "$GH_BIN" api "repos/${FULL_REPO}/actions/permissions/workflow"

  info "Milestones"
  "$GH_BIN" api "repos/${FULL_REPO}/milestones?state=all&per_page=100" \
    --jq ".[].title"

  info "Project list"
  "$GH_BIN" project list --owner "$OWNER" --format json || true
}

main() {
  require_tools
  require_auth
  ensure_repo
  configure_repo
  configure_labels
  configure_milestones
  ensure_project
  ensure_remote_and_push
  verify_setup
}

main "$@"
