# IMP-06 — Seuils ARCEP lisibles par l'API (volet référentiel)

| Champ | Valeur |
|---|---|
| Dépôt | `mobiletest` |
| Issue | [yele-bf/mobilefront#12](https://github.com/yele-bf/mobilefront/issues/12) (item porté côté application) |
| Priorité / vague | P2 / V5 Dashboard & référentiel |
| Owner au classeur | NIKIEMA Abdoul Fatahou / NONGUIERMA WENDINSOO LEONEL |
| Branche | `feat/IMP-06-seuils-arcep` |
| Validation imposée | Local (jeux de valeurs autour des seuils) |

## Constat et impact

Le classeur donne IMP-06 sur `mobilefront` **et** `mobiletest` : l'application
ne peut ancrer ses scores sur le référentiel ARCEP que si l'API expose les
mêmes seuils qu'elle.

Or, sur `main`, le module ARCEP n'est plus présent : ni `config/seuils.json`,
ni `results/arcep.go`, ni les routes `/api/arcep/*`. Le référentiel validé
(version 3) n'existe que sur une branche non fusionnée, alors que le backend
Docker en service a bien été construit à partir de cet état. Résultat : aucun
moyen, depuis `main`, de lire les seuils ARCEP qui ont servi à valider l'item.

## Correction

Cette branche prépare le volet référentiel sans réintroduire tout le module :

- `results/arcep_qos.go` : types du bloc « qos » de `config/seuils.json`
  (navigation : 30 s / 95 % / connexion 20 s / 95 % ; streaming : 5 s / 95 % ;
  définition Yélé : rebuffering ≤ 10 %, résolution ≥ 480p, 0 interruption),
  décodeur `ParseArcepQosSeuils` et validation.
- `results/arcep_qos_test.go` : chargement du référentiel v3, refus d'un bloc
  `qos` absent, refus d'un seuil nul, acceptation de la borne 100 %.

Choix non évidents :

- Aucune route n'est branchée : `web/web.go` sur `main` ne connaît pas ARCEP, et
  exposer `/api/arcep/qos` sans le reste du module laisserait une API à moitié
  vivante. Le branchement sera fait avec le retour du module.
- Les seuils ne sont pas recopiés en Go : le fichier ne décrit que leur forme,
  la valeur reste dans `config/seuils.json` (règle posée par `seuil.md`).
- La validation refuse un seuil à zéro : il validerait n'importe quelle mesure
  et produirait un score faussement bon.

## Critère d'acceptation

Pour chaque niveau de performance, le score obtenu correspond aux seuils
validés — la partie vérifiable ici est que l'API lit **exactement** le
référentiel validé (30 / 95 / 20 / 95 ; 5 / 95) et refuse un référentiel
incomplet.

**Non atteint à ce stade** : aucun endpoint ne publie ces seuils, donc
l'application ne peut pas encore les consommer.

## Validation

- **Automatisée** : `go build ./...`, `go vet ./...`, `go test ./...` exécutés
  sur la branche.
- **Support imposé par le classeur** : local (jeux de valeurs autour des
  seuils) — **non fait** pour la partie score, qui vit dans `mobilefront`.
- **Preuve** : sortie de `go test ./results -run Arcep` à joindre à la PR.

## Limites connues

- Aucun endpoint exposé ; le module ARCEP (`results/arcep.go`,
  `config/seuils.json`, routes `/api/arcep/*`) n'est pas sur `main` et sa
  réintroduction est une décision d'équipe, pas un effet de bord de cet item.
- Le registre de ce dépôt contient encore deux lignes d'exemple
  (`_(exemple — à remplacer)_`, 2026-10-14) qui valident `IMP-11` et `ISS-12`
  sans contribution réelle : le garde-fou les accepte. À nettoyer, comme cela a
  été fait sur `mobilefront` (PR #39).
- Le tableau « Suivi des 27 items » en fin de registre ne compte pas pour le
  garde-fou : seule la ligne datée ajoutée ici fait foi.

---

## Liste de contrôle avant d'ouvrir la PR

- [x] La fiche est dans `docs/fiches/` avec le nom `IMP-06-seuils-arcep-scoring.md`.
- [x] Les champs du tableau viennent de `docs/items-v14.json`.
- [x] Une ligne **datée** a été ajoutée au tableau d'historique de
      `docs/registre-revisions.md` (sans elle, `guard` est rouge).
- [ ] Le `CHANGELOG.md` du dépôt a une entrée sous `## [Non publiée]` —
      **le fichier n'existe pas dans ce dépôt** (§3 du CONTRIBUTING non
      appliqué ici) ; à créer avec le socle, hors périmètre de cet item.
- [ ] Le corps de la PR porte `Closes yele-bf/mobilefront#12` et la même liste
      de validation.
