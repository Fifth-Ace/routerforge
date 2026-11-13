# RT-C3/C4 — реальный browser/API/IPK execution plan

## P0 / Source and provenance — STOP gate

1. Extract **exact** `repository/` from clean `dev` SHA `d069c1f09f95392368ab0e128857bc5a67e96c75`, not R15. Compare git blob hashes of key files and `FILE_SHA256.tsv`. If remote HEAD drift, STOP; no overlay.
2. Work on isolated staging copy. Original dev tree and user PC service configurations untouched. Full-source ZIP must include module `frontend/`, Go backend, `scripts/`, lockfile, IPK packaging scripts and tests.
3. Record browser/Node/Vite/Go versions. `frontend/package.json` pins `svelte=5.49.2`, `vite=6.4.1`, plugin `6.2.4`; do not silently install global programs. If preinstalled Chromium absent, **BLOCKED**; no substitute regexp PASS.

## P1 / Build and stage the real Svelte UI

- `npm ci` in an isolated runtime module copy against its lockfile (only after local dependency approval); `npm run build`; check compiler warnings and expected `build/svelte-mode.js`, CSS and actual package UI layout. The module's `vite.config.js` uses IIFE `svelte-mode.js` and relative base `./`.
- Stage the **actual** compiled JS/CSS plus `index.html`/`module.css` exactly as the IPK builder would ship, not a handwritten fake DOM. Real Go backend serves `/v1/ui/` and `/v1/config`; browser's relative `../config` request is `/v1/config`.
- Start a disposable localhost-only mock HTTP server on `127.0.0.1:PORT` with `/v1/ui/*` assets and strictly enumerated `/v1/*` fixtures. Never proxy to the router or to any public upstream. Do not let the test harness trigger physical Keenetic mutations.

## P2 / Two complementary **actual** browser modes

**A. Shipped-bundle E2E:** launch already installed Chromium through CDP with a *fresh temporary profile*, load `http://127.0.0.1:PORT/v1/ui/`, collect request/response headers, console errors, DOM snapshots and screenshots. Desktop `1440×900`, mobile `390×844` with device metrics/touch. Assert real Svelte rendering and requests. Use no external network; fail on any request leaving localhost.

**B. Real Svelte mount/unmount component harness:** import the same `DnsRuntimeApp.svelte` in a test-only Vite entry, call `mount(App,{target})`, then `await unmount(instance)`, remount within the **same JS realm**. Merely removing `#app` or navigating away is NOT a genuine Svelte teardown test. Keep production `main.js` unchanged. Delay mock responses to detect requests/state surviving unmount.

Record `CDP command log`, HAR-like JSON, response MIME/body status, page screenshot and exact artifact hashes. Do not mark success just because page loaded; assert state before/after UI interaction.

## P3 / RT-C3 acceptance order

1. `RTC3-01..06` — mount/navigation/tab keyboard/empty/degraded/desktop/mobile/unmount. Oracle: **zero write requests on mount/navigation**.
2. `RTC3-07..20` — draft, save, 403/409/428 invalid JSON, stale PUT, reload cancel, import preview, domain dedup. Make all mutating tests explicit clicks with mocked protected server. Assert `{If-Match: '"r1"', X-RouterForge-Action: 'save'}` and exactly one PUT on double-click. Save r1→r2 must **not** clear a newer dirty generation.
3. `RTC3-21..30` — abort scans and shadow status on teardown, shadow 403/409, confirm refused, single-flight, DOM feedback one-owner. Record exact in-flight request counts after remount. Source findings `RT-RISK-01..04` receive pass/fail only here.
4. `RTC3-31..34` — managed trial one-shot token, durable prepare/confirm/apply, ledger-bound rollback, diagnostics polling cleanup. All are mocked; deny real system/write destinations. Confirmation denied must cause zero mutation. A lost response after apply must **not** auto retry.

For each test run use fresh mock state and fresh browser context; explicitly separate `GET`, `PUT`, `POST`, and `POST` analyses that are semantically read-only. Error body and `Content-Type` are asserted, never translated to false success.

## P4 / RT-C4 backend + package + upgrades

1. `RTC4-01`: Go `api.go` serves actual UI `/v1/ui/`, correct content-type, index points to compiled Svelte IIFE and stylesheet, no stale legacy `app.js` dependency.
2. `RTC4-02..04`: real Go httptest/injected fake interfaces for 403/409/428, CAS, owned status and one-use tokens. Never point protected tests at live Keenetic.
3. `RTC4-05`: on Linux run **real** `go test ./...`, `go test -race ./...`, `go vet ./...` for correct package/module scopes. Windows `GOOS=linux` cross-build is not test execution.
4. `RTC4-06`: inspect actual **experimental** IPK manifest, payload, arch, path, MIME, installed UI version for each supported target (do not assume package count). Compare bundle hash with frontend build from exact SHA.
5. `RTC4-07`: isolated upgrade fixtures OFF→OFF, ON→ON on accepted applied snapshot, dirty draft kept, fail/rollback. Hardware acceptance on physical router remains **BLOCKED by separate authorization**; do not counterfeit with a mock.
6. `RTC4-08`: all 12 RT features have actual evidence; G0–G8 all PASS on one source state. MODULE DONE only after signoff.

## P5 / Fail fast, logs, claims

- First actual FAIL → STOP, save exact case ID, fixture, request trace, HAR, browser console, screenshot, source/asset SHA, Linux/IPK logs and diagnostic directory. No automatic code rewrites, no CI hotfix loop.
- Status classification: PASS only for executed test; FAIL for executed failing assertion; BLOCKED for missing source/tool; PENDING for not executed; SKIPPED mandatory ≠ PASS.
- Deliver a **single signed module ZIP** with `repository/` unchanged at this step, `docs/migration`, `evidence/RTC3-*`, `evidence/RTC4-*`, `MANIFEST.json`, SHA256; then only with approval can propose a guarded source patch (if a confirmed defect) and eventual Git/CI.

## Current execution blocker (updated 2026-11-11)

Exact consolidated source ZIP is uploaded and verified; G0 **PASS**. In this container no npm registry DNS, missing quic-go offline Go module, and Chromium organization policy blocks loopback. Browser and Go actual runs **BLOCKED**; Node 62/62 **PASS**. A real Chrome/CDP localhost mock harness and exact PS5.1 runner are in the staged source package. Do not treat its existence as executed browser PASS.
