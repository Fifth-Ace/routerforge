# ADR: App Center release metadata and OPKG feed authority

Status: Accepted for Core 0.6 beta

## Context

RouterForge already publishes two package-distribution views:

1. The RouterForge release index used by Core to select an exact package asset, SHA256, release channel and target.
2. `Packages` / `Packages.gz`, generated for OPKG-compatible package feeds.

Both contain useful metadata, but they have different trust and lifecycle semantics. The release index is already the executable contract used by Core for RouterForge-managed packages. Generic OPKG metadata is broader and is also used for Entware and third-party integrations.

## Decision

The RouterForge release index remains the authoritative executable source for RouterForge-managed package lifecycle in Core 0.6.

Each newly built RouterForge release entry may include:

- `architecture`
- `size_bytes`
- `installed_size_bytes`
- `depends`
- `conflicts`
- existing version, asset, SHA256 and minimum Core version fields

The metadata is derived from the built IPK itself. `size_bytes` is the exact published candidate file size. `installed_size_bytes` is the sum of regular-file payload sizes from `data.tar.gz`.

For a same-version candidate, the merge step keeps the already-published asset, SHA256 and byte-derived size metadata. A rebuild is not assumed byte-identical. Safe declarative metadata (`architecture`, `depends`, `conflicts`) may be refreshed without republishing the package.

`Packages.gz` remains a parallel OPKG-compatible view. It is not promoted to the RouterForge Core authority during Core 0.6.

## Consequences

- App Center can show architecture, download size, installed payload size and dependencies before installation.
- Core still verifies the exact RouterForge asset by SHA256 from the release index.
- Generic Entware packages continue to use configured OPKG feeds and `opkg info/status`.
- No duplicate runtime package database is introduced.
- A future migration toward consuming `Packages.gz` more directly can be evaluated without breaking the current release pipeline.

## External web application groundwork

App Center manifest v1 gains typed `web` metadata with only:

- scheme: `http` or `https`
- numeric port
- path beginning with `/`
- embed preference as a boolean

There is deliberately no arbitrary upstream URL, host override, reverse-proxy target or credential field. Phase 6 embedding still requires the separate session request-token / Origin security gate before unrestricted iframe work proceeds.
