# RouterForge release process

## Channels
- Dev: every green `dev` push → rolling ARM64 `routerforge-dev`.
- Beta: explicit FULL RELEASE on exact dev SHA with `publish_beta=true`.
- Stable: exact validated SHA fast-forwarded to `main`; main job consumes that SHA's `routerforge-stable-promotion` artifact.

## Versioning

Компоненты версионируются независимо. Правила выбора PATCH/MINOR/MAJOR описаны в
[VERSIONING.md](VERSIONING.md).

Pre-release ordering использует `~` в OPKG version, например:

```text
0.9.0~dev.rN.sha < 0.9.0~beta.N < 0.9.0
```

Stable и Beta manifests могут содержать разные версии компонентов. Release-level version
описывает RouterForge train, но не заставляет неизменённые package versions косметически
двигаться вместе с ним.

`0.11.0-beta.1` включает восемь пакетов. Меняются только Core
(`0.10.1~beta.1`) и новый Antiscan Manager (`0.11.0~beta.1`); остальные шесть
компонентов сохраняют ранее опубликованные `0.10.0~beta.5`. Stable 0.11.0
аналогично сохраняет unchanged package versions `0.10.0`.

## Prep gate

Exact dev/main SHAs, clean tree/staging, docs+CHANGELOG+channel manifest updated,
`git diff --check`, ordinary push CI green, hardware evidence for claimed runtime changes.

First RED → stop, diagnose, no blind rerun.

## FULL RELEASE

Beta publication:

```sh
gh workflow run ci.yml --ref dev -f full_release=true -f publish_beta=true
```

Stable candidate without Beta mutation:

```sh
gh workflow run ci.yml --ref dev -f full_release=true -f publish_beta=false
```

## Stable promotion

1. Prepare Stable docs/manifest on dev.
2. Push and pass exact ordinary CI.
3. FULL RELEASE `publish_beta=false` on exact SHA.
4. Verify exact `routerforge-stable-promotion` artifact.
5. Fast-forward **the same SHA** to main.
6. Main `Publish validated stable promotion` downloads/verifies that exact artifact.
7. Publish coherent rolling `routerforge-stable`.
8. Create immutable `routerforge-v<stable-version>` without clobber.

Stable 0.11.0 topology = 8 packages × 3 targets = 24 current IPKs. Only two package IDs change relative to Stable 0.10.0:
Core 0.10.1 and new Antiscan Manager 0.11.0. DNS/Admin/Monitoring/Network Tools/NFQWS Manager/Profiling remain 0.10.0.

Post-verify: exact run SHA, rolling/immutable assets, 3 indexes, SHA256SUMS, bootstraps,
exact immutable tag target, main/dev exact SHA, Beta unchanged when `publish_beta=false`.
