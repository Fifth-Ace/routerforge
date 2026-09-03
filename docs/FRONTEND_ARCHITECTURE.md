# RouterForge frontend architecture

## Stack

Production frontend:

- Svelte 5;
- SvelteKit;
- Vite;
- `@sveltejs/adapter-static`.

Node.js используется только на development/CI машине.

Production build встраивается в RouterForge Core через Go build tag `embed_frontend`.

## Runtime

```text
RouterForge Core :2233
├── REST API
├── SSE /api/events
└── Embedded SvelteKit build
```

Shared `+layout.svelte` держит долгоживущие stores и snapshot stream между route transitions.

При недоступном SSE frontend может использовать REST polling.

## Capability-driven navigation

Primary navigation формируется из фактически установленных capabilities.

Базовые items:

```text
/           Главная
/catalog    Marketplace
/settings   Настройки
```

Conditional:

```text
/monitoring   если установлен system/thermal/storage/network
/dns          если установлен RouterForge DNS
/manage       если установлен RouterForge Control
```

Registry presentation metadata может добавлять navigation entry для установленного official module.

## Catalog

`catalog` store периодически перечитывает локальный `/api/catalog`.

Это **не равно remote GitHub poll**: Core отдельно throttles Registry/release-index sync.

Manual Marketplace update check использует:

```text
POST /api/catalog/refresh
```

Endpoint форсирует remote Registry + release-index refresh и возвращает пересчитанный catalog.

## Authentication gate

Layout сначала получает `/api/auth/status`.

Если auth required и session отсутствует, основной shell не монтируется; показывается AuthGate.

После login обычные protected API calls работают через `HttpOnly` session cookie.

## Development

```sh
cd frontend
npm install --no-audit --no-fund
VITE_API_TARGET=http://192.168.10.1:2233 npm run dev
```

Используйте только тестовый роутер для development target.

## Checks

```sh
npm run check
npm run build
```

## Production build

```sh
sh scripts/build-frontend.sh
go build -tags embed_frontend .
```

Entware/release build самодостаточен: frontend assets находятся внутри Core binary.
