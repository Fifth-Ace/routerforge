# STAGE 1 D / ownership snapshot

See outer handoff `docs/22_STAGE1_PATCH_D_TYPED_UI_AND_FEATURE_STORES.md` and `docs/23_STAGE1_D_ALL11_SOURCE_OWNER_MATRIX.md`.

- Network Tools: 7 Svelte components replace the entire API-to-HTML output pipeline, with GET request abort on unmount. No `{@html}` in frontend Svelte.
- Antiscan: 11 scoped `$state` stores; 133 control defaults, 507 static references and three routed IDs migrated from shared cells. `AntiscanControlDefaults.js` is data-only.
- Source status: Stage 1 **IN PROGRESS**, stage 2/3 **NOT STARTED**. No build/test/CI/router side effects claimed.
