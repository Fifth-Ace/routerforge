# RouterForge development delivery

This document defines the normal `dev` patch path.

## Bundle format

A development bundle contains exactly:

```text
manifest.json
change.patch
run.ps1
```

`change.patch` is a unified Git patch generated against one exact `dev` SHA.
There is no source-code search/replace logic in the bundle.

`manifest.json` pins:

- repository;
- branch;
- exact base SHA;
- commit message;
- exact list of files allowed to change.

`run.ps1` is the Windows PowerShell 5.1 delivery runner. The canonical copy is
`scripts/dev-bundle-runner.ps1`.

## Normal flow

The runner performs only:

1. verify `git`, `gh`, authentication and exact remote base SHA;
2. clone `dev` with `core.autocrlf=false`;
3. `git apply --check`;
4. `git apply --index`;
5. verify exact staged file list and `git diff --cached --check`;
6. commit and push;
7. discover the single GitHub Actions push run for the exact commit SHA;
8. `gh run watch <exact-run-id> --exit-status`;
9. verify exact run SHA, successful conclusion and final remote branch SHA.

No local Go/Node/Python build or test is required. GitHub Actions Linux is the build/test
authority.

Transient GitHub/API failures are retried. A push transport error is not treated as a
failed push when the remote branch already equals the exact new commit.

## Failure rule

A normal development bundle stops on the first real failure.

After a commit has reached GitHub, do not replay the bundle blindly. Read remote truth
first and continue from the exact commit/run boundary.

## CI scope

Ordinary development CI runs contract checks only for the areas touched by the patch.
Full-release dispatch remains the exhaustive validation path.

Remote upstream drift checks are release/audit gates, not blockers for unrelated
frontend or application work.

## Release flow

Release preparation and Stable/Beta publication are separate from normal development
bundles. A development bundle never promotes `main`, publishes Stable, or mutates Beta.
