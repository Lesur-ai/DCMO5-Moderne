# DCMO5 Moderne

Portage moderne de l'émulateur Thomson MO5 [DCMO5 v11](http://dcmo5.free.fr/)
(C/SDL, 2007, © Daniel Coulom) vers **Go / Ebitengine**.

Ce projet est un logiciel libre sous licence **GNU GPL v3+**. Voir `LICENSE`
et `NOTICE`.

---

## Périmètre v1

### Fonctionnalités émulées

- Rendu vidéo MO5 (framebuffer logique 336×216, palette Thomson)
- Audio mono
- Clavier MO5 + mapping clavier hôte
- Joysticks émulés au clavier
- Crayon optique via souris
- Chargement cassette `.k7`, disquette `.fd`, cartouche MEMO5 `.rom`
- Imprimante parallèle vers fichier
- Préférences utilisateur portables macOS / Linux

### Exclusions explicites de la v1

Les extensions suivantes **ne sont pas émulées**, conformément au périmètre de
DCMO5 v11 :

- Nanoréseau Leanord
- Quick Disk Drive QD90-128
- Contrôleur IN57-001
- Contrôleur DI90-011
- Toute extension assimilée

---

## Architecture

```
cmd/dcmo5
  └── internal/app        (Ebitengine : fenêtre, input, audio, prefs)
       └── internal/core  (machine MO5 : bus, RAM/ROM, ports, timing, IRQ)
            ├── internal/cpu6809  (Motorola 6809, pur Go, sans UI)
            ├── internal/media    (cassette, disquette, cartouche, imprimante)
            └── internal/spec     (constantes matérielles, adresses, codes touches)
```

Le cœur d'émulation (`core`, `cpu6809`, `media`, `spec`) ne dépend d'aucune
bibliothèque graphique, audio ou fichier. Ebitengine est limité à la couche
application.

Voir [`DESIGN/ARCHITECTURE.md`](DESIGN/ARCHITECTURE.md) pour les décisions
structurantes.

---

## ROM et médias

> **La ROM MO5 et la ROM CD90-640 sont des contenus soumis à copyright.
> Elles ne sont jamais embarquées dans cette application.**

L'application démarre sans ROM avec un message explicite
(« ROM manquante, importer une ROM »). L'utilisateur fournit sa propre ROM via
le mécanisme d'import. Voir [`DESIGN/LICENSING.md`](DESIGN/LICENSING.md).

---

## Pré-requis

- Go 1.22+ (voir `go.mod`)
- macOS arm64 / amd64 ou Linux amd64 (cibles de premier ordre)

## Lancer l'application

```bash
go run ./cmd/dcmo5
```

## Tests

```bash
go test ./...
```

---

## Contribuer

Voir [`CONTRIBUTING.md`](CONTRIBUTING.md). Workflow PR-only — tout merge vers
`main` passe exclusivement par une Pull Request GitHub.

---

## Référence historique

Ce portage s'appuie sur DCMO5 v11 comme référence fonctionnelle et
documentaire. Le code C d'origine reste la référence ; il n'est pas une
dépendance runtime de la version moderne.
