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
│   ├── admin/frontend/
│   ├── dns/
│   │   ├── runtime/
│   │   ├── frontend/
│   │   └── packaging/
│   ├── monitoring-runtime/
│   │   ├── *.go / *_test.go
│   │   └── packaging/
│   ├── monitoring/frontend/
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

Owns RouterForge Core: web shell, authentication, Центр приложений lifecycle, Registry/release handling, Module ABI proxy, Core tests, embedded assets, frontend and OPKG lifecycle files.

Core remains the only RouterForge process that listens on TCP port `2233`.

### `components/control/`

Owns RouterForge Control (`routerforge-admin`) backend, tests and package lifecycle files. Its standalone Svelte/Vite UI lives in `modules/admin/frontend/` and is bundled into the Admin IPK.

## Modules

### `modules/dns/`

Owns the independent DNS Module ABI v1 runtime, DNS tests, DNS UI and package lifecycle files.

The DNS UI has its own small Vite/Svelte build workspace under `modules/dns/frontend/`. It intentionally imports the shared RouterForge shell CSS from the Core frontend so both UIs consume the same visual tokens without copying them.

### `modules/monitoring-runtime/`

System, Thermal, Storage and Network collectors share one read-only implementation. The current Dev/Beta product packages that implementation as consolidated `routerforge-monitoring`, while also exposing compatible legacy sockets/API routes.

The standalone Monitoring UI lives under `modules/monitoring/frontend/`.

Legacy split package init/lifecycle files remain under:

- `modules/system/packaging/`
- `modules/thermal/packaging/`
- `modules/storage/packaging/`
- `modules/network/packaging/`

Those directories are compatibility/migration ownership, not the current Beta package topology.

### `modules/profiling/`

Owns the optional profiling package lifecycle/configuration. The actual pprof hook remains inside Core because profiling instruments the Core process itself.

## Legacy `marketplace/` compatibility exception

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
release/channels/dev.json
release/channels/beta.json
release/channels/stable.json
```

Architecture selection is independent from component layout and remains centralized in `scripts/target-env.sh`.

## Root-directory policy

The root is deliberately boring. Do not add product `.go` files, module UI source or package lifecycle scripts directly to it. New functionality belongs to its owning component/module; genuinely shared infrastructure belongs in an explicitly named shared/runtime directory.

The CI layout guard verifies the key boundaries so the repository cannot silently grow back into a flat source dump.
