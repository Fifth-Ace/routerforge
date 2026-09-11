# Contributing to RouterForge

ARM64 is the production hardware-validated target; MIPS/MIPSel remain experimental. See [docs/ARCHITECTURES.md](docs/ARCHITECTURES.md).

## Branches
- `main` — production Stable source.
- `dev` — active development and rolling ARM64 Dev.
- feature/fix work normally targets `dev`.

Beta is explicit FULL RELEASE, not every dev push. Stable main must be fast-forwarded to the **same exact SHA** that already has a successful Dev FULL RELEASE stable-promotion artifact.

## Checks

```sh
gofmt -w .
go test ./...
go vet ./...
sh scripts/build-frontend.sh
sh scripts/build-dns-frontend.sh
sh scripts/build-admin-frontend.sh
sh scripts/build-monitoring-frontend.sh
python3 -m py_compile scripts/build_routerforge_channel.py scripts/merge_release_index.py scripts/render_release_notes.py scripts/render_bootstrap.py scripts/render_universal_bootstrap.py scripts/verify-docs-current.py
```

## Ownership
See [docs/REPOSITORY_LAYOUT.md](docs/REPOSITORY_LAYOUT.md). Product code belongs to its component/module, not repository root.

## Releases
Channel manifests live under `release/channels/`.
Release-index is authoritative exact version/asset/URL/SHA256.
Beta release metadata is fail-closed against inconsistent train versions.

Full flow: [docs/RELEASE_PROCESS.md](docs/RELEASE_PROCESS.md).

## Runtime safety
Do not add blind LAN scans, arbitrary shell execution from browser input, unauthenticated mutations, broad automatic `opkg upgrade`, unnecessary LAN listeners or high-frequency persistent telemetry writes.

## Documentation
User-visible behavior must update relevant docs. Release work updates CHANGELOG and channel release notes. CI checks local Markdown links and current-version invariants.
