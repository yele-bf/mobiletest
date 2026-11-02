#!/usr/bin/env python3
"""yele_secrets_guard — refuse un secret en clair dans les lignes ajoutées d'une PR.

Usage :
    python3 scripts/ci/yele_secrets_guard.py <base_sha> <head_sha>

Pourquoi ce contrôle existe : le 30.10.2026, `settings.toml` a reçu une chaîne
de connexion MongoDB complète (utilisateur + mot de passe) dans le commit
`0fc094a`. Le dépôt est public : le secret a été lisible par n'importe qui
pendant des heures. Aucun contrôle automatique ne l'a vu ; ce script comble ce
trou.

Portée volontairement limitée aux lignes **ajoutées** par la PR : l'historique
déjà en place n'est pas jugé (une réécriture d'historique sur un dépôt public
invaliderait tous les hashes cités dans le registre des révisions).

Ce contrôle ne remplace pas une rotation : une valeur retirée du fichier reste
dans l'historique Git. Voir docs/CONTRIBUTING-equipe.md §7.

Code de sortie : 0 si aucun secret, 1 dès qu'un secret est détecté.
"""
import re
import subprocess
import sys

# Motifs de secrets : (libellé, expression, extraction du secret à examiner).
MOTIFS = [
    (
        "chaîne de connexion MongoDB",
        re.compile(r"mongodb(?:\+srv)?://([^\s\"'<>]+):([^\s\"'<>@]*)@[^\s\"'<>]+"),
        1,
    ),
    ("clé AWS", re.compile(r"\b(AKIA[0-9A-Z]{16})\b"), 1),
    ("clé Google", re.compile(r"\b(AIza[0-9A-Za-z_\-]{35})\b"), 1),
    (
        "jeton GitHub",
        re.compile(r"\b(gh[pousr]_[A-Za-z0-9]{20,}|github_pat_[A-Za-z0-9_]{20,})\b"),
        1,
    ),
    ("clé privée", re.compile(r"(-----BEGIN [A-Z ]*PRIVATE KEY-----)"), 1),
]

# Valeurs manifestement fictives : elles doivent rester autorisées, sinon le
# garde-fou punirait la documentation et .env.example, et serait désactivé.
EXEMPLES = {
    "user",
    "username",
    "utilisateur",
    "password",
    "pass",
    "motdepasse",
    "mot-de-passe",
    "mdp",
    "changeme",
    "xxx",
    "...",
    "secret",
}


def est_un_exemple(valeur):
    """Vrai si la valeur ne peut pas être un secret réel."""
    v = valeur.strip().lower()
    if not v or v in EXEMPLES:
        return True
    if set(v) <= {".", "*", "x", "-"}:
        return True
    if v.startswith("<") or v.startswith("$") or v.startswith("{") or v.endswith(">"):
        return True
    return False


def lignes_ajoutees(base, head):
    """Rend (fichier, numéro, contenu) pour chaque ligne ajoutée du diff."""
    diff = subprocess.run(
        ["git", "diff", "--unified=0", "--no-color", "--no-ext-diff", base, head],
        capture_output=True,
        text=True,
        encoding="utf-8",
        errors="replace",
    )
    if diff.returncode != 0:
        sys.stderr.write(diff.stderr or "git diff a échoué\n")
        return None
    fichier = "?"
    numero = 0
    resultat = []
    for ligne in diff.stdout.splitlines():
        if ligne.startswith("+++ b/"):
            fichier = ligne[6:]
        elif ligne.startswith("@@ "):
            m = re.search(r"\+(\d+)", ligne)
            numero = int(m.group(1)) if m else 0
        elif ligne.startswith("+") and not ligne.startswith("+++"):
            resultat.append((fichier, numero, ligne[1:]))
            numero += 1
    return resultat


def main():
    if len(sys.argv) != 3:
        sys.stderr.write("usage : yele_secrets_guard.py <base_sha> <head_sha>\n")
        return 2
    base, head = sys.argv[1], sys.argv[2]

    lignes = lignes_ajoutees(base, head)
    if lignes is None:
        return 2

    trouvailles = []
    for fichier, numero, ligne in lignes:
        for libelle, motif, groupe in MOTIFS:
            m = motif.search(ligne)
            if not m:
                continue
            if libelle == "chaîne de connexion MongoDB":
                # groupe 1 = utilisateur, groupe 2 = mot de passe
                if est_un_exemple(m.group(2)):
                    continue
            elif est_un_exemple(m.group(groupe)):
                continue
            trouvailles.append((fichier, numero, libelle))

    print(
        f"yele_secrets_guard · {len(lignes)} ligne(s) ajoutée(s) analysée(s) "
        f"sur {base[:8]}..{head[:8]}"
    )
    if not trouvailles:
        print("OK - aucun secret en clair dans les lignes ajoutées.")
        return 0

    print(f"\n{len(trouvailles)} secret(s) en clair :\n")
    for fichier, numero, libelle in trouvailles:
        print(f"  x {fichier}:{numero} · {libelle}")
    print(
        "\nUn dépôt public ne contient aucun secret : la valeur se transmet par\n"
        "variable d'environnement (voir render.yaml, `sync: false`). Retirer la\n"
        "valeur du fichier ne suffit pas si elle est déjà dans l'historique :\n"
        "il faut aussi faire tourner le secret. Voir docs/CONTRIBUTING-equipe.md §7."
    )
    return 1


if __name__ == "__main__":
    sys.exit(main())
