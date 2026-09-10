# RouterForge Management v2 Files API — Phase 8E / FE-001A

Status: **contract + path-security foundation**.

This document extends `MANAGEMENT_V2_API.md`. FE-001A adds only the filesystem path model and its security tests. **No File Manager HTTP routes or filesystem mutations are enabled by FE-001A.**

## Ownership

File Manager belongs to the optional `routerforge-admin` / Management module. It does not become a separate RouterForge package.

Package install/update/remove semantics remain owned by App Center and are not duplicated here.

## Authentication boundary

Every File Manager request, including future read/list/download requests, must require a live RouterForge session authenticated as Entware `root`.

The browser must never be able to grant itself the Core-to-module authorization marker. Core must strip an incoming marker and inject its canonical internal marker only after the root-session gate.

FE-001A does not change Core routing yet; this requirement becomes an implementation gate before the first File Manager HTTP endpoint is enabled.

## MVP path roots

Initial allowed filesystem roots:

- `/opt`
- `/tmp`

Mounted writable media already exposed below `/tmp` is therefore inside the initial path boundary. Additional roots require a later explicit policy decision; they are not inferred from arbitrary user input.

System-root mutation outside approved roots is not part of the MVP.

## Path model

Before any File Manager operation:

1. Path must be absolute.
2. NUL is rejected.
3. Any explicit `..` path segment is rejected before lexical cleaning.
4. Lexically cleaned path must be inside one configured allowed root. Prefix lookalikes such as `/opt-other` are outside `/opt`.
5. The matched allowed root is canonicalized with symlink evaluation.
6. Existing targets are canonicalized with symlink evaluation and must remain inside the canonical form of the same allowed root.
7. For a target that may not exist yet, its parent must exist, is canonicalized with symlink evaluation, and must remain inside the canonical allowed root.
8. A symlink that resolves outside the matched allowed root is rejected.
9. Canonical paths that remain inside the same allowed root are permitted.
10. Destructive handlers must revalidate immediately before acting. FE-001A path canonicalization is a mandatory barrier, not a claim of complete TOCTOU elimination.

URL decoding happens before the filesystem path resolver. Encoded traversal therefore must still arrive at the resolver as a `..` segment and be rejected.

## Planned MVP HTTP surface

These routes are **planned**, not enabled by FE-001A:

```text
GET   /api/modules/admin/files/list?path=<absolute>
GET   /api/modules/admin/files/read?path=<absolute>
GET   /api/modules/admin/files/download?path=<absolute>

POST  /api/modules/admin/files/mkdir
POST  /api/modules/admin/files/write
POST  /api/modules/admin/files/move
POST  /api/modules/admin/files/delete
POST  /api/modules/admin/files/chmod
```

`copy`, archive/extract, optional `chown`, advanced system-root mode and richer upload workflows are later layers.

## Mutation contract requirements

Before enabling each mutation route:

- HTTP method is fixed; no arbitrary operation name.
- Live Entware `root` session is mandatory.
- Core internal authorization marker is mandatory.
- JSON uses strict decoding with unknown fields rejected.
- Target path is confirmed exactly in the request body for destructive operations.
- No shell command string is constructed from request input.
- Request and response sizes are bounded.
- Existing Admin mutation body limit remains `8 KiB` until a specific file-write/upload transport is designed and tested; FE-001A does not silently enlarge it.
- Result state is verified where the operation permits reliable verification.
- Audit data is emitted by the later handler layer.

## FE-001A tests

The foundation must prove at least:

- nested allowed path accepted;
- relative path rejected;
- explicit parent traversal rejected;
- NUL rejected;
- prefix-sibling root escape rejected;
- canonical existing-target escape rejected;
- creatable target through escaping canonical parent rejected;
- missing leaf under an allowed existing parent accepted;
- canonical target that remains inside the allowed root accepted.

## Not enabled by FE-001A

- directory listing API;
- file read/download API;
- mkdir/write/move/delete/chmod API;
- upload transport;
- advanced root mode;
- Terminal/PTTY;
- arbitrary shell execution.

The next step after FE-001A is **FE-001B: root-session-gated list/read/download vertical slice**.
