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
/apps       Центр приложений
/catalog    compatibility redirect
/settings   Настройки
```

Conditional:

```text
/monitoring   если установлен RouterForge Monitoring (или обнаружен legacy split monitoring)
/dns          если установлен RouterForge DNS
/manage       если установлен RouterForge Control
```

Registry presentation metadata может добавлять navigation entry для установленного official module.

## Центр приложений / Catalog compatibility

`catalog` store периодически перечитывает локальный `/api/catalog`.

Это **не равно remote GitHub poll**: Core отдельно throttles Registry/release-index sync.

Ручная проверка обновлений в Центре приложений использует:

```text
POST /api/catalog/refresh
```

Endpoint форсирует remote Registry + release-index refresh и возвращает пересчитанный catalog.

## Authentication gate

Layout сначала получает `/api/auth/status`.

Если auth required и session отсутствует, основной shell не монтируется; показывается AuthGate.

После login обычные protected API calls работают через `HttpOnly` session cookie.

## Source workspaces

Core UI:

```text
components/core/frontend/
```

Standalone Module ABI UIs:

```text
modules/dns/frontend/
modules/admin/frontend/
modules/monitoring/frontend/
```

Module UIs own their source/build workspaces. Shared RouterForge frontend helpers remain owned by Core and are imported through `$lib` aliases at Vite build time; the resulting bundles are self-contained on the router.

Shared request/polling primitives:

```text
components/core/frontend/src/lib/http.js
components/core/frontend/src/lib/polling.js
```

Periodic async work must use serial polling rather than raw async `setInterval`. Read requests use bounded AbortController timeouts; mutation timeout policy is endpoint-specific and must not create false failure after a router-side commit.

## Development

Core:

```sh
cd components/core/frontend
npm install --no-audit --no-fund
VITE_API_TARGET=http://192.168.10.1:2233 npm run dev
```

DNS module UI:

```sh
cd modules/dns/frontend
npm install --no-audit --no-fund
npm run dev
```

Control module UI:

```sh
cd modules/admin/frontend
npm install --no-audit --no-fund
npm run dev
```

Monitoring module UI:

```sh
cd modules/monitoring/frontend
npm install --no-audit --no-fund
npm run dev
```

Используйте только тестовый роутер для development target.

## Checks / production build

```sh
sh scripts/build-frontend.sh
sh scripts/build-dns-frontend.sh
sh scripts/build-admin-frontend.sh
sh scripts/build-monitoring-frontend.sh
go build -tags embed_frontend ./components/core
```

Entware/release build самодостаточен: Core frontend assets встраиваются в Core binary, а standalone module UIs кладутся в соответствующие IPK. Node.js на роутере не нужен.
