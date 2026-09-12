---
id: CMT-05
title: Fix watcher CHMOD loop triggered by git scans
---

# CMT-05: Fix watcher CHMOD loop triggered by git scans

Status: see [../ledger.md](../ledger.md).

## Objective

Fix the perpetual index-page reload loop exposed by CMT-02: rendering the
index runs `gitStatus`, and git's content reads of modified files under
`changes/` surface through fsnotify as attribute-only `CHMOD` events, which
the store watcher treated as content changes — so every index render
retriggered a reload of every open index page, forever.

## Dependencies

- CMT-02 (the loop is an interaction between its render-time `gitStatus`
  and the pre-existing watcher behavior)

## Scope

- `internal/store/watch.go`: skip attribute-only events
  (`ev.Op&^ fsnotify.Chmod == 0`) before the debounce; content ops
  (Create/Write/Remove/Rename) still notify.
- `internal/server/docswatch.go`: same guard, so read-heavy scans
  (`git status`, `ValidateDocs` walks) stop spamming docs events and the
  extra `checkValidation()` per cycle.
- `internal/store/store_test.go`: `TestWatchIgnoresChmod` — a real
  `os.Chmod` on a watched file must not notify; a content append must.
- Remove the temporary `DEBUG watch event` log line added during diagnosis
  (done as part of the guard edit).

## Implementation steps

1. Add the CHMOD guard in both watchers with a comment explaining the
   atime/fsnotify interaction.
2. Add the regression test.
3. Rebuild, restart the :9090 server, verify: (a) six full page-load cycles
   against this dirty repo produce zero `fs`/`docs` events; (b) a real
   append to a `changes/` file still produces an `fs` event.

## Verification

- `TestWatchIgnoresChmod` and `TestWatchEvent` pass; full `go test ./...`
  green.
- Live on the dirty repo: 6 cycles → 0 fs / 0 docs events; real write →
  1 fs event.

## Completion criteria

- No reload loop on the changes page with a dirty working tree; genuine
  `changes/` edits still live-reload clients.

## Files affected

- `internal/store/watch.go`
- `internal/store/store_test.go`
- `internal/server/docswatch.go`

## Notes

- Diagnosis evidence: debug logging showed bursts of
  `op=CHMOD` events for exactly the files listed as modified by
  `git status` (`changes/2026-09-12-12/*`, `changes/ledger.md`), one burst
  per page cycle. `git diff --stat HEAD` always re-reads modified file
  contents; on macOS the resulting atime updates surface via kqueue as
  `NOTE_ATTRIB`, which fsnotify reports as `Chmod`.
- Why it only appeared now: pre-change index renders never ran git, so
  renders never read the tree; with a clean repo the loop also cannot
  start (nothing for git to read). It needs the combination of the new
  render-time `gitStatus` and a dirty tree containing `changes/` files.
- Loop anatomy: render → git reads → atime → CHMOD events → watcher
  notify → `location.reload()` on index → render → … Board pages were
  unaffected (fragment refresh), explorer ignores `fs` events.
- The debug log line used during diagnosis was removed in the same edit
  that added the guard; no instrumentation remains.
