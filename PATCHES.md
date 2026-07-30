# Patches

Ce fichier trace, pour chaque tag `vX.Y.Z-pcloud.N` de ce fork, la version
upstream Kopia de base et les patches appliques par-dessus, sur la branche
`pcloud-integration`. Un rebase sur une nouvelle version upstream incremente
`X.Y.Z` et repart de `N=1`. Voir [`README-pcloud.md`](README-pcloud.md) pour
le detail du wiring et les regles de patch.

## v0.23.1-pcloud.2

- Base upstream : inchangee, [`v0.23.1`](https://github.com/kopia/kopia/releases/tag/v0.23.1)
  ([`72ec08fd`](https://github.com/kopia/kopia/commit/72ec08fd8edb86c67ed27099bf1b955e1f308ffa),
  2026-06-15)
- Patches appliques depuis `v0.23.1-pcloud.1` (4 commits) :
  - [`b96b01d6`](https://github.com/kopia-backends/kopia/commit/b96b01d6d6f3132613cf52f9eea232b62c998dec) —
    ci: add build workflow (cross-repo checkout of go-pcloud/kopia-pcloud)
  - [`a988e4c8`](https://github.com/kopia-backends/kopia/commit/a988e4c828009fef8fc267fe15329acaa4e45e1f) —
    docs: add PATCHES.md mapping pcloud tags to upstream Kopia
  - [`b376a2fe`](https://github.com/kopia-backends/kopia/commit/b376a2fe24a252d11f6b7ac13c1ad220de610c02) —
    fix: add AGENTS.md and .agents/ to .gitignore
  - [`41eb8937`](https://github.com/kopia-backends/kopia/commit/41eb8937473f58bf2caba8955d03e42b649f58f3) —
    feat(pcloud): implement mkdir and preflight commands for pCloud integration
- Fichiers touches : `.github/workflows/build.yml`, `.gitignore`, `PATCHES.md`,
  `README-pcloud.md`, `cli/command_pcloud.go`, `cli/command_pcloud_mkdir.go`,
  `cli/command_pcloud_mkdir_test.go`, `cli/command_pcloud_preflight.go`,
  `cli/command_pcloud_preflight_test.go`

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
