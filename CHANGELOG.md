# Changelog

Format : [Keep a Changelog 1.1.0](https://keepachangelog.com/fr/1.1.0/), une
version par vague (CONTRIBUTING §3).

## [Non publiée]

### Ajouté

- IMP-07 : perte de paquets (`packetLoss`, %) lue dans `extra`, stockée (`packet_loss`) et renvoyée par `/api/dashboard/map`. Absente ou hors de 0–100 % : non mesurée (`null`), jamais confondue avec 0 %.
- IMP-03 : comptes utilisateurs. `POST /api/auth/register`, `/api/auth/login`, `/api/auth/logout` ; `GET`/`PUT`/`DELETE /api/me` ; `GET /api/me/results` (mesures du seul compte). Mots de passe PBKDF2-SHA256 (600 000 itérations, sel par compte) ; sessions par jeton aléatoire dont seule l'empreinte SHA-256 est stockée, expirées après 90 jours (index TTL) ; 20 tentatives par IP et par 10 minutes. Un test envoyé avec `Authorization: Bearer` est rattaché au compte (`user_id`, jamais renvoyé par l'API) ; supprimer un compte rend ses mesures anonymes.
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

