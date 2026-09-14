---
id: LRN-00
title: Ancestor targets and DocsJob shape
---

# LRN-00: Ancestor targets and DocsJob shape

Status: see [../ledger.md](../ledger.md).

## Objective

Close-out docs jobs carry the covered ancestors of every touched dir so
later tasks can prompt for and protect review-and-fix work there.

## Dependencies

None.

## Scope

- Lexical ancestor computation in `internal/server/touched.go`.
- `DocsJob.Ancestors []string` (JSON `ancestors,omitempty`) in
  `internal/server/docsqueue.go`; `enqueueDocsRefresh` populates it.
- Queue state roundtrip (`.lessmess/docs-queue.json`) with the new field.

## Implementation steps

1. Add a function alongside `touchedDocsDirs` (or extend its return)
   producing, for each primary dir, every covered dir on its path to the
   root: split on "/", walk `path.Dir` upward, gate each step with
   `cfg.Covered`, treat the walk reaching "." as the covered root.
   Dedup against the primary set and within itself; sort.
2. Extend `DocsJob` with `Ancestors []string` (`json:"ancestors,omitempty"`).
3. `enqueueDocsRefresh`: skip enqueue when both sets are empty (primary
   already gates); include ancestors otherwise.
4. Confirm job failure paths (`failStale`) treat ancestors as stale
   flags alongside primary dirs so the refresh union can reconcile them.

## Verification

- Table tests: nested dirs (`internal/server` → `internal`, `.`),
  sibling dedup (`a/x`, `a/y` → `a`, `.` once), uncovered intermediate
  dirs pruned, hidden paths never produce ancestors, deterministic
  order.
- Queue save/load roundtrip keeps `Ancestors` byte-stable; a legacy
  state file without the field loads with empty ancestors.

## Completion criteria

Ancestor computation and job shape land with tests green and
`go vet ./...` clean; no prompt or runner behavior changes yet.

## Files affected

- `internal/server/touched.go`
- `internal/server/docsqueue.go`
- `internal/server/touched_test.go` (new or extended docsqueue_test.go)
- `internal/server/docsqueue_test.go`

## Notes

- Placement stays footprint-driven; ancestors are derived, never parsed
  from learnings.
- Manual reconciliation jobs do not set ancestors (decision in plan).
