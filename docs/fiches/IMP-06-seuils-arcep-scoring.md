# IMP-06 — Seuils ARCEP lisibles par l'API (volet référentiel et endpoints)

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

Or, sur `main`, le module ARCEP n'était plus présent : ni `config/seuils.json`,
ni `results/arcep.go`, ni les routes `/api/arcep/*`. Le référentiel validé
(version 3) ne vivait plus que sur la branche `fix/ARCEP-routes` du 20/10,
restée sans PR et dont le dernier commit ne compilait pas (`web/web.go`
remplacé par une version gin alors que `main` utilise chi). Résultat : aucun
moyen, depuis `main`, de lire les seuils ARCEP qui ont servi à valider l'item.

## Correction

Le module est rétabli sur `main` à partir du contenu de `fix/ARCEP-routes`
(commit `cd1556b`, auteur `TESTEUR04`), sans rien réécrire du référentiel :

- `config/seuils.json` : référentiel **version 3**, repris à l'identique. Sa
  provenance est dans le fichier lui-même (`source`, `periode`,
  `method_note`) — aucune valeur n'a été inventée ni complétée.
- `results/arcep.go` : module de comparaison (zones par GPS ou par ville,
  profils D1–D4, verdicts QoS), repris à l'identique.
- `database/mongodb/arcep.go` : agrégation `FetchArcepCityStats` (moyennes par
  ville), reprise à l'identique.
- `results/arcep_qos.go` : types du bloc « qos », décodeur et validation
  (conservés de la première version de cette PR). `arcepConfig.Qos` réutilise
  désormais ces types : une seule représentation du bloc QoS dans le code.
- `web/web.go` : les trois routes `/api/arcep/comparison`,
  `/api/arcep/measure` et `POST /api/arcep/measure/batch` sont branchées en
  syntaxe **chi**, à côté des routes dashboard.
- `main.go` : `results.LoadArcepConfig("config/seuils.json")` au démarrage.
  Fichier absent ou invalide ⇒ les routes répondent une erreur explicite ; le
  backend ne sert jamais de seuils inventés.

Choix non évidents :

- Le `web/web.go` de `cd1556b` n'a **pas** été repris : il troquait chi contre
  gin, supprimait `ListenAndServe` et emportait `go-chi/cors` et
  `pires/go-proxyproto` avec lui. Seules les trois lignes de routes étaient
  réutilisables.
- `results/arcep_qos.go` valide désormais le référentiel au chargement
  (`loadArcepFile`), pas seulement à la demande : un `page_load_s` à zéro
  validerait n'importe quelle mesure et produirait un score faussement bon.
- Les commentaires qui renvoyaient à `seuil.md` (absent du dépôt) et à un
  montage Docker (la solution n'utilise pas Docker) pointent maintenant vers
  le référentiel et vers cette fiche.

## Critère d'acceptation

Pour chaque niveau de performance, le score obtenu correspond aux seuils
validés. Vérifiable ici : l'API charge **exactement** le référentiel version 3
(30 s / 95 % / 20 s / 95 % en navigation, 5 s / 95 % en streaming, profils
D1–D4 et 10 zones) et publie ces valeurs sur `/api/arcep/*`.

## Validation

- **Automatisée** : `go build ./...`, `go vet ./...`, `go test ./...` verts sur
  la branche ; 7 tests `results`, dont `TestLoadArcepFileReferentiel` qui
  charge le fichier livré et contrôle version, profils, rayon, seuils QoS et
  alias de zone, et deux tests de refus (profil inconnu, seuil QoS nul).
- **Support imposé par le classeur** : local (jeux de valeurs autour des
  seuils) — fait le 2026-10-26 : binaire compilé lancé sur
  `config/seuils.json`, journal `arcep: 10 zones chargées depuis
  config/seuils.json (config v3)`, les trois routes répondent (garde-fou
  `database_type=mongodb` renvoyé tel quel hors MongoDB).
- **Preuve** : sortie de `go test ./results -run Arcep -v` (7/7 PASS) et
  journal de démarrage cité ci-dessus.

## Limites connues

- Les réponses JSON des handlers n'ont pas pu être relues de bout en bout :
  elles joignent les moyennes MongoDB, et aucune instance de test n'est
  disponible ici. Le chargement du référentiel, les routes et les tests, eux,
  sont vérifiés.
- Le registre de ce dépôt contient encore deux lignes d'exemple
  (`_(exemple — à remplacer)_`, 2026-10-14) qui valident `IMP-11` et `ISS-12`
  sans contribution réelle : le garde-fou les accepte. À nettoyer avec le
  durcissement du garde-fou (travail d'équipe, pas de cet item).
- Le tableau « Suivi des 27 items » en fin de registre ne compte pas pour le
  garde-fou : seule la ligne datée ajoutée ici fait foi.

---

## Liste de contrôle avant d'ouvrir la PR

- [x] La fiche est dans `docs/fiches/` avec le nom `IMP-06-seuils-arcep-scoring.md`.
- [x] Les champs du tableau viennent de `docs/items-v14.json`.
- [x] Une ligne **datée** a été ajoutée au tableau d'historique de
      `docs/registre-revisions.md` (sans elle, `guard` est rouge).
- [x] Le `CHANGELOG.md` du dépôt a une entrée sous `## [Non publiée]`
      (fichier créé avec cet item, §3 du CONTRIBUTING).
- [x] Le corps de la PR porte la référence à l'issue et la même liste de
      validation.
