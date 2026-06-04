# Contribuer à DCMO5 Moderne

## Workflow Git (PR-only, IMPÉRATIF)

Tous les merges vers `main` se font **exclusivement via Pull Request GitHub**.
Aucun merge local sur `main`.

### Cycle nominal

```bash
# 1. Repartir d'un main propre
git checkout main && git pull --ff-only

# 2. Créer la branche feature
git checkout -b phaseX/N-slug-issue

# 3. Travailler, commiter atomiquement
git add <fichiers> && git commit -m "..."

# 4. Avant PR : rebase sur main à jour
git fetch origin && git rebase origin/main

# 5. Pousser + ouvrir la PR
git push -u origin phaseX/N-slug-issue
gh pr create --base main --title "..." --body "Closes #N ..."

# 6. Après merge : nettoyer
git checkout main && git pull --ff-only
git branch -d phaseX/N-slug-issue
```

### Conventions de branche

```
phaseX/N-slug-court
```

- `X` = numéro de phase (0–8)
- `N` = numéro d'issue GitHub
- `slug` = 2–4 mots en minuscules avec tirets

Exemples : `phase0/12-contributing`, `phase2/3-cpu6809-decode`.

### Commits

- Commits atomiques : 1 commit = 1 changement cohérent.
- Message : ligne de titre < 72 caractères, impératif (« Ajoute », « Corrige »,
  « Refactorise »).
- Pas de commits WIP/fixup poussés sur la branche finale (rebase interactif
  local avant push si nécessaire).

## Lien PR ↔ Issue (IMPÉRATIF)

Toute PR qui résout une issue **doit** contenir un mot-clé de fermeture
GitHub dans le **body** (pas le titre) :

```
Closes #N
```

Mots-clés acceptés : `close`, `closes`, `closed`, `fix`, `fixes`, `fixed`,
`resolve`, `resolves`, `resolved` (insensibles à la casse).

Pour une dépendance sans fermeture : `Refs #N` ou `Related to #N`.

**Vérification post-création :**

```bash
gh issue view <N> --json closedByPullRequestsReferences
# Le champ doit contenir la PR. S'il est vide, le mot-clé est absent.
```

## Mode de merge

**Squash merge** uniquement. 1 commit sur `main` = 1 issue résolue.
La branche est supprimée après merge (`--delete-branch`).

## Commandes interdites par défaut

| Commande | Raison |
|---|---|
| `git merge` sur `main` local | Divergence et historique corrompu |
| `git push --force` sur `main` | Perte de commits partagés |
| `git commit` directement sur `main` | Bypass du workflow PR-only |

## Review de code

Chaque PR passe une review **Codex** (`codex exec review --base main`) avant
merge. Les findings sont publiés dans la PR. Les points valides doivent être
corrigés ou explicitement motivés avant merge.

## Contrôles CI

La CI (GitHub Actions, configurée dans P0.4) doit être **verte** avant merge.
Elle couvre : `gofmt`, `go vet`, build, tests — sur macOS et Linux.

## Garde-fous assets (IMPÉRATIF)

- **Ne jamais commiter** de ROM MO5, ROM CD90-640, logiciels MO5 commerciaux
  (`.k7`, `.fd`, `.rom` commerciaux).
- **Ne jamais embarquer** de fichier binaire copyright sans validation
  explicite.
- Les données de test doivent être libres ou générées.

Voir `DESIGN/LICENSING.md` pour la classification complète.

## Signaler un bug ou proposer une évolution

Ouvrir une issue GitHub avec le template approprié.
