# Ledger — EXC-00

- Task: EXC-00 (change 2026-09-16-q7t4k)
- Last updated: 2026-09-16

## Status definitions

| Status | Meaning |
| --- | --- |
| Not started | Work has not begun. |
| In progress | Implementation or verification is actively underway. |
| Blocked | Work cannot continue until a documented dependency, decision, approval, or external condition is resolved. |
| Test | Implementation and verification are complete; awaiting user acceptance before Done. |
| Done | Accepted by the user; agents set this only on explicit user instruction. |
| Cancelled | The task was intentionally removed from scope and the reason is recorded. |

## Tasks

Row order is display and priority order; top row is highest priority.

| Task | Title | Status | Depends on | Updated | Notes |
| --- | --- | --- | --- | --- | --- |
| [EXC-00.00](tasks/00-stream-writer.md) | Streaming CSV writer | In progress | — | 2026-09-16 | — |
| [EXC-00.01](tasks/01-column-map.md) | Column mapping | Not started | EXC-00.00 | 2026-09-16 | — |
| [EXC-00.02](tasks/02-fixtures.md) | Golden-file fixtures | Blocked | EXC-00.01 | 2026-09-16 | Waiting on schema decision |

## Decision log

- 2026-09-16: decomposed EXC-00 into three subtasks.
