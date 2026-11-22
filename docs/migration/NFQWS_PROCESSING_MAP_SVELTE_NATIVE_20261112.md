# NFQWS Processing Map — Svelte 5 functional ownership

- **Read-only:** Svelte owns hostname entry, status/list fetch, router-side HTTPS probe, Keenetic projection and strategy inventory. Legacy Processing Map DOM renderer, onclick handlers, guided HTML/timeline renderer, route HTML and temporary NFQUEUE runner removed.
- **Live:** Svelte fetches a current `/status`, verifies exact active config SHA from the earlier map, then sends exactly one explicit `POST /bench-autotune` with `{mode:fast, server_name, expected_config_sha256, confirm:ROUTERFORGE_AUTOTUNE_TLS}`. The backend is responsible for temporary NFQUEUE setup/cleanup. Client never runs an automatically triggered live check.
- **Proof:** response `ok=true`, `cleanup_baseline_after=true`, bidirectional queue packets, and attempt cleanup are required before UI claims a proven candidate. HTTPS availability alone is not packet evidence.
- **Static semantics:** `buildStaticPath` stays in the legacy JS as pure read-only matcher through SHA-scoped `processingStaticPath`, with current config comparison. It is not an imperative UI owner, pending further extraction into a pure model.
- **Svelte:** route overview, static hits (open exact file and line), guided verdicts, actions into Overview/Lists/Test Lab/Preflight, copy JSON/support summary and Stop are owned by NfqwsProcessingMap.svelte.
- **Known gaps:** pinned Vite and browser acceptance pending; strategy graphic parity with historical multi-step route pending; other legacy screens/production CI/IPK/atomic server CAS pending. No MODULE DONE signoff.
