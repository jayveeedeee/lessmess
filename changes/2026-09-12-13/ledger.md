# Ledger — 2026-09-12-13

- Change ID: 2026-09-12-13
- Plan: [plan.md](plan.md)
- Branch: main
- Overall status: Done
- Last updated: 2026-09-12

## Status definitions

| Status | Meaning |
| --- | --- |
| Not started | Work has not begun. |
| In progress | Implementation or verification is actively underway. |
| Blocked | Work cannot continue until a documented dependency, decision, approval, or external condition is resolved. |
| Done | All verification and completion criteria in the task file have passed. |
| Cancelled | The task was intentionally removed from scope and the reason is recorded. |

## Tasks

Row order is display and priority order; top row is highest priority.

| Task | Title | Status | Depends on | Updated | Notes |
| --- | --- | --- | --- | --- | --- |
| [IGN-00](tasks/00-fix-gitignore.md) | Anchor artifact pattern and restore main.go tracking | Done | — | 2026-09-12 | Verified: check-ignore correct both ways; main.go + docs files staged; validate OK. |

## Decision log

- 2026-09-12: Root cause recorded: bare `tasktracker` pattern ignored `cmd/tasktracker/` since init; `main.go` never committed in any of the three commits. Fix: anchor `/tasktracker`, add `.DS_Store`, re-add main.go.
- 2026-09-12: Task-ID prefix for this change registered as `IGN`.
