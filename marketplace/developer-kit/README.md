# RouterForge App Center — Developer Kit

Verification is intentionally simple.

## 1. Verify the project once

Copy `verify.json` to your repository as:

`.routerforge/verify.json`

Edit only the repository identity:

```json
{
  "schema_version": 1,
  "project": "OWNER/REPOSITORY"
}
```

Then open **Verify app in RouterForge App Center** in the RouterForge issue tracker.

RouterForge verifies the GitHub project identity once. Normal releases, version changes and App Center card edits do not require re-verification.

Verification is bound to:

- GitHub `OWNER/REPOSITORY`;
- the publisher ID used by the App Center entry.

A repository transfer to another owner, publisher identity change, revocation or a security/ownership incident may require a new verification.

## 2. Custom App Center card is optional

You do not have to maintain RouterForge-specific metadata.

If `.routerforge/manifest.json` is absent, RouterForge can keep using the card it already discovered or curated from public project metadata.

If you want to control the card, copy the supplied `manifest.json` template to:

`.routerforge/manifest.json`

You can provide the display name, description, category, compatibility and other supported metadata.

Changing this manifest does **not** remove project verification.

## 3. VERIFIED is not permanent shell permission

`VERIFIED` confirms project/publisher identity.

It does not grant permanent permission to execute arbitrary install/update/remove code. Lifecycle actions continue to use RouterForge lifecycle safety rules.
