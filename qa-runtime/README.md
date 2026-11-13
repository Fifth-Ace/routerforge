Real Svelte CDN-free localhost CDP regression. It requires production+component bundles produced by the pinned module npm/Vite build. `node qa-runtime/browser-e2e.mjs` is test-only and never shipped to IPK. Expected browser policy block is reported as BLOCKED, not a product FAIL.

## CDP environmental injection isolation (2026-11-11)

The Windows test host injects AdGuard content/user scripts through `local.adguard.org` despite the runner's clean Chrome profile and `--disable-extensions`. This is not a RouterForge endpoint. The runner now enables CDP Fetch request interception *before* navigation, continues only the exact ephemeral `http://127.0.0.1:<fixture-port>` origin, and fails all other network requests **before the wire**. Only intercepted, known AdGuard URLs whose initiator is not the compiled Svelte bundle can be counted as `ENV_INJECTION_BLOCKED`; unknown external URLs remain `FIRST_REAL_FAIL`. Request URLs in evidence have query strings redacted. The `browser-network-boundary.test.mjs` suite covers prefix-spoofing, unknown endpoints, Svelte attribution, and redaction. The test runner makes no changes to installed AdGuard or browser policy.

## RTC3-02 CDP expression syntax regression (2026-11-11)

Windows actual Chromium reached `interactionPreflight()` but stopped on `BROWSER_JS: SyntaxError: missing ) after argument list` before any Runtime interaction. Root cause was a test-only template expression in `CDP.point()`: an unquoted `ELEMENT_DISABLED:` label became invalid browser JavaScript. No RouterForge production defect was demonstrated.

The browser helper now imports `pointExpression` from `browser-interaction-expression.mjs`. All selector and error literals are JSON-quoted. `browser-interaction-expression.test.mjs` uses Node `vm.Script` to compile BOTH enabled and disabled generated expressions and executes nine real semantic cases (visible, quote escaping, hidden ancestors, disabled, obscured, offscreen, missing, invalid input). This test runs in the initial Node gate BEFORE `npm ci`/Chromium, preventing the same syntactic regression from reaching Windows CDP. Network isolation remains unchanged.
