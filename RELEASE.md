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

### 4. Créer le tag

```bash
git tag v0.1.0
git push origin v0.1.0
```

Le workflow CI `.github/workflows/release.yml` se déclenche automatiquement
et produit les archives pour :
- `dcmo5-darwin-arm64.tar.gz`
- `dcmo5-darwin-amd64.tar.gz`
- `dcmo5-linux-amd64.tar.gz`

### 5. Vérification post-release

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

- ROM système MO5 non fournie (voir `DESIGN/LICENSING.md`).
- L'émulation cassette/disque/cartouche est architecturalement prête
  mais non testée sans ROM réelle.
- macOS : warning CVDisplayLink (Ebitengine v2.9.9, sans impact).
- Linux : nécessite les libs X11/GL (voir README).
