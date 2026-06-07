# Checklist release privée — DCMO5 Moderne

> Référence : DESIGN/LICENSING.md pour les contraintes légales.

## Avant de créer un tag de release

### 1. Tests

```bash
# Tous les tests
go test ./...

# Tests fidelity spécifiquement
go test ./internal/core/... -run TestFidelity -v
```

Tous les tests doivent être verts.

### 2. Vérification assets (IMPÉRATIF)

```bash
# Aucune ROM ni logiciel MO5 copyright dans le repo
git ls-files | grep -E '\.(rom|k7|fd)$'
# → doit retourner vide
```

Vérifier aussi manuellement :
- [ ] Aucun fichier `.rom`, `.k7`, `.fd` committé
- [ ] `testdata/` ne contient que des fichiers générés
- [ ] `dcmo5v11.0/` est listé dans `.gitignore`

### 3. Build local de vérification

```bash
# macOS arm64 (natif)
GOOS=darwin GOARCH=arm64 go build ./cmd/dcmo5

# macOS amd64 (natif ou cross depuis arm64)
GOOS=darwin GOARCH=amd64 go build ./cmd/dcmo5
```

> **Linux :** Ebitengine requiert CGO (GLFW) et ne se compile pas en cross-compile
> simple depuis macOS (`CGO_ENABLED=0` échoue sur les symboles OpenGL).
> La vérification du build Linux se fait via la CI GitHub Actions ou sur un
> environnement Linux natif avec les libs X11/GL installées (voir README).

### 4. Mettre à jour version et changelog

- [ ] `VERSION` reflète la version à publier (ex. `1.0.0`).
- [ ] `CHANGELOG.md` a une section datée pour cette version (Ajouté / Changé /
  Corrigé / Limites connues) et le lien `[x.y.z]` en bas pointe vers le tag.

### 5. Créer le tag

Le tag DOIT correspondre au contenu de `VERSION`, préfixé par `v` :

```bash
git tag v1.0.0
git push origin v1.0.0
```

Le workflow CI `.github/workflows/release.yml` se déclenche automatiquement
et produit les archives pour :
- `dcmo5-darwin-arm64.tar.gz`
- `dcmo5-darwin-amd64.tar.gz`
- `dcmo5-linux-amd64.tar.gz`

### 6. Vérification post-release

- [ ] La release GitHub contient les 3 archives.
- [ ] Les archives se lancent sur chaque plateforme cible.
- [ ] Le message "ROM manquante" s'affiche correctement sans ROM.
- [ ] L'application accepte `-rom` sans crash.

## Installation depuis une archive

```bash
tar xzf dcmo5-darwin-arm64.tar.gz
./dcmo5-darwin-arm64 -rom /chemin/vers/mo5.rom
```

## Limites connues v1

- **Crayon optique** : la fonction BASIC `PEN(...)` ne suit pas la souris. La
  routine bas niveau (trap `0x4B`) est émulée, mais le BASIC dérive la position
  d'un handshake matériel du crayon optique non émulé — **conforme à dcmo5 v11**
  (qui ne fait pas suivre la souris à `PEN` non plus). Cf. issue #86.
- ROM système MO5 / contrôleur CD90-640 et logiciels MO5 inclus dans le dépôt
  sous réserve (voir `DESIGN/LICENSING.md` §3.1 : provenance, appréciation,
  procédure de retrait).
- Cassette `.k7`, disquette `.fd` (densité variable + DOS CD90-640) et cartouche
  MEMO5 sont fonctionnelles sur ROM réelle (alignement trap via patch ROM en
  mémoire, cf. `internal/core/rompatch.go`).
- macOS : warning CVDisplayLink (Ebitengine v2.9.9, sans impact).
- Linux : nécessite les libs X11/GL (voir README).
