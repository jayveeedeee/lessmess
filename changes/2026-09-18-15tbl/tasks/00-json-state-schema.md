---
id: JSI-00
title: JSON state schema and model types
---

# JSI-00: JSON state schema and model types

Status: see [../ledger.md](../ledger.md).

## Objective

Define and implement the canonical JSON state model for the workflow: Go structs, load/save, and stable serialization for `.lessmess/workflow/index.json` and `.lessmess/workflow/changes/<id>.json`, with round-trip tests. No wiring into the store yet.

## Dependencies

— (first task; foundation for everything else)

## Scope

- New types in `internal/model` (or a sibling file/package agreed during implementation): `WorkflowIndex` and `ChangeState` with the task-tree node struct per the plan schema (id, seq, parent, title, file, status, dependsOn, updated, notes) plus change-level fields (overall status value + derived flag + updated, decision log).
- Status values reuse the exact current display strings from `internal/model` vocabularies.
- JSON marshal/unmarshal with stable field order (struct order), 2-space indent, trailing newline; save via `model.WriteFileAtomic`; load with strict decoding (`DisallowUnknownFields` where practical) returning typed errors.
- Helpers: task lookup by dotted ID, array-order-is-priority semantics, dotted-ID validation reuse from `taskid.go`, prefix registry lookups on the index.

Out of scope: migration, store wiring, rendering, endpoints.

## Implementation steps

1. Design the structs against the schema in `plan.md` (including `archived` handling on the index).
2. Implement load/save with atomic writes and strict decode.
3. Implement lookup/ordering/dotted-ID helpers.
4. Write round-trip tests (marshal → unmarshal → marshal equality) and error tests (unknown fields, bad status values, duplicate IDs, self/missing dependency references).

## Verification

`go test ./internal/model/...` green; round-trip fixtures pinned in `testdata/`.

## Completion criteria

Schema types compile with tests covering round-trip, validation errors, ordering, and dotted-ID behavior; no consumer changes yet.

## Files affected

- `internal/model/` (new state types + tests, possibly new `testdata/` fixtures)

## Notes

Decide inline whether the JSON types live in `internal/model` or a new package; keep the decision recorded here when made.
