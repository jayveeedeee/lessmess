---
id: NTD-01
title: Model container ledgers and dotted IDs
---

# NTD-01: Model container ledgers and dotted IDs

Status: see [../ledger.md](../ledger.md).

## Objective

Parse, mutate, and render container `ledger.md` files with the same strictness as change ledgers, and provide dotted task-ID helpers.

## Dependencies

NTD-00

## Scope

- `internal/model` only: new task-ledger type + parser/renderer, shared row-mutation core, dotted-ID helpers, template. No store/server changes.

## Implementation steps

1. Add `ParseTaskLedger(name, data)`: require `- Task: <id> (change <change-id>)` and `- Last updated:` headers; reuse `TaskColumns` via `FindTable`; strict statuses, link cells, `|` rejection — same error discipline as `ParseChangeLedger`.
2. Extract the shared task-table mutation core (`AppendTask`, `MoveTask`/`insertionPos`, `syncTable`, `Row`) so `ChangeLedger` and `TaskLedger` both use it without duplicating logic; keep `ChangeLedger`'s public surface byte-stable.
3. Add `RenderTaskLedger(taskID, changeID, date)` to `templates.go` matching the spec's minimal header set and status-definitions table.
4. Dotted-ID helpers: `ParentTaskID` (drops last segment), `ChildTaskID(parent, seq)`, `LastTaskSegment`, `TaskIDDepth`, plus validation that segments are two-digit zero-padded numbers.
5. New testdata fixtures for valid and malformed container ledgers; do not regenerate existing fixtures.

## Verification

- `go test ./internal/model/` green: round-trip parse/render/mutate of container ledgers; rejection cases (bad headers, unknown status, non-link Task cell, literal `|`); helper edge cases (top-level parent = empty, depth counting).
- Existing model tests unchanged and green.

## Completion criteria

Container ledgers parse/render/mutate with change-ledger strictness through one shared code path; dotted-ID helpers covered by tests.

## Files affected

- `internal/model/ledger.go`
- `internal/model/serialize.go`
- `internal/model/templates.go`
- `internal/model/taskid.go` (new)
- `internal/model/ledger_test.go`, `internal/model/serialize_test.go`, `internal/model/templates.go` tests
- `internal/model/testdata/` (new fixtures)

## Notes

`TaskFile` frontmatter stays exactly `id`+`title` — hierarchy is positional, no schema change.
