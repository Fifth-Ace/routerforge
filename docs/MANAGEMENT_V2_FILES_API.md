# RouterForge Management v2 Files API — Phase 8E / FE-001B

Status: **read-only File Manager backend enabled on Dev**.

This document extends `MANAGEMENT_V2_API.md`.

FE-001A established the filesystem path-security foundation. FE-001B enables the first useful vertical slice: root-session-gated directory listing, bounded UTF-8 text reading and streamed file download. Filesystem mutations are still disabled.

## Ownership

File Manager belongs to the optional `routerforge-admin` / Management module. It does not become a separate RouterForge package.

Package install/update/remove semantics remain owned by App Center and are not duplicated here.

## Authentication boundary

Every File Manager request, including read/list/download, requires a live RouterForge session authenticated as Entware `root`.

Core applies the same-origin check when an `Origin` header is present, strips any browser-supplied internal Admin authorization marker, and injects the canonical marker only after the root-session gate.

The Admin module independently requires that internal marker for all `/v1/files/*` routes. Direct browser access to the Unix-socket backend cannot authorize itself.

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
10. Destructive handlers must revalidate immediately before acting. Path canonicalization is a mandatory barrier, not a claim of complete TOCTOU elimination.

URL decoding happens before the filesystem path resolver. Encoded traversal therefore still arrives at the resolver as a `..` segment and is rejected.

## Enabled FE-001B HTTP surface

```text
GET       /api/modules/admin/files/list?path=<absolute>
GET       /api/modules/admin/files/read?path=<absolute>
GET/HEAD  /api/modules/admin/files/download?path=<absolute>
```

### Directory list

`files/list`:

- requires an existing directory;
- returns bounded metadata only;
- maximum `2048` entries per request;
- directories are sorted before non-directories, then by name;
- returns name, logical path, kind, size, mode and modification time;
- does not expose canonical filesystem paths in the response.

A directory larger than the entry limit returns HTTP `413` instead of silently truncating.

### Text read

`files/read`:

- requires a regular file;
- maximum text payload is `256 KiB`;
- accepts UTF-8 text only;
- NUL-containing or invalid UTF-8 content returns HTTP `415`;
- oversized content returns HTTP `413`;
- returns logical path, size, mode, modification time, encoding and content.

This endpoint is for the future editor/preview UI, not arbitrary binary transport.

### Download

`files/download`:

- requires a regular file;
- streams directly from the filesystem instead of buffering the whole file in RAM;
- supports `GET` and `HEAD`;
- sends `Content-Disposition: attachment`;
- sends `application/octet-stream`;
- sends `X-Content-Type-Options: nosniff`;
- remains behind the root-session and internal-marker boundary.

## Planned mutation surface — not enabled yet

```text
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
- Request sizes are bounded.
- Existing Admin mutation body limit remains `8 KiB` until a specific file-write/upload transport is designed and tested.
- Result state is verified where the operation permits reliable verification.
- Audit data is emitted by the later handler layer.

## FE-001B security/behavior tests

The implementation must prove at least:

- File Manager route detection is segment-exact;
- unauthenticated file access is rejected even when general RouterForge auth is disabled;
- cross-origin file access is rejected when `Origin` is present;
- a spoofed browser marker is replaced by the canonical Core marker only after a root session;
- direct module file access without the marker is rejected;
- directory metadata listing works and is bounded;
- UTF-8 text reading works;
- binary/NUL content is rejected;
- oversized text content is rejected;
- GET/HEAD download works;
- explicit parent traversal remains rejected;
- FE-001A canonical boundary tests remain green.

## Still not enabled

- mkdir/write/move/delete/chmod;
- upload transport;
- advanced system-root mode;
- Terminal/PTTY;
- arbitrary shell execution.

The next step after FE-001B is **FE-001C: mkdir + bounded atomic write/edit foundation**.
