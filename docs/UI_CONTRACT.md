# RouterForge UI Contract

Status: current development contract
Baseline: RouterForge dev after R20 component-rhythm unification
Scope: Management, Monitoring, DNS, App Center, Network Tools

## Purpose

RouterForge modules must look like parts of one product. New UI work must reuse the
canonical tab, header and component geometry instead of introducing a module-local
visual language.

Network Tools is the visual reference for the tab strip. Shared-shell modules use
the common shell CSS; iframe-style modules such as DNS mirror the same geometry.

## 1. Tab strip

Canonical geometry:

- framed dark strip
- 5 px inner padding
- 6 px gap
- 165 px minimum tab width
- 40 px minimum tab height
- 8 px outer radius / 6 px tab radius
- semantic theme accent for the active tab
- horizontal overflow when labels do not fit
- text labels first; decorative glyphs before tab names are not part of the contract

Network Tools top-level tabs are explicitly text-only.

Shared-shell modules should prefer the existing `.subtabs` or `.module-selector`
wiring instead of adding another tab component.

DNS `.module-tabs` and `.parity-subtabs` intentionally mirror the same contract.

## 2. Module header

Canonical geometry:

- title: at least 28 px
- subtitle: at least 13 px
- readable subtitle line length: max 920 px
- consistent kicker / eyebrow rhythm
- right-side action cluster uses standard controls
- responsive stacking below 900 px

Shared-shell modules use `.page-head`.
Network Tools uses `.nt-heading`.
DNS mirrors the same geometry inside its iframe.

## 3. Component rhythm

Canonical geometry:

- panel radius: 8 px
- control radius: 6 px
- standard control/button height: 38 px
- compact controls: 32 px
- panel-header minimum height: 50 px
- table-row rhythm: 42 px
- sticky table headers
- consistent cell padding
- subtle zebra and hover rows
- consistent empty-state spacing
- visible keyboard focus

Theme/accent values must come from RouterForge semantic variables. Do not hard-code
a module-specific green/blue accent as the contract.

## 4. Current module wiring

- Management: `.page-head`, `.subtabs.admin-tabs`
- App Center: `.page-head`, `.subtabs.app-center-tabs`
- Monitoring: `.page-head`, `.module-selector`
- DNS: `.page-head`, `.module-tabs`, `.subtabs.parity-subtabs`
- Network Tools: `.nt-heading`, `.nt-tabs`

Profiling currently has no standalone frontend surface and therefore has no tab
or header contract to render.

## 5. CI guard

`scripts/verify-ui-contract.py` is a structural regression guard. It intentionally
fails when canonical wiring or the accepted R18/R19/R20 geometry disappears.

A deliberate redesign must update this document and the verifier in the same
commit. Silent drift between modules is not accepted.

## 6. Non-goals

This contract does not force every module to have identical content. It standardizes
the frame around content: navigation, headers, panels, controls, tables, empty
states and focus behavior.

Runtime APIs, package topology and module functionality are outside this UI contract.
