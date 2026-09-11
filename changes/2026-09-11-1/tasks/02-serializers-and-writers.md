---
id: KAN-02
title: Serializers and atomic writers
---

# KAN-02: Serializers and atomic writers

Status: see [../ledger.md](../ledger.md).

## Objective

Implement the write side of `internal/model`: render updated ledger tables back into their files preserving all other content byte-for-byte, create new task/change files from templates, and write files atomically.

## Dependencies

KAN-01.

## Scope

In scope: task-table re-rendering (status cell update, row reorder) within the preserved raw ledger; root-ledger row append/update; task-file creation from the `AGENTS.md` skeleton (frontmatter + fixed headings); change-directory scaffold generation (plan.md, ledger.md, tasks/); `WriteFileAtomic` helper (temp file in same dir + fsync + rename).
Out of scope: change-number allocation logic (store, KAN-03), HTTP layer.

## Implementation steps

1. `RenderTaskTable(rows)` emitting the pinned schema; splice into raw ledger content at recorded offsets.
2. `UpdateTaskRow` / `MoveTaskRow(toStatus, toIndex)` operations producing new table content.
3. `AppendRootRow`, `UpdateRootRow` (status/date cells).
4. `NewTaskFile(id, title)` and `NewChangeScaffold(id, title, prefix, date)` templates matching `AGENTS.md` formats exactly.
5. `WriteFileAtomic(path, data)`.
6. Round-trip tests: parse → mutate → serialize → reparse → compare; byte-identity test for untouched regions; template output validated by the KAN-01 parsers.

## Verification

- `go test ./internal/model` passes, including round-trip equality and byte-preservation of non-table content.
- Generated task/change files parse cleanly with the strict parsers.

## Completion criteria

- All serializer operations round-trip without data loss; atomic write helper verified (no partial file on crash path is simulated by error-injection test).

## Files affected

- `internal/model/` (serializer, templates, atomic write)

## Notes

- Templates are the single source of file generation; keep them byte-aligned with the `AGENTS.md` skeleton.
