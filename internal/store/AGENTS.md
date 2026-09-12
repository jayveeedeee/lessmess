# Agent notes: internal/store

<!-- tasktracker:begin -->
- Purpose: maintains the in-memory model of the repository `changes/` tree, watches it for external edits, validates it against the AGENTS.md rules, and performs safe writes.
- Key types: `Store` (cache plus watcher), `Change` (one change directory), `Event` (fs/write notifications), and `Violation` (rule breach); entry points are `Open`, `Reload`, and methods like `MoveTask`, `CreateTask`, `CreateChange`, `SetChangeStatus`.
- Writes are conflict-safe: every mutation re-reads the target file, applies the change to fresh on-disk content, and writes atomically via `model.WriteFileAtomic`; external edits are never clobbered.
- Errors are sentinel-based: unknown change or task IDs return `ErrNotFound`, and writes against unparseable files return `ErrInvalid`. Sequence numbers are never reused (highest existing + 1).
- `Watch` uses fsnotify with a 150ms debounce and ignores `.tt-` temp files; listeners subscribe via `Subscribe`/`Unsubscribe` and receive `fs` and `write` events.
- (2026-09-12-7) Docs validation deliberately lives in `internal/docs.ValidateDocs`, not here, despite the original DOC-07 file list; the store stays focused on the `changes/` contract and gains no docs dependency.
<!-- tasktracker:end -->
