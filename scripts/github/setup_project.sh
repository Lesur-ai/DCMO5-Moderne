#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CONFIG_FILE="${1:-}"
DRY_RUN="${DRY_RUN:-0}"
GH_BIN="${GH_BIN:-/opt/homebrew/bin/gh}"

COMMON_LABELS=(
  "phase-0|0E8A16|Phase 0 - cadrage, repository, architecture, CI"
  "phase-1|1D76DB|Phase 1 - socle produit ou technique initial"
  "phase-2|5319E7|Phase 2 - premiere tranche fonctionnelle majeure"
  "phase-3|0052CC|Phase 3 - integration et stabilisation"
  "phase-4|8957E5|Phase 4 - experience utilisateur ou exploitation"
  "phase-5|D93F0B|Phase 5 - durcissement, gouvernance, observabilite"
  "phase-6|006B75|Phase 6 - extension fonctionnelle ou scale"
  "phase-7|BFD4F2|Phase 7 - compatibilite, migration, documentation avancee"
  "phase-8|C2E0C6|Phase 8 - release, distribution, maintenance"
  "area:architecture|FBCA04|Architecture et decisions structurantes"
  "area:product|FBCA04|Produit, cadrage fonctionnel, UX"
  "area:backend|FBCA04|Backend, services, logique serveur"
  "area:frontend|FBCA04|Frontend, UI, experience utilisateur"
  "area:infra|FBCA04|Infrastructure, runtime, deploiement"
  "area:data|FBCA04|Donnees, schemas, migrations, corpus"
  "area:api|FBCA04|API, contrats, compatibilite externe"
  "area:security|FBCA04|Securite, confidentialite, durcissement"
  "area:ci|FBCA04|CI, outillage, automatisation"
  "area:docs|FBCA04|Documentation, runbooks, guides"
  "area:legal|FBCA04|Licences, conformite, contraintes legales"
  "area:tests|FBCA04|Tests, qualite, non-regression"
  "debt|5319E7|Dette technique ou documentaire"
  "gate|B60205|Verification bloquante"
  "risk|B60205|Risque produit, technique, legal ou operationnel"
  "status:in-progress|FBCA04|Label informatif seulement, ne remplace pas Project Status"
  "opus|6F42C1|Issue a traiter par un modele Opus"
  "sonnet|030E98|Issue a traiter par un modele Sonnet"
  "gpt5-5|B60205|Issue a traiter par un modele GPT-5.5"
  "gpt_5-5-pro|B60205|Modele recommande : GPT-5.5 Pro"
  "opus_4-8|6F42C1|Modele recommande : Opus 4.8"
  "sonnet_4-6|030E98|Modele recommande : Sonnet 4.6"
)

DEFAULT_MILESTONES=(
  "P0 - Fondations projet|Repository, architecture, setup GitHub, CI initiale, politique de contribution."
  "P1 - Socle executable|Premier squelette qui se lance, structure de code, tests minimaux."
  "P2 - Tranche fonctionnelle 1|Premier bloc fonctionnel majeur, teste et utilisable."
  "P3 - Tranche fonctionnelle 2|Deuxieme bloc fonctionnel majeur ou integration structurante."
  "P4 - Experience et workflows|UX, workflows, ergonomie, operations courantes."
  "P5 - Qualite et gouvernance|Tests, securite, observabilite, dette, documentation technique."
  "P6 - Extension ou scale|Extension fonctionnelle, performance, multi-tenant, volumetrie ou compatibilite."
  "P7 - Stabilisation|Non-regression, migrations, compatibilite, corrections de bord."
  "P8 - Release et maintenance|Release, distribution, runbooks, support, maintenance."
)

PROJECT_FIELDS=(
  "Priority|SINGLE_SELECT|P0,P1,P2,P3"
  "Size|SINGLE_SELECT|XS,S,M,L,XL"
  "Estimate|NUMBER|"
  "Start date|DATE|"
  "Target date|DATE|"
)

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

usage() {
  cat <<'USAGE'
Usage:
  scripts/github/setup_project.sh <config-file>

Environment:
  GH_BIN=/path/to/gh      Override GitHub CLI path.
  DRY_RUN=1              Print planned setup without network writes.

Config files are shell profiles defining OWNER, REPO, PROJECT_TITLE,
REPO_DESCRIPTION, DEFAULT_BRANCH, VISIBILITY and optional PROJECT_LABELS,
MILESTONES, PROJECT_FIELDS, ENABLE_PROJECT.
USAGE
}

run() {
  if [ "$DRY_RUN" = "1" ]; then
    printf 'DRY-RUN:' >&2
    printf ' %q' "$@" >&2
    printf '\n' >&2
    return
  fi

  "$@"
}

require_config_file() {
  if [ -z "$CONFIG_FILE" ]; then
    usage
    die "Missing config file argument"
  fi

  [ -f "$CONFIG_FILE" ] || die "Config file not found: ${CONFIG_FILE}"
}

load_config() {
  # shellcheck source=/dev/null
  source "$CONFIG_FILE"

  : "${OWNER:?OWNER is required}"
  : "${REPO:?REPO is required}"
  : "${PROJECT_TITLE:?PROJECT_TITLE is required}"
  : "${REPO_DESCRIPTION:?REPO_DESCRIPTION is required}"
  : "${DEFAULT_BRANCH:?DEFAULT_BRANCH is required}"
  : "${VISIBILITY:?VISIBILITY is required}"

  FULL_REPO="${FULL_REPO:-${OWNER}/${REPO}}"
  ENABLE_PROJECT="${ENABLE_PROJECT:-1}"

  if ! declare -p PROJECT_LABELS >/dev/null 2>&1; then
    PROJECT_LABELS=()
  fi

  if ! declare -p MILESTONES >/dev/null 2>&1; then
    MILESTONES=("${DEFAULT_MILESTONES[@]}")
  fi
}

require_tools() {
  command -v git >/dev/null 2>&1 || die "git is required"

  if [ ! -x "$GH_BIN" ]; then
    GH_BIN="$(command -v gh || true)"
  fi

  [ -n "$GH_BIN" ] && [ -x "$GH_BIN" ] || die "gh is required"
}

require_auth() {
  if [ "$DRY_RUN" = "1" ]; then
    info "Skipping GitHub auth check in dry-run mode"
    return
  fi

  "$GH_BIN" auth status >/dev/null 2>&1 || {
    "$GH_BIN" auth status || true
    die "GitHub CLI authentication is not valid. Run: gh auth login -h github.com -s repo,project,workflow"
  }
}

ensure_repo() {
  if [ "$DRY_RUN" != "1" ] && "$GH_BIN" repo view "$FULL_REPO" >/dev/null 2>&1; then
    info "Repository ${FULL_REPO} already exists"
    return
  fi

  info "Creating ${VISIBILITY} repository ${FULL_REPO}"
  case "$VISIBILITY" in
    private)
      run "$GH_BIN" repo create "$FULL_REPO" --private --description "$REPO_DESCRIPTION"
      ;;
    public)
      run "$GH_BIN" repo create "$FULL_REPO" --public --description "$REPO_DESCRIPTION"
      ;;
    internal)
      run "$GH_BIN" repo create "$FULL_REPO" --internal --description "$REPO_DESCRIPTION"
      ;;
    *)
      die "Unsupported VISIBILITY=${VISIBILITY}"
      ;;
  esac
}

configure_repo() {
  info "Configuring repository options"
  run "$GH_BIN" api -X PATCH "repos/${FULL_REPO}" \
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
  run "$GH_BIN" api -X PUT "repos/${FULL_REPO}/actions/permissions" \
    -F enabled=true \
    -f allowed_actions=all >/dev/null

  run "$GH_BIN" api -X PUT "repos/${FULL_REPO}/actions/permissions/workflow" \
    -f default_workflow_permissions=read \
    -F can_approve_pull_request_reviews=false >/dev/null
}

upsert_label() {
  label_name="$1"
  label_color="$2"
  label_description="$3"

  if [ "$DRY_RUN" = "1" ]; then
    info "Would upsert label ${label_name}"
    return
  fi

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
  for label_spec in "${COMMON_LABELS[@]}" "${PROJECT_LABELS[@]}"; do
    [ -n "$label_spec" ] || continue
    IFS='|' read -r label_name label_color label_description <<<"$label_spec"
    upsert_label "$label_name" "$label_color" "$label_description"
  done
}

milestone_exists() {
  milestone_title="$1"
  "$GH_BIN" api "repos/${FULL_REPO}/milestones?state=all&per_page=100" \
    --jq ".[].title" | grep -Fx "$milestone_title" >/dev/null
}

create_milestone() {
  milestone_title="$1"
  milestone_description="$2"

  if [ "$DRY_RUN" = "1" ]; then
    info "Would ensure milestone ${milestone_title}"
    return
  fi

  if milestone_exists "$milestone_title"; then
    return
  fi

  "$GH_BIN" api -X POST "repos/${FULL_REPO}/milestones" \
    -f title="$milestone_title" \
    -f description="$milestone_description" >/dev/null
}

configure_milestones() {
  info "Creating milestones"
  if [ ${#MILESTONES[@]} -eq 0 ]; then
    MILESTONES=("${DEFAULT_MILESTONES[@]}")
  fi

  for milestone_spec in "${MILESTONES[@]}"; do
    [ -n "$milestone_spec" ] || continue
    IFS='|' read -r milestone_title milestone_description <<<"$milestone_spec"
    create_milestone "$milestone_title" "$milestone_description"
  done
}

ensure_project() {
  if [ "$ENABLE_PROJECT" != "1" ]; then
    info "Project v2 disabled by config"
    return
  fi

  if [ "$DRY_RUN" = "1" ]; then
    info "Would ensure Project v2 '${PROJECT_TITLE}'"
    return
  fi

  if ! "$GH_BIN" project list --owner "$OWNER" --format json >/dev/null 2>&1; then
    warn "Cannot list GitHub Projects for ${OWNER}; check that gh auth has the project scope."
    return
  fi

  project_number="$(project_number_for_title)"

  if [ -n "$project_number" ]; then
    info "Project v2 '${PROJECT_TITLE}' already exists as #${project_number}"
    return
  fi

  info "Creating Project v2 '${PROJECT_TITLE}'"
  "$GH_BIN" project create --owner "$OWNER" --title "$PROJECT_TITLE" >/dev/null
}

project_number_for_title() {
  "$GH_BIN" project list --owner "$OWNER" --format json \
    --jq ".projects[] | select(.title == \"${PROJECT_TITLE}\") | .number" | head -n 1
}

project_field_exists() {
  project_number="$1"
  field_name="$2"

  "$GH_BIN" project field-list "$project_number" --owner "$OWNER" --format json \
    --jq ".fields[].name" | grep -Fx "$field_name" >/dev/null
}

create_project_field_if_missing() {
  project_number="$1"
  field_name="$2"
  data_type="$3"
  options="${4:-}"

  if project_field_exists "$project_number" "$field_name"; then
    return
  fi

  if [ -n "$options" ]; then
    "$GH_BIN" project field-create "$project_number" \
      --owner "$OWNER" \
      --name "$field_name" \
      --data-type "$data_type" \
      --single-select-options "$options" >/dev/null
    return
  fi

  "$GH_BIN" project field-create "$project_number" \
    --owner "$OWNER" \
    --name "$field_name" \
    --data-type "$data_type" >/dev/null
}

configure_project() {
  if [ "$ENABLE_PROJECT" != "1" ]; then
    return
  fi

  if [ "$DRY_RUN" = "1" ]; then
    info "Would configure Project v2 fields and repository link"
    warn "GitHub CLI cannot create Project views or Iteration fields directly."
    return
  fi

  project_number="$(project_number_for_title)"
  if [ -z "$project_number" ]; then
    warn "Project v2 '${PROJECT_TITLE}' not found; skipping project field setup."
    return
  fi

  info "Configuring Project v2 #${project_number}"
  "$GH_BIN" project edit "$project_number" \
    --owner "$OWNER" \
    --visibility PRIVATE \
    --description "$REPO_DESCRIPTION" >/dev/null

  "$GH_BIN" project link "$project_number" \
    --owner "$OWNER" \
    --repo "$FULL_REPO" >/dev/null 2>&1 || true

  for field_spec in "${PROJECT_FIELDS[@]}"; do
    [ -n "$field_spec" ] || continue
    IFS='|' read -r field_name data_type options <<<"$field_spec"
    create_project_field_if_missing "$project_number" "$field_name" "$data_type" "$options"
  done

  warn "GitHub CLI cannot create Project views or Iteration fields directly; configure them in the GitHub UI if required."
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
    run git remote add origin "$remote_url"
  fi

  current_branch="$(git branch --show-current)"
  [ "$current_branch" = "$DEFAULT_BRANCH" ] || die "Current branch is ${current_branch}, expected ${DEFAULT_BRANCH}"

  info "Pushing ${DEFAULT_BRANCH} to origin"
  run git push -u origin "$DEFAULT_BRANCH"
}

verify_setup() {
  if [ "$DRY_RUN" = "1" ]; then
    info "Skipping remote verification in dry-run mode"
    return
  fi

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
  require_config_file
  load_config
  require_tools
  require_auth
  ensure_repo
  configure_repo
  configure_labels
  configure_milestones
  ensure_project
  configure_project
  ensure_remote_and_push
  verify_setup
}

main "$@"
