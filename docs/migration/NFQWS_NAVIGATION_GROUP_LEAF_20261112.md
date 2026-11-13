# NFQWS primary/leaf navigation contract — 12 Nov 2026

`strategies` is an overloaded identifier in the historic navigation: a primary group label and the leaf Configuration editor. In this checkpoint `NfqwsNavigationModel.js` separates them explicitly. `navigatePrimary('strategies')` restores a candidate from Strategy Library group; `navigateTab('strategies')` always enters `view-strategies` Configuration page. The model refuses unknown leaf keys, unknown group keys and cross-group history.

To prevent regression the Chromium owner test opens both namespaces and verifies that configuration operations still use their `NFQWS_CONFIG` confirmation token with denied `403/409` results. Its fixture implements backend `GET /v1/v2/strategies` using the exact JSON envelope and never mocks a successful mutation. Related tests: `NfqwsNavigationModel.test.js`, `qa-nfqws/browser-strategy-fixture.test.mjs`.

Confirmed by user Windows: previous bundle fails after 13 browser tests; current source Node and graph PASS, new bundle/browser PENDING. Runtime commands, CAS, backups, Smart Apply, service autostart, rollback and router not touched. Full NFQWS ownership remains incomplete.
