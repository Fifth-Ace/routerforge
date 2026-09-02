# DNS Monitor frontend architecture

## Decision

The production frontend is a **Svelte 5 / SvelteKit / Vite** application built with
`@sveltejs/adapter-static`, following the same proven deployment model used by
AWG Manager:

- the router does **not** run Node.js;
- CI/build machines compile the frontend into `frontend/build`;
- production Go builds use the `embed_frontend` build tag;
- the compiled static bundle is embedded into the `dns-monitor` binary;
- unknown non-API routes fall back to SvelteKit `index.html`;
- SvelteKit performs client-side route navigation without document reloads.

## Runtime model

```text
Go backend :2233
├── REST API
│   ├── /api/snapshot
│   ├── /api/history
│   ├── /api/clients
│   ├── /api/client
│   ├── /api/interfaces
│   ├── /api/system
│   └── /api/catalog
├── SSE
│   └── /api/events
└── Embedded SvelteKit build
    ├── shared +layout.svelte
    ├── /
    ├── /servers
    ├── /routing
    ├── /monitoring
    ├── /tools
    ├── /catalog
    └── /settings
```

The shared layout owns the long-lived snapshot EventSource connection. Top-level
route changes keep the application shell and stores alive; only the route
component changes.

`/api/events` pushes snapshot updates at the configured UI refresh interval.
If EventSource is unavailable or temporarily disconnected, the frontend
automatically falls back to REST polling until SSE recovers.

Page-specific heavier resources (history, clients, interfaces, system, catalog)
are loaded on demand by the route that uses them.

## Development

```sh
cd frontend
npm install
VITE_API_TARGET=http://192.168.10.1:2233 npm run dev
```

## Production build

```sh
sh scripts/build-frontend.sh
go build -tags embed_frontend .
```

The Entware packaging script performs the frontend build automatically when
`frontend/build/index.html` is absent.

## Build-tag split

`frontend_assets_dev.go` is used by normal local Go builds/tests and serves
`frontend/build` from disk.

`frontend_assets_embed.go` is enabled only with `-tags embed_frontend` and embeds
the complete static build into the binary.

This keeps ordinary `go test ./...` independent from Node while guaranteeing that
Entware/release binaries are self-contained.
