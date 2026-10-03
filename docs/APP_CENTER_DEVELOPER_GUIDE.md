# RouterForge App Center — verification for developers

RouterForge uses **persistent project verification**.

A GitHub project is verified once. Routine releases, version changes and edits to App Center metadata do not require another verification.

## Quick start

### 1. Add a verification marker

Create `.routerforge/verify.json` in the default branch:

```json
{
  "schema_version": 1,
  "project": "OWNER/REPOSITORY"
}
```

Then open **Verify app in RouterForge App Center** in `Fifth-Ace/routerforge`.

RouterForge checks that the marker exists in the repository and records the binding between the publisher identity and GitHub `OWNER/REPOSITORY`.

### 2. Optionally customize the App Center card

This step is optional.

Without `.routerforge/manifest.json`, RouterForge can continue using an automatically discovered or RouterForge-curated card.

If you want to control the card, add `.routerforge/manifest.json`. A template is available in `marketplace/developer-kit/manifest.json`.

The manifest can evolve with the project. Editing it does not reset `VERIFIED`.

## What VERIFIED means

`VERIFIED` confirms the identity binding between:

- the App Center publisher;
- the GitHub repository.

It is project-level verification, not release-level verification.

A normal application update, new GitHub Release, changed binary asset, description edit or card redesign does not require another verification.

The current manifest SHA256 may still be recorded as audit/provenance metadata, but it is not the identity credential for project verification.

## What can require verification again

- repository transfer to another GitHub owner;
- publisher ID change;
- project deletion/recreation under a different identity;
- verification revocation;
- a security or ownership incident.

## Lifecycle remains separate

A verified project does not gain unrestricted execution authority.

Install/update/remove actions continue to be controlled by RouterForge lifecycle validation and safety policy.

## Templates

`marketplace/developer-kit/verify.json` — one-time project ownership marker.

`marketplace/developer-kit/manifest.json` — optional App Center card template.
