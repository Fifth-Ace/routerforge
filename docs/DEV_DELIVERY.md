# RouterForge development delivery

This document defines the normal `dev` patch path.

## Bundle format

A development bundle contains:

```text
manifest.json
run.ps1
payload/
  path/to/changed-file
  ...
```

`payload/` contains the complete final contents of every changed/added file. The runner
does not patch source text, search for anchors, or perform string replacement.

`manifest.json` pins:

- repository;
- branch;
- exact base SHA;
- commit message;
- exact payload file list;
- optional exact deletion list.

`run.ps1` is the Windows PowerShell 5.1 delivery runner. The canonical copy is
`scripts/dev-bundle-runner.ps1`.

## Normal flow

The runner performs only:

1. verify `git`, `gh`, authentication, payload scope, and exact remote base SHA;
2. clone `dev` with `core.autocrlf=false`;
3. copy the final payload files into the clone;
4. stage only the exact manifest file list;
5. run `git diff --cached --check` and verify exact staged scope;
6. commit and push;
7. discover the single GitHub Actions push run for the exact commit SHA;
8. `gh run watch <exact-run-id> --exit-status`;
9. verify exact run SHA, successful conclusion, and final remote branch SHA.

No local Go/Node/Python build or test is required. GitHub Actions Linux is the build/test
authority.

Transient Git/GitHub transport failures are retried where observation is involved. A push
transport error is not treated as a failed push when the remote branch already equals the
exact new commit.

## Failure rule

A development bundle stops on the first real failure.

After a commit has reached GitHub, do not replay the bundle blindly. Read remote truth
first and continue from the exact commit/run boundary.

## CI scope

Ordinary development CI runs contract checks only for the areas touched by the commit.
Full-release dispatch remains the exhaustive validation path.

Remote upstream drift checks are release/audit gates, not blockers for unrelated
frontend or application work.

## Release flow

Release preparation and Stable/Beta publication are separate from normal development
bundles. A development bundle never promotes `main`, publishes Stable, or mutates Beta.
