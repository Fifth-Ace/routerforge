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

Stable manifest может содержать разные версии компонентов. Beta release train,
напротив, намеренно публикуется единым coherent prerelease train.

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

Stable 0.9.0 topology = 6 packages × 3 targets = 18 IPKs, with independent package versions:
Core 0.9.0, DNS 0.8.1, Admin 0.8.1, Monitoring 0.8.0, Network Tools 0.9.0, Profiling 0.7.1.

Post-verify: exact run SHA, rolling/immutable assets, 3 indexes, SHA256SUMS, bootstraps,
exact immutable tag target, main/dev exact SHA, Beta unchanged when `publish_beta=false`.
