# Note de setup GitHub — Portal

> Etat observe le 2026-06-04 sur `Lesur-ai/portal`.
> Objectif : permettre a un autre agent de reproduire le setup GitHub initial sur
> un nouveau repository, avec la meme discipline de travail et les memes objets
> GitHub structurants.

## 1. Principes de setup

Le repository GitHub n'est pas seulement un depot de code. Pour Portal, il sert
de systeme de pilotage :

- le code vit sur `main`, mais l'integration vers `main` se fait uniquement via
  Pull Request GitHub ;
- les issues portent le probleme, les decisions d'attaque et le lien vers le
  Project ;
- les PR portent l'execution, les checks, les reviews et la trace `Closes #N` ;
- GitHub Projects v2 porte le statut d'avancement, pas les labels.

Point important : au moment de cette note, `main` n'a ni branch protection
native, ni ruleset GitHub. La contrainte "PR only" est donc une discipline
operationnelle documentee dans `AGENTS.md`, pas un verrou GitHub configure.

## 2. Creation du repository

Parametres observes sur `Lesur-ai/portal` :

| Parametre | Valeur |
|---|---|
| Organisation | `Lesur-ai` |
| Repository | `portal` |
| Visibilite | `PRIVATE` |
| Branche par defaut | `main` |
| Description | `Portal Lesur.AI — Rails frontend & API for chat.lesur.ai (SaaS souverain, multi-tenant, chat + Transkryptor)` |
| Homepage | vide |
| Topics | aucun |
| Issues | activees |
| Projects | actifs |
| Wiki | actif |
| Discussions | desactivees |

Commande type pour un nouveau projet :

```bash
gh repo create Lesur-ai/<repo> \
  --private \
  --description "Portal Lesur.AI — Rails frontend & API for chat.lesur.ai (SaaS souverain, multi-tenant, chat + Transkryptor)"
```

Puis appliquer les options repo :

```bash
gh api -X PATCH repos/Lesur-ai/<repo> \
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

## 3. Actions GitHub

Actions est active avec :

- `allowed_actions = all` ;
- `sha_pinning_required = false` ;
- `default_workflow_permissions = read` ;
- `can_approve_pull_request_reviews = false`.

Commandes de verification :

```bash
gh api repos/Lesur-ai/<repo>/actions/permissions
gh api repos/Lesur-ai/<repo>/actions/permissions/workflow
```

Le repo ne definit aucun secret ou variable Actions au niveau repository.

La CI est dans `.github/workflows/ci.yml` et se declenche sur :

- `push` vers `main` ;
- toute `pull_request`.

Jobs declares :

| Job | Role |
|---|---|
| `lint` | Ruby setup + `bundle exec rubocop -f github` |
| `security` | Brakeman + bundler-audit |
| `test` | PostgreSQL 14, tests JS Node 22, `db:test:prepare`, smoke, gates P2/P4/P5/P6, puis RSpec complet |
| `sidecar` | Python 3.12, install `chat-engine-svc`, Ruff, gate P3, pytest complet, build Docker sidecar |

La CI doit etre presente avant d'ouvrir les premieres PR d'implementation.

## 4. Branches et PR

Flux nominal :

```bash
git checkout main
git pull --ff-only
git checkout -b phaseX/<issue>-slug
# travail + commits atomiques
git fetch origin
git rebase origin/main
git push -u origin phaseX/<issue>-slug
gh pr create --base main --title "..." --body-file /tmp/pr-body.md
```

Le body de toute PR qui resout une issue doit contenir en premiere ligne :

```text
Closes #<N>
```

Verification apres creation :

```bash
gh issue view <N> --repo Lesur-ai/<repo> --json closedByPullRequestsReferences
```

La PR est mergee sur GitHub uniquement, jamais par `git merge` local sur
`main`. Apres merge GitHub :

```bash
git checkout main
git pull --ff-only
git branch -d phaseX/<issue>-slug
```

## 5. Labels

Les labels de base GitHub sont conserves, puis les labels projet suivants sont
ajoutes.

Labels de phase :

| Label | Couleur | Description |
|---|---|---|
| `phase-0` | `0E8A16` | Phase 0 — fondations & scaffold |
| `phase-1` | `1D76DB` | Phase 1 — identite, tenancy & isolation |
| `phase-2` | `5319E7` | Phase 2 — billing & metrage durable |
| `phase-3` | `0052CC` | Phase 3 — sidecar chat-engine (Cloud Temple) |
| `phase-4` | `8957E5` | Phase 4 — Chat MVP (streaming/persistance/metrage) |
| `phase-5` | `D93F0B` | Phase 5 — tooling, MCP, gouvernance & BYOK |
| `phase-6` | `006B75` | Phase 6 — compte, confidentialite, preferences |
| `phase-7` | `BFD4F2` | Phase 7 — Transkryptor & API mobile |
| `phase-8` | `C2E0C6` | Phase 8 — durcissement & scale |

Labels de domaine :

| Label | Couleur | Description |
|---|---|---|
| `area:bootstrap` | `FBCA04` | Domaine : bootstrap |
| `area:identity` | `FBCA04` | Domaine : identity |
| `area:tenancy` | `FBCA04` | Domaine : tenancy |
| `area:billing` | `FBCA04` | Domaine : billing |
| `area:inference` | `FBCA04` | Domaine : inference |
| `area:chat` | `FBCA04` | Domaine : chat |
| `area:tooling` | `FBCA04` | Domaine : tooling |
| `area:memory` | `FBCA04` | Domaine : memory |
| `area:transkryptor` | `FBCA04` | Domaine : transkryptor |
| `area:platform` | `FBCA04` | Domaine : platform |
| `area:api` | `FBCA04` | Domaine : api |
| `area:ci` | `FBCA04` | Domaine : ci |
| `area:security` | `FBCA04` | Domaine : security |
| `area:tests` | `FBCA04` | Domaine : tests |

Labels de pilotage :

| Label | Couleur | Description |
|---|---|---|
| `debt` | `5319E7` | Dette technique |
| `gate` | `B60205` | Verification bloquante (gate de phase) |
| `status:in-progress` | `FBCA04` | Travail en cours |

`status:in-progress` existe encore comme label, mais ne doit pas representer le
statut officiel d'une issue. Le statut officiel est le champ `Status` du Project
v2.

Labels de routage modele :

| Label | Couleur | Description |
|---|---|---|
| `opus` | `6F42C1` | Issue a traiter par un modele Opus |
| `sonnet` | `030E98` | Issue a traiter par un modele Sonnet |
| `gpt5-5` | `B60205` | Issue a traiter par un modele GPT-5.5 |
| `gpt_5-5-pro` | `B60205` | Modele recommande : GPT-5.5 Pro |
| `opus_4-8` | `6F42C1` | Modele recommande : Opus 4.8 |
| `sonnet_4-6` | `030E98` | Modele recommande : Sonnet 4.6 |

Exemple de creation idempotente :

```bash
gh label create phase-4 --repo Lesur-ai/<repo> --color 8957E5 \
  --description "Phase 4 — Chat MVP (streaming/persistance/metrage)" \
  || gh label edit phase-4 --repo Lesur-ai/<repo> --color 8957E5 \
  --description "Phase 4 — Chat MVP (streaming/persistance/metrage)"
```

## 6. Milestones

Milestones observes :

| Milestone | Description |
|---|---|
| `P2 — Billing & métrage durable` | Facturation & metrage durable AVANT le chat (Plan, Subscription/Stripe, Entitlement, UsageRecord, Outbox, Rollup, quotas, ecrans 13/10). Ref. archi §10 (P2), §5.2, §3.4. |
| `P3 — Sidecar chat-engine` | Sidecar FastAPI (chat-engine[http]), Inference:: + seed Cloud Temple, usage durable, mTLS+JWT, default-deny, lib/chat_engine_client. Ref. archi §10 (P3), §3, §5.4. |
| `P4 — Chat MVP` | Chat conversationnel: Project/Conversation/Message/Run, gateway SSE (tx courtes), metrage reconcilie, ephemere, ecrans 02-06. Ref. archi §10 (P4), §7, §8.1. |
| `P5 — Tooling, MCP, gouvernance & BYOK complet` | Tooling, MCP, gouvernance, BYOK complet, capabilities, audit, deferred tools et Memory bindings. Ref. archi §10 P5, §5.4, §5.5, §5.6. |
| `P6 — Compte, confidentialité, préférences, recherche` | Compte, confidentialite, preferences, recherche, export/suppression RGPD et OpenAPI account/privacy. Ref. archi §10 P6. |

Exemple :

```bash
gh api -X POST repos/Lesur-ai/<repo>/milestones \
  -f title="P4 — Chat MVP" \
  -f description="Chat conversationnel: Project/Conversation/Message/Run, gateway SSE (tx courtes), metrage reconcilie, ephemere, ecrans 02-06. Ref. archi §10 (P4), §7, §8.1."
```

## 7. Project v2 principal

Projet principal observe :

| Parametre | Valeur |
|---|---|
| Owner | `Lesur-ai` |
| Numero | `7` |
| Titre | `Portal chat.lesur.ai build` |
| Visibilite | prive |
| URL | `https://github.com/orgs/Lesur-ai/projects/7` |
| Items observes | 150 |

Champs :

| Champ | Type / options |
|---|---|
| `Title` | champ systeme |
| `Assignees` | champ systeme |
| `Status` | single select : `Todo`, `Ready`, `In progress`, `Done` |
| `Labels` | champ systeme |
| `Linked pull requests` | champ systeme |
| `Milestone` | champ systeme |
| `Repository` | champ systeme |
| `Reviewers` | champ systeme |
| `Parent issue` | champ systeme |
| `Sub-issues progress` | champ systeme |
| `Created`, `Updated`, `Closed` | champs systeme |
| `Priority` | single select, sans option configuree observee |
| `Size` | single select : `XS`, `S`, `M`, `L`, `XL` |
| `Estimate` | number |
| `Iteration` | iterations de 14 jours, premiere iteration le 2026-05-28 |
| `Start date` | date |
| `Target date` | date |

Vues :

| Vue | Layout |
|---|---|
| `Backlog` | table |
| `Board` | board |
| `Current iteration` | board |
| `Roadmap` | roadmap |
| `My items` | table |

Creation :

```bash
gh project create --owner Lesur-ai --title "Portal chat.lesur.ai build"
```

La creation exacte des champs single-select et des vues peut necessiter l'API
GraphQL Projects v2 ou l'interface GitHub selon les capacites de la version de
`gh`. Une fois cree, recuperer les IDs :

```bash
gh project field-list <project-number> --owner Lesur-ai --format json
```

## 8. Cycle de vie des issues

Au demarrage d'une issue :

```bash
gh issue edit <N> --repo Lesur-ai/<repo> --add-assignee "@me"
```

Ensuite, passer l'item Project a `In progress` via l'API Projects v2. Ne pas
utiliser le label `status:in-progress` pour cela.

Mutation type :

```bash
gh api graphql -f query='
mutation(
  $projectId: ID!
  $itemId: ID!
  $statusFieldId: ID!
  $inProgressOptionId: String!
) {
  updateProjectV2ItemFieldValue(input: {
    projectId: $projectId
    itemId: $itemId
    fieldId: $statusFieldId
    value: { singleSelectOptionId: $inProgressOptionId }
  }) {
    projectV2Item { id }
  }
}' \
  -F projectId="$PROJECT_ID" \
  -F itemId="$PROJECT_ITEM_ID" \
  -F statusFieldId="$STATUS_FIELD_ID" \
  -F inProgressOptionId="$IN_PROGRESS_OPTION_ID"
```

Les decisions de conception avant PR vont en commentaire d'issue :

```bash
gh issue comment <N> --repo Lesur-ai/<repo> --body "Decision: ..."
```

Apres ouverture de la PR, les discussions de revue basculent dans la PR.

## 9. Review et auto-review

Le standard de review du projet est un commentaire PR, pas `gh pr review`.

Format attendu :

```text
LGTM

Reviewed-Head: <sha>

Checks pris en compte:
- lint: pass
- security: pass
- sidecar: pass
- test: pass

Limites:
- ...

Findings:
- Aucun finding bloquant.
```

S'il existe un finding bloquant, le commentaire commence par
`changements demandés`.

Avant toute demande de merge, verifier que le dernier `Reviewed-Head` correspond
au `headRefOid` actuel de la PR :

```bash
gh pr view <PR> --repo Lesur-ai/<repo> --json headRefOid
gh pr view <PR> --repo Lesur-ai/<repo> --comments
```

## 10. Verification du setup

Checklist rapide pour un nouveau repo :

```bash
gh repo view Lesur-ai/<repo> --json nameWithOwner,visibility,defaultBranchRef
gh api repos/Lesur-ai/<repo>/actions/permissions
gh api repos/Lesur-ai/<repo>/actions/permissions/workflow
gh label list --repo Lesur-ai/<repo> --limit 200
gh api 'repos/Lesur-ai/<repo>/milestones?state=all&per_page=100'
gh project list --owner Lesur-ai --format json
gh project field-list <project-number> --owner Lesur-ai --format json
gh api repos/Lesur-ai/<repo>/branches/main/protection
gh api repos/Lesur-ai/<repo>/rulesets
```

Sur `Lesur-ai/portal`, les deux dernieres verifications retournent
respectivement `Branch not protected` et une liste vide de rulesets.

