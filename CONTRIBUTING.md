# Contributing to RouterForge

RouterForge targets Keenetic / Netcraze ARM64 routers running Entware.

## Branches

- `main` — production source for the RouterForge Stable channel.
- `dev` — active development and RouterForge Beta channel.
- feature/fix branches should normally target `dev`.

A change reaches `main` only after it has passed CI and, when runtime behavior changes, has been validated on a test router.

## Local checks

Backend:

```sh
gofmt -w .
go test ./...
go vet ./...
```

Frontend:

```sh
cd frontend
npm install --no-audit --no-fund
npm run check
npm run build
```

Release tooling:

```sh
python3 -m py_compile \
  scripts/build_routerforge_channel.py \
  scripts/merge_release_index.py \
  scripts/render_release_notes.py \
  scripts/render_bootstrap.py
```

Shell:

```sh
sh -n scripts/install-repo.sh
sh -n scripts/remove-repo.sh
sh -n scripts/build-opkg.sh
sh -n scripts/build-admin-opkg.sh
sh -n scripts/build-module-opkg.sh
```

CI performs a broader package/container/runtime verification.

## Component versions

Official versions are declared independently:

```text
packaging/channels/beta.json
packaging/channels/stable.json
```

To release one component, bump only that component.

Do **not** bump Core just because an optional module changed.

CI preserves the previous release asset and SHA256 for any component whose version is unchanged.

## Release channels

- push to `dev` → `routerforge-beta` Pre-release;
- push to `main` → `routerforge-stable` production/Latest release.

Channel release-index is authoritative for exact version, asset, URL and SHA256.

## Marketplace manifests

Do not turn Registry metadata into arbitrary shell execution.

Executable lifecycle must use supported typed methods and explicit package/path constraints.

Changes to approved manifests must change their manifest SHA and therefore require a new trust decision.

## Keenetic parsers

Keenetic/NDMS command output varies between firmware versions and hardware families.

Parsers should:

- tolerate missing fields;
- avoid hardcoding transient IDs when discovery is possible;
- fail visibly rather than silently inventing data;
- keep diagnostics safe for production routers.

## Runtime safety

Do not add:

- background benchmark workloads to monitoring;
- frequent persistent writes for high-rate telemetry;
- new LAN listeners for helper modules without a strong reason;
- broad `opkg upgrade` behavior;
- unauthenticated mutation endpoints.

## Bug reports

Include sanitized:

- router model;
- KeeneticOS/NDMS version;
- ARM64 confirmation;
- Stable/Beta channel;
- `routerforge-core` version;
- affected module versions;
- relevant RouterForge logs.

Never publish credentials, cookies, private keys, tunnel secrets, private infrastructure names, public IPs or MAC addresses.
