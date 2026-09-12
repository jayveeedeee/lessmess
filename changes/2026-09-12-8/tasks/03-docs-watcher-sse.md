---
id: EXP-03
title: Docs watcher and SSE live refresh
---

# EXP-03: Docs watcher and SSE live refresh

Status: see [../ledger.md](../ledger.md).

## Objective

Live-update the explorer: watch the covered directories with fsnotify, emit
`docs` events merged into the existing `/events` SSE stream, and refresh the
tree fragment client-side (preserving expansion state) — while other pages
ignore `docs` events.

## Dependencies

- EXP-01 (fragment to refresh)

## Scope

- `internal/server/docswatch.go`: fsnotify over covered dirs; rebuild the watch
  set on structural events (dir add/remove); 300ms debounce; started in `New`
  when docs are enabled, stopped in `Close`.
- `/events`: merge store events and docs-watcher events; emit kind `docs`.
- `app.js`: on `docs` events, explorer page htmx-swaps the tree fragment and
  restores open `<details>` by path; non-explorer pages skip `docs` events.
- Tests for watcher debounce/rebuild; no store dependency (watcher lives in
  server).

## Implementation steps

1. Watcher with debounce + watch-set rebuild (model on internal/store/watch.go
   conventions; covered-set via `docs.Walk`).
2. SSE merge in the events handler (select over both channels).
3. JS filtering + refresh + open-state preservation.
4. Tests: file write triggers debounced event; dir add rebuilds watch set;
   disabled docs → no watcher.

## Verification

- `go test ./internal/server` passes; manual: gardener/seed run visibly
  refreshes the explorer without a page reload.

## Completion criteria

- Live refresh works end-to-end; board/index do not react to `docs` events.

## Files affected

- `internal/server/docswatch.go` (new)
- `internal/server/docswatch_test.go` (new)
- `internal/server/server.go` (events merge, lifecycle)
- `web/static/app.js`

## Notes

- 2026-09-12 — Implemented in `internal/server/docswatch.go`: fsnotify over the
  covered dirs (set synced from `docs.Walk`), 300ms quiet-period debounce,
  watch-set resync after every quiet period (picks up added/removed covered
  dirs), ignores `.tt-*` temp files and hidden entries. Merged into `/events`
  as kind `docs`; `app.js` refreshes `#explorer-tree` via `/explorer/tree` and
  restores open `<details>` by `data-rel`; non-explorer pages return early.
  Watcher starts in `New` only when docs are enabled; `Close` stops it.
  Decision: the SSE stream is not unit-tested (blocking stream; merge is a
  5-line select case) — covered by watcher unit tests + EXP-04 manual check.
- Verification evidence: `go test ./internal/server -count=1` — write in a
  covered dir emits, temp/hidden writes are ignored, new covered dir is picked
  up after resync (write inside it fires). `gofmt`/`go vet` clean; full suite
  green.