# Agent notes: internal/store

<!-- tasktracker:begin -->
- Purpose: maintains the in-memory model of the repository `changes/` tree, watches it for external edits, validates it against the AGENTS.md rules, and performs safe writes.
- Key types: `Store` (cache plus watcher), `Change` (one change directory), `Event` (fs/write notifications), and `Violation` (rule breach); entry points are `Open`, `Reload`, and methods like `MoveTask`, `CreateTask`, `CreateChange`, `SetChangeStatus`.
- Writes are conflict-safe: every mutation re-reads the target file, applies the change to fresh on-disk content, and writes atomically via `model.WriteFileAtomic`; external edits are never clobbered.
- Errors are sentinel-based: unknown change or task IDs return `ErrNotFound`, and writes against unparseable files return `ErrInvalid`. Sequence numbers are never reused (highest existing + 1).
- `Watch` uses fsnotify with a 150ms debounce and ignores `.tt-` temp files; listeners subscribe via `Subscribe`/`Unsubscribe` and receive `fs` and `write` events.
- (2026-09-12-7) Docs validation deliberately lives in `internal/docs.ValidateDocs`, not here, despite the original DOC-07 file list; the store stays focused on the `changes/` contract and gains no docs dependency.
- (manual) `statedir.go` exports `StateDirName` (`.lessmess`) and `MigrateStateDir`, called at CLI startup before `Open`; it renames a legacy `.tasktracker` dir only when `.lessmess` is absent, so an existing current dir always wins and nothing is merged or deleted.
- (2026-09-13-0) `Watch` skips attribute-only events (`ev.Op&^ fsnotify.Chmod == 0`) before the debounce: git scans read modified files, and macOS reports the resulting atime updates as `CHMOD`, which would otherwise retrigger a reload on every index render. Create/Write/Remove/Rename still notify (`TestWatchIgnoresChmod`).
<!-- tasktracker:end -->
