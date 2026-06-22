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

### 2.1 Dependances runtime introduites

| Dependance | Version | Licence | Compat GPLv3+ | Note |
|---|---|---|---|---|
| `github.com/hajimehoshi/ebiten/v2` | v2.9.9 | Apache-2.0 | oui | Moteur 2D. |
| `github.com/ebitenui/ebitenui` | v0.7.3 | MIT | oui | IHM (lot #117). 100 % Go, sans cgo. |
| `github.com/frustra/bbcode` | transitive (figee go.sum) | MIT | oui | Transitive d'ebitenui (texte enrichi). Pas de release taguee : maintenance faible, a surveiller. |
| `golang.org/x/exp` | transitive (figee go.sum) | BSD-3 | oui | Transitive d'ebitenui. **Experimentale, sans promesse de compatibilite Go 1** — risque a documenter (suivi conformite). |
| `golang.org/x/sync` | v0.20.0 | BSD-3 | oui | Upgrade transitif (0.17.0 -> 0.20.0). |

Toutes ces dependances sont pures Go : la cible Windows compile en
`CGO_ENABLED=0` (garde-fou CI). Licences MIT / Apache-2.0 / BSD-3, compatibles
GPLv3+. Empreinte verrouillee dans `go.sum` (`go mod verify`).

## 3. Classification des assets

> **Mise a jour (2026-06-06).** Decision du mainteneur : `rom/` et `software/`
> sont desormais **inclus dans le depot** pour rendre l'emulateur utilisable
> immediatement. Voir l'appreciation de redistribuabilite et la procedure de
> retrait ci-dessous (§3.1).

| Categorie | Exemples | Statut | Regle |
|---|---|---|---|
| **Inclus (sous reserve)** | ROM MO5 (`rom/mo5-v1.1.rom`), ROM CD90-640 (`rom/cd90-640.rom`), logiciels MO5 historiques (`software/*.k7`, `*.fd`, `*.rom`) | Materiel/logiciels Thomson MO5 (1984+), preservation communautaire | **Versionnes** sous l'appreciation du §3.1. Retrait sur demande d'un ayant droit. |
| **Reference** | Code C `dcmo5v11.0/source`, documentation historique | GPLv3+ (D. Coulom) | Consultable comme reference. L'arborescence `dcmo5v11.0/` reste hors versioning (`.gitignore`) jusqu'a decision explicite. |
| **Produit par le projet** | Donnees de test generees, ROM factices de test | Libre / produit par le projet | Autorise sans reserve. |

### 3.1 ROM et logiciels MO5 — appreciation et retrait

- **Provenance.** Materiel et ecosysteme Thomson MO5 (commercialise en 1984) ;
  contenus issus de la communaute de preservation/emulation, notamment la
  distribution [DCMO5 v11](http://dcmo5.free.fr/) de Daniel Coulom.
- **Appreciation.** Compte tenu de l'anciennete du materiel (obsolete depuis des
  decennies) et de la diffusion etablie de ces contenus a des fins de
  preservation, le mainteneur les considere comme raisonnablement
  redistribuables et les inclut dans le depot.
- **Reserve explicite.** Cette appreciation **n'est pas un avis juridique** et
  **n'affirme pas** un statut de domaine public etabli. Le statut exact de
  certains titres `software/` (logiciels potentiellement commerciaux) peut
  rester incertain ; ils sont inclus sous la meme reserve.
- **Procedure de retrait.** Tout ayant droit souhaitant le retrait d'un contenu
  peut **ouvrir une issue** sur le depot ; le contenu sera retire sans delai.

## 4. Garde-fous (IMPERATIF)

1. Le **code moderne** reste sous GPLv3+ ; les dependances tierces doivent rester
   compatibles GPLv3+.
2. Les ROM/logiciels inclus le sont **sous l'appreciation et la reserve du §3.1**,
   avec procedure de retrait sur demande des ayants droit.
3. L'application sait demarrer **sans ROM** (etat explicite « ROM manquante »)
   et accepte l'**import utilisateur** de ROM et medias.
4. Aucun asset n'est **embarque en dur dans le binaire** : les ROM/medias sont
   charges depuis des fichiers (le binaire ne contient pas les octets ROM).
5. `dcmo5v11.0/` (arborescence de reference C) reste **non suivi par Git**
   (`.gitignore`) jusqu'a une decision explicite distincte.

## 5. References

- `LICENSE` — texte GPLv3 integral.
- `NOTICE` — attribution et contenus exclus.
- `DESIGN/ARCHITECTURE.md` — section « Ressources et droits ».
- Licence historique : `dcmo5v11.0/licence/dcmo5v11-licence.txt` (hors repo).
