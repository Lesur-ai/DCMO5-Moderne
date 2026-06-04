# Setup GitHub du projet DCMO5

> Etat cible pour le repository GitHub du portage moderne de DCMO5.
> Date : 2026-06-04.
> Objet : fournir un setup reproductible pour un repository prive, ses labels,
> ses jalons, son Project v2 et sa discipline de Pull Request.

## 1. Variables du projet

Les commandes ci-dessous utilisent les variables suivantes :

```bash
OWNER="Lesur-ai"
REPO="dcmo5"
FULL_REPO="${OWNER}/${REPO}"
PROJECT_TITLE="DCMO5 modern port"
```

Le repository doit rester prive tant que les sujets de licence ROM/logiciels MO5
et la direction produit ne sont pas stabilises.

## 2. Principes de travail

Le repository GitHub sert de systeme de pilotage :

- le code vit sur `main` ;
- l'integration vers `main` se fait par Pull Request ;
- les issues portent le probleme, le contexte, la decision d'attaque et le lien
  vers le Project ;
- les PR portent l'execution, les checks, les reviews et la trace `Closes #N` ;
- GitHub Projects v2 porte le statut officiel d'avancement ;
- les labels de statut ne remplacent pas le champ `Status` du Project.

Pour le demarrage, la contrainte "PR only" peut etre une discipline
operationnelle avant d'etre verrouillee par branch protection ou ruleset.

## 3. Creation du repository prive

Creation :

```bash
gh repo create "${FULL_REPO}" \
  --private \
  --description "DCMO5 modern port — Thomson MO5 emulator rewritten in Go/Ebitengine for macOS and Linux"
```

Options repository :

```bash
gh api -X PATCH "repos/${FULL_REPO}" \
  -f has_issues=true \
  -f has_projects=true \
  -f has_wiki=true \
  -f has_discussions=false \
  -f allow_squash_merge=true \
  -f allow_merge_commit=true \
  -f allow_rebase_merge=true \
  -f allow_auto_merge=false \
  -f delete_branch_on_merge=false \
  -f allow_update_branch=false
```

Remote local :

```bash
git remote add origin "https://github.com/${FULL_REPO}.git"
git push -u origin main
```

## 4. Actions GitHub

Actions doit etre active avec :

- `allowed_actions = all` ;
- `sha_pinning_required = false` ;
- `default_workflow_permissions = read` ;
- `can_approve_pull_request_reviews = false`.

Verification :

```bash
gh api "repos/${FULL_REPO}/actions/permissions"
gh api "repos/${FULL_REPO}/actions/permissions/workflow"
```

Le repo ne doit pas stocker de secrets Actions au demarrage. Les ROM, logiciels
MO5, tokens et chemins locaux ne doivent jamais etre commits.

CI cible initiale :

- `go test ./...` ;
- `go vet ./...` ;
- verification formatting `gofmt` ;
- verification que les payloads sensibles restent absents du repo ;
- build macOS/Linux quand le module Go existe.

La CI doit etre presente avant les premieres PR d'implementation significatives.

## 5. Branches et Pull Requests

Flux nominal :

```bash
git checkout main
git pull --ff-only
git checkout -b phase0/<issue>-slug
# travail + commits atomiques
git fetch origin
git rebase origin/main
git push -u origin phase0/<issue>-slug
gh pr create --base main --title "..." --body-file /tmp/pr-body.md
```

Le body de toute PR qui resout une issue doit contenir en premiere ligne :

```text
Closes #<N>
```

Verification apres creation :

```bash
gh issue view <N> --repo "${FULL_REPO}" --json closedByPullRequestsReferences
```

La PR est mergee sur GitHub uniquement. Apres merge GitHub :

```bash
git checkout main
git pull --ff-only
git branch -d phase0/<issue>-slug
```

## 6. Labels

Les labels de base GitHub peuvent etre conserves. Les labels projet suivants
doivent etre crees ou mis a jour de maniere idempotente.

### Labels de phase

| Label | Couleur | Description |
|---|---|---|
| `phase-0` | `0E8A16` | Phase 0 - cadrage, architecture, repository, CI |
| `phase-1` | `1D76DB` | Phase 1 - squelette Go/Ebitengine et packaging minimal |
| `phase-2` | `5319E7` | Phase 2 - CPU Motorola 6809 |
| `phase-3` | `0052CC` | Phase 3 - bus MO5, memoire, ROM, ports |
| `phase-4` | `8957E5` | Phase 4 - video, clavier, joystick, crayon |
| `phase-5` | `D93F0B` | Phase 5 - media k7/fd/rom, imprimante, stockage |
| `phase-6` | `006B75` | Phase 6 - application desktop complete |
| `phase-7` | `BFD4F2` | Phase 7 - fidelite, compatibilite, regression suite |
| `phase-8` | `C2E0C6` | Phase 8 - distribution, durcissement, documentation |

### Labels de domaine

| Label | Couleur | Description |
|---|---|---|
| `area:architecture` | `FBCA04` | Domaine : architecture et decisions structurantes |
| `area:cpu6809` | `FBCA04` | Domaine : emulation CPU Motorola 6809 |
| `area:core` | `FBCA04` | Domaine : machine MO5, bus, memoire, ports |
| `area:video` | `FBCA04` | Domaine : rendu video et framebuffer |
| `area:audio` | `FBCA04` | Domaine : audio et cadence |
| `area:input` | `FBCA04` | Domaine : clavier, joystick, souris/crayon |
| `area:media` | `FBCA04` | Domaine : k7, fd, rom, imprimante |
| `area:app` | `FBCA04` | Domaine : application desktop Ebitengine |
| `area:packaging` | `FBCA04` | Domaine : packaging macOS/Linux |
| `area:ci` | `FBCA04` | Domaine : CI et outillage |
| `area:docs` | `FBCA04` | Domaine : documentation |
| `area:legal` | `FBCA04` | Domaine : licences, ROM, logiciels MO5 |
| `area:tests` | `FBCA04` | Domaine : tests et golden data |

### Labels de pilotage

| Label | Couleur | Description |
|---|---|---|
| `debt` | `5319E7` | Dette technique |
| `gate` | `B60205` | Verification bloquante |
| `risk` | `B60205` | Risque produit, technique ou legal |
| `status:in-progress` | `FBCA04` | Label informatif seulement, ne remplace pas Project Status |

### Labels de routage modele

| Label | Couleur | Description |
|---|---|---|
| `opus` | `6F42C1` | Issue a traiter par un modele Opus |
| `sonnet` | `030E98` | Issue a traiter par un modele Sonnet |
| `gpt5-5` | `B60205` | Issue a traiter par un modele GPT-5.5 |
| `gpt_5-5-pro` | `B60205` | Modele recommande : GPT-5.5 Pro |
| `opus_4-8` | `6F42C1` | Modele recommande : Opus 4.8 |
| `sonnet_4-6` | `030E98` | Modele recommande : Sonnet 4.6 |

Commande type idempotente :

```bash
gh label create phase-0 --repo "${FULL_REPO}" --color 0E8A16 \
  --description "Phase 0 - cadrage, architecture, repository, CI" \
  || gh label edit phase-0 --repo "${FULL_REPO}" --color 0E8A16 \
  --description "Phase 0 - cadrage, architecture, repository, CI"
```

## 7. Jalons

Les jalons structurent le portage par increments testables.

| Milestone | Description |
|---|---|
| `P0 - Fondations projet` | Repository prive, architecture, setup GitHub, CI initiale, politique licences/ROM. |
| `P1 - Squelette Go/Ebitengine` | Module Go, structure packages, fenetre Ebitengine minimale, etat ROM manquante explicite. |
| `P2 - CPU Motorola 6809` | Portage CPU deterministe, registres uint8/uint16, opcodes prioritaires, tests flags/cycles. |
| `P3 - Machine MO5 core` | Bus memoire, RAM/ROM, banques MEMO5, ports, reset, IRQ, scheduler de cycles. |
| `P4 - Video et entrees` | Palette, framebuffer 336x216, clavier, joysticks clavier, crayon optique souris. |
| `P5 - Media et persistence` | Cassette k7, disquette fd, cartouche rom, imprimante fichier, config utilisateur portable. |
| `P6 - Desktop complet` | Menus, preferences, chargement fichiers, audio bufferise, packaging macOS/Linux. |
| `P7 - Fidelity suite` | Golden tests, checksums deterministes, corpus de programmes autorises, corrections timing. |
| `P8 - Distribution privee` | Release privee, documentation utilisateur, procedure d'import ROM, durcissement final. |

Commande type :

```bash
gh api -X POST "repos/${FULL_REPO}/milestones" \
  -f title="P0 - Fondations projet" \
  -f description="Repository prive, architecture, setup GitHub, CI initiale, politique licences/ROM."
```

## 8. Project v2 principal

Projet principal cible :

| Parametre | Valeur |
|---|---|
| Owner | `Lesur-ai` |
| Titre | `DCMO5 modern port` |
| Visibilite | privee |

Creation :

```bash
gh project create --owner "${OWNER}" --title "${PROJECT_TITLE}"
```

Champs souhaites :

| Champ | Type / options |
|---|---|
| `Status` | single select : `Todo`, `Ready`, `In progress`, `Done` |
| `Priority` | single select : `P0`, `P1`, `P2`, `P3` |
| `Size` | single select : `XS`, `S`, `M`, `L`, `XL` |
| `Estimate` | number |
| `Iteration` | iteration 14 jours |
| `Start date` | date |
| `Target date` | date |

Vues souhaites :

| Vue | Layout |
|---|---|
| `Backlog` | table |
| `Board` | board |
| `Current iteration` | board |
| `Roadmap` | roadmap |
| `My items` | table |

Selon les capacites de la version de `gh`, la creation exacte des champs et vues
peut necessiter l'API GraphQL Projects v2 ou l'interface GitHub.

Verification :

```bash
gh project list --owner "${OWNER}" --format json
gh project field-list <project-number> --owner "${OWNER}" --format json
```

## 9. Cycle de vie des issues

Au demarrage d'une issue :

```bash
gh issue edit <N> --repo "${FULL_REPO}" --add-assignee "@me"
```

Ensuite, passer l'item Project a `In progress` via l'API Projects v2. Ne pas
utiliser le label `status:in-progress` comme source de verite.

Les decisions de conception avant PR vont en commentaire d'issue :

```bash
gh issue comment <N> --repo "${FULL_REPO}" --body "Decision: ..."
```

Apres ouverture de la PR, les discussions de revue basculent dans la PR.

## 10. Review et auto-review

Le standard de review du projet est un commentaire PR, pas `gh pr review`.

Format attendu :

```text
LGTM

Reviewed-Head: <sha>

Checks pris en compte:
- go test: pass
- go vet: pass
- gofmt: pass
- packaging smoke: pass

Limites:
- ...

Findings:
- Aucun finding bloquant.
```

S'il existe un finding bloquant, le commentaire commence par :

```text
changements demandes
```

Avant toute demande de merge, verifier que le dernier `Reviewed-Head` correspond
au `headRefOid` actuel de la PR :

```bash
gh pr view <PR> --repo "${FULL_REPO}" --json headRefOid
gh pr view <PR> --repo "${FULL_REPO}" --comments
```

## 11. Verification du setup

Checklist rapide :

```bash
gh repo view "${FULL_REPO}" --json nameWithOwner,visibility,defaultBranchRef
gh api "repos/${FULL_REPO}/actions/permissions"
gh api "repos/${FULL_REPO}/actions/permissions/workflow"
gh label list --repo "${FULL_REPO}" --limit 200
gh api "repos/${FULL_REPO}/milestones?state=all&per_page=100"
gh project list --owner "${OWNER}" --format json
gh api "repos/${FULL_REPO}/branches/main/protection"
gh api "repos/${FULL_REPO}/rulesets"
```

Les deux dernieres verifications peuvent retourner respectivement
`Branch not protected` et une liste vide de rulesets au demarrage. Si c'est le
cas, la discipline PR-only reste une regle operationnelle jusqu'a mise en place
des protections.
