---
id: LRN-04
title: Refresh union and lint context in manual jobs
---

# LRN-04: Refresh union and lint context in manual jobs

Status: see [../ledger.md](../ledger.md).

## Objective

Lint-flagged dirs flow into `POST /docs/refresh`'s reconciliation job,
and the manual gardener prompt names the flagged references so the fix
is targeted.

## Dependencies

- LRN-03 (lint exists and returns dir → refs).

## Scope

- `docsRefresh` union in `internal/server/docsqueue.go`.
- `DocsJob.LintRefs map[string][]string` (`json:"lintRefs,omitempty"`).
- Manual branch of `gardenerPrompt` renders flagged refs.

## Implementation steps

1. `docsRefresh`: union queue-stale, hash-stale, and lint-flagged dirs.
   A lint error logs and continues with the other sets (mirrors the
   hash-check error path). Empty union still returns 200
   `nothing to refresh`.
2. When the union came from a manual enqueue, attach
   `LintRefs` (dir → refs, only dirs present in the job) to the job.
3. `gardenerPrompt` manual branch: after the standard section, list
   `dir — flagged references: a, b` per dir with refs, instructing the
   model to fix or delete the learnings citing them under the same
   hard rules; jobs without refs render exactly as today.
4. Close-out (change) jobs never carry `LintRefs`.

## Verification

- Server tests with a fake lint source: refresh enqueues one manual job
  whose dirs and `LintRefs` match the union; lint error degrades to the
  previous union; all-empty still 200 `nothing to refresh`; prompt
  contains the flagged refs for manual jobs and no ref text for
  change jobs; state-file roundtrip preserves `LintRefs`.

## Completion criteria

A flagged dir is reconcilable end-to-end from the bell button with the
model told exactly what to look for; all suites green.

## Files affected

- `internal/server/docsqueue.go`
- `internal/server/docssession.go`
- `internal/server/docsqueue_test.go`
- `internal/server/docssession_test.go`

## Notes

- This is the composition that covers Part B's blind spot: parent dirs
  no change touched get visited via the lint-routed manual job.
