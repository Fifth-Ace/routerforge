# Repository layout

RouterForge is organized by **component ownership**. Product code should not be dropped into the repository root.

```text
routerforge/
├── components/
│   ├── core/
│   │   ├── *.go / *_test.go
│   │   ├── embedded/
│   │   ├── frontend/
│   │   └── packaging/
│   └── control/
│       ├── *.go / *_test.go
│       └── packaging/
├── modules/
│   ├── dns/
│   │   ├── runtime/
│   │   ├── frontend/
│   │   └── packaging/
│   ├── monitoring-runtime/
│   │   ├── *.go / *_test.go
│   │   └── packaging/
│   ├── system/packaging/
│   ├── thermal/packaging/
│   ├── storage/packaging/
│   ├── network/packaging/
│   └── profiling/packaging/
├── release/channels/
├── marketplace/
├── scripts/
├── docs/
├── archive/
├── .github/
└── project metadata / documentation
```

## Components

### `components/core/`

Owns RouterForge Core: web shell, authentication, Marketplace lifecycle, Registry/release handling, Module ABI proxy, Core tests, embedded assets, frontend and OPKG lifecycle files.

Core remains the only RouterForge process that listens on TCP port `2233`.

### `components/control/`

Owns RouterForge Control (`routerforge-admin`), its tests and package lifecycle files.

## Modules

### `modules/dns/`

Owns the independent DNS Module ABI v1 runtime, DNS tests, DNS UI and package lifecycle files.

The DNS UI has its own small Vite/Svelte build workspace under `modules/dns/frontend/`. It intentionally imports the shared RouterForge shell CSS from the Core frontend so both UIs consume the same visual tokens without copying them.

### `modules/monitoring-runtime/`

System, Thermal, Storage and Network currently share one read-only Unix-socket runtime selected by `-module`. The common Go server/collector implementation therefore lives here once instead of being copied four times.

Package-specific init scripts still live with their public modules:

- `modules/system/packaging/`
- `modules/thermal/packaging/`
- `modules/storage/packaging/`
- `modules/network/packaging/`

If those modules become independent runtimes later, their Go implementation can move into the corresponding module directory without changing package names.

### `modules/profiling/`

Owns the optional profiling package lifecycle/configuration. The actual pprof hook remains inside Core because profiling instruments the Core process itself.

## Marketplace compatibility exception

`marketplace/` intentionally remains at the repository root. Older RouterForge Core versions fetch this public URL directly:

```text
.../<branch>/marketplace/registry/index.json
```

Moving that public path would break compatibility. Core therefore embeds an exact mirror at:

```text
components/core/embedded/marketplace-index.json
```

`marketplace/build_registry.py` writes/checks both copies and CI rejects drift.

## Release metadata

Source channel manifests live in:

```text
release/channels/beta.json
release/channels/stable.json
```

Architecture selection is independent from component layout and remains centralized in `scripts/target-env.sh`.

## Root-directory policy

The root is deliberately boring. Do not add product `.go` files, module UI source or package lifecycle scripts directly to it. New functionality belongs to its owning component/module; genuinely shared infrastructure belongs in an explicitly named shared/runtime directory.

The CI layout guard verifies the key boundaries so the repository cannot silently grow back into a flat source dump.
