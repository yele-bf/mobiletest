# Changelog

Format : [Keep a Changelog 1.1.0](https://keepachangelog.com/fr/1.1.0/), une
version par vague (CONTRIBUTING §3).

## [Non publiée]

### Ajouté

- IMP-09 : `/api/dashboard/barometer` accepte `operator` et renvoie, en plus des centiles nationaux, `nationalAvgDownload` et la comparaison parmi les clients de cet opérateur (`operator`, `operatorSampleSize`, `operatorDownloadPercentile`, `operatorUploadPercentile`, `operatorPingPercentile`, `operatorAvgDownload`, à partir de 5 mesures). Libellés unifiés (`operatorFamily` : « Orange BF », « Orange Burkina Faso », « 61302 » → « Orange »). Calcul isolé dans `barometerFrom` et testé.
- IMP-06 : référentiel ARCEP version 3 (`config/seuils.json`, zones, profils
  D1–D4, seuils QoS navigation et streaming) rétabli sur `main` depuis la
  branche `fix/ARCEP-routes` (commit `cd1556b`), avec son module de
  comparaison (`results/arcep.go`) et l'agrégation par ville
  (`database/mongodb/arcep.go`).
- IMP-06 : endpoints `GET /api/arcep/comparison`, `GET /api/arcep/measure` et
  `POST /api/arcep/measure/batch`, branchés en chi comme les routes dashboard.
- IMP-06 : validation du bloc « qos » au démarrage (`results/arcep_qos.go`) et
  tests du référentiel livré (`results/arcep_referentiel_test.go`) — un seuil
  nul, un profil inconnu ou un bloc `qos` absent font échouer le chargement.

### Corrigé

- Garde-fou : le contrôle « le Sous-ID a une ligne au registre » ne porte plus que
  sur le **sujet** du commit (une PR de documentation qui nomme un Sous-ID dans
  le corps n'est plus refusée à tort), et les deux lignes d'exemple datées du
  registre sont retirées — elles satisfaisaient ce contrôle pour `IMP-11` et
  `ISS-12`. Règle écrite au CONTRIBUTING §5.

