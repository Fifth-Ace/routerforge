# Native UI ownership / full consolidated source checkpoint

This is a **functional code transfer checkpoint**, not MODULE DONE.

- Source base: `d069c1f09f95392368ab0e128857bc5a67e96c75` (remote dev); old R15 overlay forbidden.
- DNS Lite: P25/P27–31/P24x configuration state, DNS server editor and scanner state, DNS source selections, logs, shell/page visibility, iframe resize and lifecycle migrated to Svelte-owned state/components. Legacy controller no longer reads/writes form elements via `document.getElementById`.
- NFQWS: service status, nav, logs, Lua, DPI streaming, overview, Backup Center, list sources and Smart Apply UI now Svelte-owned. Legacy editor/strategy panels remain, source explicitly retained.
- Backend API/command semantics not rewritten; protected actions remain explicit and CAS-bound.
- No autostart changes, backend changes, Git commits, CI runs, packages published or router changes.
- Evidence: 47 new dynamic Node tests PASS; local Svelte compiler 0 warnings; pinned Vite and actual cross-browser/Go/IPK/upgrade new-slice checks PENDING.
