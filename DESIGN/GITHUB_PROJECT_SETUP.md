# Template Setup GitHub Projet

> Template generique pour initialiser et piloter un repository GitHub prive.
> Date de reference : 2026-06-04.
> Objet : fournir une discipline reproductible pour tous nos projets :
> repository, labels, jalons, Project v2, Pull Requests, reviews et
> verifications de setup.

## 1. Variables a renseigner

Chaque projet doit commencer par renseigner ces variables. Elles sont le contrat
minimal entre la documentation, les commandes d'administration et les agents.

```bash
OWNER="Lesur-ai"
REPO="<repo>"
FULL_REPO="${OWNER}/${REPO}"
PROJECT_TITLE="<nom lisible du projet>"
REPO_DESCRIPTION="<description courte du projet>"
DEFAULT_BRANCH="main"
VISIBILITY="private"
```

Variables optionnelles utiles :

```bash
PROJECT_KIND="<software|documentation|infra|data|research|mixed>"
PRIMARY_STACK="<go|rails|python|typescript|rust|mixed|none>"
CI_PROFILE="<go|rails|python|node|mixed|docs|none>"
LICENSE_PROFILE="<internal|oss|mixed|sensitive>"
```

Regle : ne jamais mettre de secret, token, endpoint sensible ou identifiant
d'environnement privilegie dans ce document. Ces valeurs vivent dans les
gestionnaires de secrets, les variables d'environnement ou la configuration MCP.

## 2. Principes communs

Le repository GitHub sert de systeme de pilotage, pas seulement de stockage de
code :

- le travail integre vit sur `${DEFAULT_BRANCH}` ;
- l'integration vers `${DEFAULT_BRANCH}` se fait par Pull Request ;
- les issues portent le probleme, le contexte, les decisions et le lien Project ;
- les PR portent l'execution, les checks, les reviews et la trace `Closes #N` ;
- GitHub Projects v2 porte le statut officiel d'avancement ;
- les labels qualifient le travail, mais ne remplacent pas le champ `Status` ;
- les documents canoniques du repository priment sur la memoire ou les notes.

Pour un nouveau repository, la contrainte "PR only" peut d'abord etre une
discipline operationnelle. Elle peut ensuite etre renforcee par branch protection
ou ruleset quand la CI existe.

## 3. Creation du repository

Creation du repository prive :

```bash
gh repo create "${FULL_REPO}" \
  --private \
  --description "${REPO_DESCRIPTION}"
```

Options repository communes :

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
git push -u origin "${DEFAULT_BRANCH}"
```

Si le repository existe deja, verifier avant modification :

```bash
gh repo view "${FULL_REPO}" --json nameWithOwner,visibility,defaultBranchRef
git remote -v
```

## 4. Actions GitHub et CI

Actions doit etre active avec :

- `allowed_actions = all` ;
- `sha_pinning_required = false` ;
- `default_workflow_permissions = read` ;
- `can_approve_pull_request_reviews = false`.

Verifications :

```bash
gh api "repos/${FULL_REPO}/actions/permissions"
gh api "repos/${FULL_REPO}/actions/permissions/workflow"
```

Profil CI minimal selon `CI_PROFILE` :

| Profil | Checks minimaux |
|---|---|
| `go` | `go test ./...`, `go vet ./...`, `gofmt` |
| `rails` | tests unitaires, lint Ruby, audit securite, migrations test |
| `python` | tests, lint, type-check si applicable |
| `node` | tests, lint, type-check, build |
| `mixed` | checks par sous-projet + smoke global |
| `docs` | liens, format, build documentation si applicable |
| `none` | aucun check automatique au demarrage ; a documenter comme risque |

La CI doit etre presente avant les premieres PR d'implementation significatives.
Un projet peut demarrer sans CI seulement si le risque est explicitement note
dans une issue ou un document de cadrage.

## 5. Branches et Pull Requests

Flux nominal :

```bash
git checkout "${DEFAULT_BRANCH}"
git pull --ff-only
git checkout -b phase0/<issue>-slug
# travail + commits atomiques
git fetch origin
git rebase "origin/${DEFAULT_BRANCH}"
git push -u origin phase0/<issue>-slug
gh pr create --base "${DEFAULT_BRANCH}" --title "..." --body-file /tmp/pr-body.md
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
git checkout "${DEFAULT_BRANCH}"
git pull --ff-only
git branch -d phase0/<issue>-slug
```

## 6. Labels socle

Les labels GitHub de base peuvent etre conserves. Les labels ci-dessous forment
le socle commun a tous les projets.

### Labels de phase

| Label | Couleur | Description |
|---|---|---|
| `phase-0` | `0E8A16` | Phase 0 - cadrage, repository, architecture, CI |
| `phase-1` | `1D76DB` | Phase 1 - socle produit ou technique initial |
| `phase-2` | `5319E7` | Phase 2 - premiere tranche fonctionnelle majeure |
| `phase-3` | `0052CC` | Phase 3 - integration et stabilisation |
| `phase-4` | `8957E5` | Phase 4 - experience utilisateur ou exploitation |
| `phase-5` | `D93F0B` | Phase 5 - durcissement, gouvernance, observabilite |
| `phase-6` | `006B75` | Phase 6 - extension fonctionnelle ou scale |
| `phase-7` | `BFD4F2` | Phase 7 - compatibilite, migration, documentation avancee |
| `phase-8` | `C2E0C6` | Phase 8 - release, distribution, maintenance |

### Labels de domaine communs

| Label | Couleur | Description |
|---|---|---|
| `area:architecture` | `FBCA04` | Architecture et decisions structurantes |
| `area:product` | `FBCA04` | Produit, cadrage fonctionnel, UX |
| `area:backend` | `FBCA04` | Backend, services, logique serveur |
| `area:frontend` | `FBCA04` | Frontend, UI, experience utilisateur |
| `area:infra` | `FBCA04` | Infrastructure, runtime, deploiement |
| `area:data` | `FBCA04` | Donnees, schemas, migrations, corpus |
| `area:api` | `FBCA04` | API, contrats, compatibilite externe |
| `area:security` | `FBCA04` | Securite, confidentialite, durcissement |
| `area:ci` | `FBCA04` | CI, outillage, automatisation |
| `area:docs` | `FBCA04` | Documentation, runbooks, guides |
| `area:legal` | `FBCA04` | Licences, conformite, contraintes legales |
| `area:tests` | `FBCA04` | Tests, qualite, non-regression |

### Labels de pilotage

| Label | Couleur | Description |
|---|---|---|
| `debt` | `5319E7` | Dette technique ou documentaire |
| `gate` | `B60205` | Verification bloquante |
| `risk` | `B60205` | Risque produit, technique, legal ou operationnel |
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
  --description "Phase 0 - cadrage, repository, architecture, CI" \
  || gh label edit phase-0 --repo "${FULL_REPO}" --color 0E8A16 \
  --description "Phase 0 - cadrage, repository, architecture, CI"
```

## 7. Labels specifiques projet

Chaque projet peut ajouter des labels `area:*` specifiques. Ils doivent rester
stables et representer des domaines de responsabilite, pas des statuts.

Exemples :

| Type de projet | Labels specifiques possibles |
|---|---|
| Emulateur | `area:cpu`, `area:video`, `area:audio`, `area:input`, `area:media` |
| SaaS | `area:billing`, `area:identity`, `area:tenancy`, `area:chat` |
| Infra | `area:kubernetes`, `area:network`, `area:observability`, `area:backup` |
| Data/IA | `area:dataset`, `area:evaluation`, `area:training`, `area:inference` |
| Documentation | `area:content`, `area:publishing`, `area:taxonomy`, `area:search` |

Regle : les labels specifiques doivent etre declares dans un document projet,
par exemple `DESIGN/GITHUB_PROJECT_SETUP.<repo>.md`, ou dans une section
"Profil projet" ajoutee au setup local.

## 8. Jalons generiques

Les jalons doivent refleter les increments du projet. Le socle generique suivant
convient a la plupart des nouveaux repositories :

| Milestone | Description |
|---|---|
| `P0 - Fondations projet` | Repository, architecture, setup GitHub, CI initiale, politique de contribution. |
| `P1 - Socle executable` | Premier squelette qui se lance, structure de code, tests minimaux. |
| `P2 - Tranche fonctionnelle 1` | Premier bloc fonctionnel majeur, teste et utilisable. |
| `P3 - Tranche fonctionnelle 2` | Deuxieme bloc fonctionnel majeur ou integration structurante. |
| `P4 - Experience et workflows` | UX, workflows, ergonomie, operations courantes. |
| `P5 - Qualite et gouvernance` | Tests, securite, observabilite, dette, documentation technique. |
| `P6 - Extension ou scale` | Extension fonctionnelle, performance, multi-tenant, volumetrie ou compatibilite. |
| `P7 - Stabilisation` | Non-regression, migrations, compatibilite, corrections de bord. |
| `P8 - Release et maintenance` | Release, distribution, runbooks, support, maintenance. |

Commande type :

```bash
gh api -X POST "repos/${FULL_REPO}/milestones" \
  -f title="P0 - Fondations projet" \
  -f description="Repository, architecture, setup GitHub, CI initiale, politique de contribution."
```

Un projet peut remplacer les descriptions generiques par des descriptions
metier, mais il doit conserver une progression lisible de `P0` a `P8`.

## 9. Project v2 principal

Projet principal cible :

| Parametre | Valeur |
|---|---|
| Owner | `${OWNER}` |
| Titre | `${PROJECT_TITLE}` |
| Visibilite | privee par defaut |

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

Vues souhaitees :

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

## 10. Cycle de vie des issues

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

## 11. Review et auto-review

Le standard de review du projet est un commentaire PR, pas `gh pr review`.

Format attendu :

```text
LGTM

Reviewed-Head: <sha>

Checks pris en compte:
- <check 1>: pass
- <check 2>: pass
- <check 3>: pass

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

## 12. Verification du setup

Checklist rapide :

```bash
gh repo view "${FULL_REPO}" --json nameWithOwner,visibility,defaultBranchRef
gh api "repos/${FULL_REPO}/actions/permissions"
gh api "repos/${FULL_REPO}/actions/permissions/workflow"
gh label list --repo "${FULL_REPO}" --limit 200
gh api "repos/${FULL_REPO}/milestones?state=all&per_page=100"
gh project list --owner "${OWNER}" --format json
gh api "repos/${FULL_REPO}/branches/${DEFAULT_BRANCH}/protection"
gh api "repos/${FULL_REPO}/rulesets"
```

Les deux dernieres verifications peuvent retourner respectivement
`Branch not protected` et une liste vide de rulesets au demarrage. Si c'est le
cas, la discipline PR-only reste une regle operationnelle jusqu'a mise en place
des protections.

## 13. Adaptation par projet

Avant d'executer ce template sur un nouveau projet, produire un court bloc de
profil projet :

```text
Projet:
- OWNER:
- REPO:
- PROJECT_TITLE:
- REPO_DESCRIPTION:
- PROJECT_KIND:
- PRIMARY_STACK:
- CI_PROFILE:
- LICENSE_PROFILE:

Labels specifiques:
- ...

Jalons specialises:
- ...

Risques initiaux:
- ...
```

Ce bloc peut vivre dans le README, dans `DESIGN/ARCHITECTURE.md`, ou dans un
fichier `DESIGN/GITHUB_PROJECT_SETUP.<repo>.md` si le projet necessite des
ecarts importants au socle commun.
