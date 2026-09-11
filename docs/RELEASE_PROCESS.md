# RouterForge release process

## Channels
- Dev: every green `dev` push → rolling ARM64 `routerforge-dev`.
- Beta: explicit FULL RELEASE on exact dev SHA with `publish_beta=true`.
- Stable: exact validated SHA fast-forwarded to `main`; main job consumes that SHA's `routerforge-stable-promotion` artifact.

## Version ordering

```text
0.7.1~dev.rN.sha < 0.7.1~beta.N < 0.7.1
```

## Prep gate
Exact dev/main SHAs, clean tree/staging, docs+CHANGELOG+channel manifest updated, `git diff --check`, ordinary push CI green, hardware evidence for claimed runtime changes.

First RED → stop, diagnose, no blind rerun.

## Beta train guard
Beta builder rejects inconsistent `release_version`, component `version`, or non-empty `min_core_version`.

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

Stable 0.7.1 = 5 packages × 3 targets = 15 IPKs.

Post-verify: exact run SHA, rolling/immutable assets, 3 indexes, SHA256SUMS, bootstraps, exact immutable tag target, main/dev exact SHA, Beta unchanged when `publish_beta=false`.
