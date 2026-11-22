# NFQWS Svelte 5: observed targets, target memory and Auto Setup reactivity (2026-11-12)

BASE: `d069c1f09f95392368ab0e128857bc5a67e96c75` / `dev` only.

- `NfqwsObserved.svelte` owns the inventory, explicit scan, ignore and SHA-checked `user.list` append, with two-step confirmation and cancellation on unmount.
- `NfqwsTargetMemory.svelte` owns history, filters, policy, explicit two-step clearing with backend confirmation token. The old DOM renderer and handlers are removed.
- `NfqwsAutoSetup.svelte` renames the `state(...)` function that masked Svelte 5's `$state` rune and caused `store_rune_conflict` / `non_reactive_update`.
- All previous Selector / Sandbox / Config Library / List Editor patches are preserved.
- Strict build runner refuses every Svelte compiler warning including cases where Vite exits successfully.
- Node 168/168 PASS locally. New pinned Vite and browser: PENDING until Windows runner.
- Legacy controllers remain for not-yet-converted screens, full MODULE DONE PENDING.
- No push, commit, CI, release, tags, or router updates.
