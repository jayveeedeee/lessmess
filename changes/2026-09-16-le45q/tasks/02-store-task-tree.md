---
id: NTD-02
title: Store recursive task tree and operations
---

# NTD-02: Store recursive task tree and operations

Status: see [../ledger.md](../ledger.md).

## Objective

Build the recursive in-memory task tree and the write operations: decompose a task, create a subtask inside a container, move a task within its governing ledger, and compute subtree statistics.

## Dependencies

NTD-01

## Scope

- `internal/store`: scan/tree, `DecomposeTask`, `CreateTask` with parent, `MoveTask` via governing ledger, `TaskByID`, `SubtreeStats`. Validation rules are NTD-03; sessions are NTD-05.

## Implementation steps

1. Extend `scan` to walk `tasks/` recursively: each node = parsed task file, href, parent node, children, governing `*model.TaskLedger` (or the change ledger at top level). Directories must match a sibling task file; mismatches are recorded for NTD-03 to report (scan tolerates, validate flags).
2. Add `TaskByID(changeID, taskID)` resolving across the whole tree, and `SubtreeStats(node)` returning per-status counts over all descendants (cancelled excluded from the denominator, test+done = complete) for rollups.
3. `DecomposeTask(changeID, taskID)`: refuse when the container already exists (sentinel error); otherwise create `NN-slug/ledger.md` via `RenderTaskLedger` and `NN-slug/tasks/`, atomically, then reload + notify.
4. `CreateTask(changeID, parentTaskID, title)`: allocate the next sequence among the parent's existing children (no reuse), mint the dotted ID, write the task file inside the container's `tasks/`, append the row to the governing ledger. Empty parent = change root (today's behavior).
5. `MoveTask`: resolve the governing ledger from the task's parent instead of assuming the change ledger; keep status-group insertion semantics.
6. Verify `watch.go` picks up newly created nested directories (watch-set rebuild); extend if it only watches pre-existing dirs.
7. Keep all writes re-read/atomic per package discipline.

## Verification

- `go test ./internal/store/` green with nested fixture trees: scan depth, decompose idempotence refusal, child numbering (`00`, `01`, … per container), move within governing ledger, `SubtreeStats` across grandchildren, watcher firing on new nested dir.
- Existing store tests unchanged and green.

## Completion criteria

Tree scan, decompose, parented create, governing-ledger move, and subtree stats all work on disk with tests; no validation enforcement yet (next task).

## Files affected

- `internal/store/store.go`
- `internal/store/watch.go`
- `internal/store/store_test.go`, `internal/store/watch.go` tests

## Notes

Close-out readiness helper may land here or in NTD-03 with the validation walk — implement with NTD-03 where the tree traversal naturally lives.
