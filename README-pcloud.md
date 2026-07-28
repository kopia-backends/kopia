# Kopia fork wiring for pCloud

Ce dossier contient un fork local de Kopia v0.23.1 avec le provider pCloud
natif branche sur la CLI.

Regles :

- ne pas copier le SDK pCloud dans le fork ;
- ne pas ajouter de logique metier pCloud ici ;
- limiter les changements a l'enregistrement du provider ;
- conserver les `replace` vers `../go-pcloud` et `../kopia-pcloud` pendant le developpement local.

Fichiers modifies pour le provider pCloud :

- `go.mod` ajoute `github.com/kopia-backends/kopia-pcloud` et garde les `replace` locaux ;
- `cli/storage_pcloud.go` ajoute la commande CLI `repository create pcloud`.
- `cli/command_pcloud.go` ajoute `pcloud authorize` pour generer ou echanger un
  token OAuth pCloud sans script externe.
- `cli/command_pcloud_storage.go` ajoute `pcloud storage` pour afficher ou
  surveiller le quota et detecter les destinations partagees.

Le provider est enregistre sous le type Kopia `pcloud` via `blob.AddSupportedStorage`.
La logique pCloud reste dans `../kopia-pcloud` et `../go-pcloud`.

Correspondance entre chaque tag `vX.Y.Z-pcloud.N` et la version upstream Kopia
de base : voir [`PATCHES.md`](PATCHES.md).

Verification rapide depuis `../kopia-pcloud-infra` :

```sh
scripts/kopia.sh repository create pcloud --help
```

Exemple minimal avec les variables de `.env.local` chargees par le helper :

```sh
scripts/kopia.sh repository create pcloud \
  --root-path "$PCLOUD_TEST_ROOT/manual-repo" \
  --password "change-me" \
  --create-only
```

OAuth integre :

```sh
kopia-pcloud pcloud authorize --interactive
kopia-pcloud pcloud authorize --callback-url "URL_COMPLETE_RETOUR_PCLOUD"
kopia-pcloud pcloud authorize --code "CODE_PCLOUD" --oauth-hostname eapi.pcloud.com
```

Coller l'URL complete de retour pCloud est preferable au code seul, car elle
contient aussi le bon endpoint `api.pcloud.com` ou `eapi.pcloud.com`.
Si pCloud affiche une page avec `Your access code is` et `Hostname`, coller le
code puis le hostname dans le mode interactif, ou passer `--oauth-hostname`.

Pour les assistants de configuration, `--env-file PATH` ecrit le resultat OAuth
dans un fichier sourceable par `sh` au lieu d'afficher le token dans le
terminal.

Surveillance du stockage :

```sh
kopia-pcloud pcloud storage --root-path /kopia-pcloud-prod/mirrors/example-mirror
kopia-pcloud pcloud storage --root-path /kopia-pcloud-prod/mirrors/example-mirror --watch
```

Les commandes `repository create pcloud` et `repository sync-to pcloud`
acceptent aussi `--client-id`, `--client-secret`, `--oauth-code`,
`--callback-url`, `--oauth-hostname` et `--root-path`, avec les variables d'environnement
`PCLOUD_CLIENT_ID`, `PCLOUD_CLIENT_SECRET`, `PCLOUD_OAUTH_CODE`,
`PCLOUD_OAUTH_CALLBACK_URL`, `PCLOUD_OAUTH_HOSTNAME`, `PCLOUD_ACCESS_TOKEN` et
`PCLOUD_ROOT_PATH`.
