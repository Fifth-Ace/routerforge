# RouterForge Management v2 Files API

Status: **Stable 0.7.1 — backend and File Manager UI enabled**.

## Boundary
Every route requires live Entware-root session + Core internal marker. Approved roots: `/opt`, `/tmp`.

Paths must be absolute; NUL and explicit `..` are rejected; canonical symlink resolution must remain within the same allowed root; destructive handlers revalidate immediately before acting.

## Read

```text
GET       /api/modules/admin/files/list
GET       /api/modules/admin/files/read
GET/HEAD  /api/modules/admin/files/download
```

List bounded to 2048 entries; text read UTF-8 ≤256 KiB; downloads stream.

## Mutations

```text
POST /api/modules/admin/files/mkdir
POST /api/modules/admin/files/write
POST /api/modules/admin/files/move
POST /api/modules/admin/files/delete
POST /api/modules/admin/files/chmod
```

Write/edit uses bounded strict JSON, exact confirmation, size+mtime preconditions for edits and atomic same-filesystem rename. Move is same-filesystem rename. Delete is non-recursive. Chmod accepts normal permission bits only.

## UI
Commander/Explorer, folder tree, volumes, UTF-8 editor, create/rename/move/delete/download, properties, visual chmod.

## Not enabled
recursive copy/delete, arbitrary binary upload, archive/extract, chown, arbitrary system-root mode, arbitrary shell execution.
