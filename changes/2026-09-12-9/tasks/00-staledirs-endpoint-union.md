---
id: REF-00
title: StaleDirs helper and /docs/refresh union
---

# REF-00: StaleDirs helper and /docs/refresh union

Status: see [../ledger.md](../ledger.md).

## Objective

Add `docs.StaleDirs(root)` returning covered dirs whose STRUCTURE.md freshness
hash lags the current tree (or whose meta is missing), and make
`POST /docs/refresh` enqueue one manual job for the union of queue-stale and
hash-stale dirs.

## Dependencies

- —

## Scope

- `StaleDirs` in `internal/docs` (walk + `TreeHashes` + per-dir meta read),
  sorted output; missing STRUCTURE.md/meta counts as stale.
- Endpoint change: union, sorted; empty union → `{"status":"nothing to refresh"}`;
  success → 202 `{"enqueued":[dirs]}` as today.
- Tests for both.

## Implementation steps

1. `StaleDirs` in `internal/docs/validate.go` (+ tests on a seeded fixture:
   clean after seed; stale after file add; stale when meta missing).
2. Update `docsRefresh` handler in `internal/server/docsqueue.go` to merge
   `q.staleDirs()` with `docs.StaleDirs(s.st.Dir)` (dedupe, sort).
3. Endpoint tests: union enqueued; empty → status message.

## Verification

- `go test ./internal/docs ./internal/server` passes.

## Completion criteria

- The endpoint reconciles both staleness kinds with one job.

## Files affected

- `internal/docs/validate.go`, `internal/docs/validate_test.go`
- `internal/server/docsqueue.go`, `internal/server/docsqueue_test.go`

## Notes

- 2026-09-12 — Implemented: `docs.StaleDirs` (missing/meta-lag/corrupt counts as
  stale, sorted, nil when disabled) and the endpoint union (queue-stale ∪
  hash-stale → one manual job; empty → `nothing to refresh`). Decisions:
  missing STRUCTURE.md counts as stale (a refresh writes the skeleton); a hash
  check error logs and falls back to queue-stale only rather than failing the
  request.
- Verification evidence: `go test ./internal/docs ./internal/server -count=1`
  — StaleDirs (disabled/fresh-after-seed/file-add bubbling to ancestors/
  deleted-file), endpoint union (fresh → nothing to refresh; file add → 202
  with exactly the stale union as one manual job), pre-existing queue-stale
  reconcile test updated for union semantics (fixture now seeds skeletons so
  only queue-stale remains). Live: POST on this repo enqueued exactly the six
  hash-stale dirs from the EXP work. Full suite green.