# Licences et politique d'assets — DCMO5 Moderne

> Statut : decision de cadrage P0 (issue #10).
> Date : 2026-06-05.
> Perimetre : droits sur le code, les bibliotheques et les assets du portage
> moderne, et garde-fous d'embedding avant distribution.

## 1. Licence du code moderne

Le code Go de DCMO5 Moderne est publie sous **GNU General Public License
version 3 ou ulterieure (GPLv3+)**. Le texte complet est dans `LICENSE`.

### Justification

Le portage s'appuie sur le code C de DCMO5 v11 comme reference fonctionnelle
et documentaire (logique d'emulation, mappings memoire, comportement CPU). Il
constitue donc une **oeuvre derivee** au sens du droit d'auteur, et non une
reecriture *clean-room* independante.

DCMO5 v11 etant distribue sous GPLv3+, la GPL impose que les oeuvres derivees
distribuees restent sous GPLv3+. Une licence permissive (MIT, Apache-2.0) ou
proprietaire serait non conforme tant que le code derive du C GPL.

Ce choix est compatible avec l'objectif de distribution privee (P8) :
en usage interne non distribue, la GPL n'impose aucune publication ; des qu'il
y a distribution, le projet reste conforme.

### Attribution

- Auteur du portage moderne : **Christophe Lesur** (`NOTICE`).
- Le copyright d'origine de **Daniel Coulom** (DCMO5 v11, 2007) est conserve
  dans `NOTICE`.

## 2. Bibliotheques tierces

| Composant historique | Licence | Decision portage |
|---|---|---|
| SDL / SDL_ttf | LGPL 2.1+ | **Abandonne.** Remplace par Ebitengine. Aucune contrainte heritee. |
| Police Bitstream Vera | Permissive (redistribuable, modifiable si renommee) | **Non reprise.** Le rendu texte passe par la couche application moderne. |

Les dependances Go runtime devront etre verifiees pour compatibilite GPLv3+
au fur et a mesure de leur introduction (Ebitengine est sous Apache-2.0,
compatible GPLv3+).

## 3. Classification des assets

| Categorie | Exemples | Statut | Regle |
|---|---|---|---|
| **Exclu** | ROM MO5, ROM CD90-640, logiciels MO5 commerciaux (`software/*.k7`, `*.fd`, `*.rom`) | Copyright tiers | **Jamais embarque, jamais commite, jamais distribue.** Import utilisateur uniquement. |
| **Reference** | Code C `dcmo5v11.0/source`, documentation historique | GPLv3+ (D. Coulom) | Consultable comme reference. L'arborescence `dcmo5v11.0/` reste hors versioning (`.gitignore`) jusqu'a decision explicite. |
| **Importable** | Donnees de test libres ou **generees** dans le repo moderne | Libre / produit par le projet | Autorise. Aucune capture issue d'un asset copyright. |

## 4. Garde-fous (IMPERATIF)

1. **Aucune ROM ni logiciel MO5 copyright** n'est embarque dans l'application
   ou ses paquets sans validation ecrite des ayants droit.
2. L'application demarre **sans ROM** avec un etat explicite
   (« ROM manquante, importer une ROM ») — cf. `DESIGN/ARCHITECTURE.md`.
3. Un **mecanisme d'import utilisateur** fournit ROM et medias ; les chemins
   attendus sont documentes.
4. Les **donnees de test** du repo moderne sont libres ou generees ; aucune ne
   derive d'un asset copyright.
5. `dcmo5v11.0/` (code, ROM, software) reste **non suivi par Git** tant qu'une
   decision explicite d'import (avec verification de licence) n'a pas ete prise.

## 5. References

- `LICENSE` — texte GPLv3 integral.
- `NOTICE` — attribution et contenus exclus.
- `DESIGN/ARCHITECTURE.md` — section « Ressources et droits ».
- Licence historique : `dcmo5v11.0/licence/dcmo5v11-licence.txt` (hors repo).
