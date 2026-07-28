# Patches

Ce fichier trace, pour chaque tag `vX.Y.Z-pcloud.N` de ce fork, la version
upstream Kopia de base et les patches appliques par-dessus, sur la branche
`pcloud-integration`. Un rebase sur une nouvelle version upstream incremente
`X.Y.Z` et repart de `N=1`. Voir [`README-pcloud.md`](README-pcloud.md) pour
le detail du wiring et les regles de patch.

## v0.23.1-pcloud.1

- Base upstream : [`v0.23.1`](https://github.com/kopia/kopia/releases/tag/v0.23.1)
  ([`72ec08fd`](https://github.com/kopia/kopia/commit/72ec08fd8edb86c67ed27099bf1b955e1f308ffa),
  2026-06-15)
- Patches appliques (1 commit) :
  - [`ad6e0ba`](https://github.com/kopia-backends/kopia/commit/ad6e0ba6) —
    feat(pcloud): register native pCloud storage provider
- Fichiers touches : `README-pcloud.md`, `cli/app.go`,
  `cli/command_pcloud.go`, `cli/command_pcloud_storage.go`,
  `cli/storage_pcloud.go`, `go.mod`
