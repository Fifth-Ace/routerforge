# Contributing to RouterForge

Contributions, bug reports and improvement ideas are welcome through [GitHub Issues](https://github.com/Fifth-Ace/routerforge/issues) and pull requests.

## Branches

- `main` — Stable source.
- `dev` — active development.
- Changes ordinarily target `dev`.

Releases are built from verified commits; Stable must use an already validated exact commit. Do not publish a release as part of an unrelated code or documentation change.

## Build and test

```sh
gofmt -w .
go test ./...
go vet ./...
sh scripts/build-frontend.sh
sh scripts/build-dns-frontend.sh
sh scripts/build-admin-frontend.sh
sh scripts/build-monitoring-frontend.sh
python3 scripts/verify-docs-current.py
```

Backend: Go 1.21+. Frontend build: Node.js 22.x.

## Layout

- `components/` — Core and shared product components.
- `modules/` — optional modules.
- `internal/` — shared internal packages.
- `marketplace/` — application registry, schemas and metadata.
- `release/` — channel manifests.
- `scripts/` — build and release tools.
- `docs/` — public documentation.

## Safety and documentation

Preserve explicit authorization and confirmation for sensitive operations. Avoid automatic changes to system firewall, unbounded shell execution, blind LAN scanning and high-frequency writes on router storage.

Document user-visible changes in public documentation. Update release notes only for the corresponding release. Retain all required license and attribution notices.
