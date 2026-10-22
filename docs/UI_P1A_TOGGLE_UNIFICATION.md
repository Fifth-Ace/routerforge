# RouterForge UI P1A — unified binary switches

Status: Dev patch, based on `30b700568f4ddae839043944544259131f387d52`.

## Canonical visual and interaction contract

- For on/off **settings**, use `RfToggle.svelte`: a compact text pill (`ON` / `OFF`) immediately to the left of the slider track, visually based on the existing Security switch.
- One clickable and keyboard-accessible `button[role="switch"]` with `aria-checked`, descriptive `aria-label`, visible focus, disabled / busy states and reduced-motion support.
- Off is neutral dark; on is semantic green. The accent color marks focus / hover, not the enabled state.
- This visual component never calls an API, changes a stored setting, or decides when confirmation is required: the parent supplies the current value and `onToggle(nextValue)` handler.
- Checkboxes remain valid for **multi-select, bulk selection, permissions lists, consent** or other non-binary-selection use cases; do not blindly replace them.

## P1A implementation scope

- `components/core/frontend/src/lib/components/RfToggle.svelte` (new).
- `components/core/frontend/src/routes/settings/+page.svelte` (two existing binary controls):
  1. Technical diagnostics / telemetry on the System panel; preserves `changeTelemetry(nextValue)` and its asynchronous busy state.
  2. Panel authentication on the Security panel; preserves `requestAuthToggle()` including the existing enable confirmation modal and state/error handling.
- The two existing status badges and old checkbox/switch markup are not duplicated.
- All other settings panels, module functionality, persistence, API calls, route navigation, session logic, theme controls and router runtime remain unchanged.

## Next passes in UI Unification

- P2: inventory each boolean setting in Core pages; apply the canonical control only where semantic purpose is a true on/off setting.
- P3/P4: adapt the same contract to the standalone HTML/JS UIs of NFQWS, Antiscan, Port Access and CrowdSec, preserving runtime controls and each module's own lifecycles. External third-party UIs are not rewritten.
- P5: verify keyboard operation, screen readers, `ON/OFF`, busy/disabled states, focus contrast, RU/EN, desktop/tablet/mobile, dark/light themes and CSS isolation.

## Acceptance tests

1. `Settings` contains no legacy `type="checkbox"` for the telemetry feature and no `security-switch` custom button.
2. Exactly two `RfToggle` instances and both handlers remain wired to the original logic.
3. No application/runtime/server changes; changes are scoped to the exact three files in this package.
4. CI `test` and `Publish rolling ARM64 Dev channel` succeed for the exact pushed SHA; protected branches and release assets remain unchanged.
5. Visual review is still required on DSTest after the Dev Core package is installed.
