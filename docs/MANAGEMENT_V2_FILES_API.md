# RouterForge Management v2 Files API — Phase 8E / FE-001C

Status: **read-only File Manager backend + guarded mkdir/atomic text write-edit implemented on Dev**.

This document extends `MANAGEMENT_V2_API.md`.

FE-001A established the filesystem path-security foundation. FE-001B enabled root-session-gated directory listing, bounded UTF-8 text reading and streamed file download. FE-001C adds the first guarded filesystem mutations: single-directory creation and bounded atomic UTF-8 text create/edit.

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

## Enabled FE-001C mutation surface

```text
POST  /api/modules/admin/files/mkdir
POST  /api/modules/admin/files/write
```

### mkdir

`files/mkdir`:

- requires the same live Entware `root` session and canonical internal marker as other Admin mutations;
- creates exactly one directory; recursive parent creation is not part of FE-001C;
- requires exact `confirm_path == path`;
- applies the FE-001A canonical root/symlink boundary before acting;
- returns conflict when the target already exists;
- creates with mode `0755` subject to the platform umask;
- verifies that the directory exists after creation;
- emits a success audit log entry.

### atomic text create/edit

`files/write`:

- accepts strict JSON only;
- maximum decoded UTF-8 text content is `128 KiB`;
- dedicated Core/Admin request-body limit is `272 KiB` only for the exact file-write route;
- all other Admin mutation routes retain the existing `8 KiB` body limit;
- requires exact `confirm_path == path`;
- new-file creation requires `"create": true`, rejects an existing target, and creates with mode `0600`;
- editing an existing file requires both `expected_size` and `expected_mtime_ns`;
- stale size/mtime preconditions return HTTP `409` without replacing the file;
- missing edit preconditions return HTTP `428`;
- leaf symlinks are rejected for write/edit;
- writes to a temporary file in the canonical target directory and replaces with same-filesystem `rename`;
- existing mode and ownership are preserved before replacement;
- temporary data is `Sync`ed before close/rename;
- target path and precondition are revalidated immediately before rename;
- successful replacement is verified and audited.

The FE-001C atomic rename narrows partial-write risk but is not claimed to eliminate every privileged local TOCTOU race. Stronger descriptor-relative/openat-style hardening remains a later security option if measurements and threat model justify the added implementation cost.

## Remaining planned mutation surface

```text
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
- Generic Admin mutations retain the `8 KiB` body limit. FE-001C defines a route-scoped `272 KiB` request limit for `/files/write`, with decoded text content capped at `128 KiB`.
- Result state is verified where the operation permits reliable verification.
- Audit data is emitted by the later handler layer.

## FE-001C security/behavior tests

The implementation must preserve all FE-001A/FE-001B tests and additionally prove at least:

- mkdir requires exact confirmation and creates one directory only;
- write/create is bounded and uses mode `0600`;
- edit requires size + mtime preconditions;
- stale edit returns conflict without changing the file;
- existing file mode is preserved across atomic replacement;
- leaf-symlink write is rejected;
- explicit parent traversal remains rejected;
- strict JSON rejects unknown write fields;
- Core applies the larger body limit only to the exact file-write route;
- oversized file-write request bodies are rejected.

## Still not enabled

- move/delete/chmod;
- generic binary upload transport;
- archive/extract;
- advanced system-root mode;
- Terminal/PTTY;
- arbitrary shell execution.

The next step after FE-001C is **FE-001D: move/rename + delete + chmod guarded mutations**, then FE-002 File Manager UI.
