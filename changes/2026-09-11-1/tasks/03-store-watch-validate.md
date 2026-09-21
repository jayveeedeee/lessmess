
# KAN-03: Store — scan, cache, watch, validate

Status: see [../ledger.md](../ledger.md).

## Objective

Implement `internal/store`: the in-memory model of a `changes/` tree with scanning, fsnotify-based invalidation, safe write operations, and the seven-rule validation contract from `AGENTS.md`.

## Dependencies

KAN-02.

## Scope

In scope: `Store` type with `Load(dir)`, cached model (root ledger + per-change ledgers + task metadata), `Watch()` via fsnotify with ~150ms debounce and per-file invalidation, a notification channel for SSE, write operations (`MoveTask`, `CreateTask`, `CreateChange` incl. zero-based number allocation per naming rules, `ArchiveChange` excluded), pre-write freshness check (re-stat + one rescan-retry, then conflict error), `Validate()` implementing all seven `AGENTS.md` rules, path-traversal guard confining operations to `changes/`.
Out of scope: HTTP/SSE delivery (KAN-04), CLI validate wiring (KAN-06).

## Implementation steps

1. `Load`: walk `changes/` (skipping `archive/` for the active set but indexing it), parse all files, build model.
2. `Validate`: rules 1–7 from `AGENTS.md` "Validation" section, returning structured violations.
3. `Watch`: fsnotify on `changes/` recursively, debounce, rescan changed files, publish events.
4. Write ops with freshness check + atomic write + cache update + event publish.
5. Tempdir tests: scan, validate (one violation per rule), watch event on external edit, conflict path, allocation (incl. gap rule — no reuse).

## Verification

- `go test ./internal/store` passes, including a real fsnotify round trip (write file externally → event received) and the freshness-conflict retry path.

## Completion criteria

- Store loads this repository's real `changes/` tree with zero validation violations, watches it, and performs all write operations safely in tempdir tests.

## Files affected

- `internal/store/` (new)

## Notes

- Keep `Validate()` read-only; it reports, never repairs.
