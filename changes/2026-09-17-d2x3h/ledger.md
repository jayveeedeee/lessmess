# Ledger — 2026-09-17-d2x3h

- Change ID: 2026-09-17-d2x3h
- Plan: [plan.md](plan.md)
- Branch: —
- Overall status: Done
- Last updated: 2026-09-17

## Status definitions

| Status | Meaning |
| --- | --- |
| Not started | Work has not begun. |
| In progress | Implementation or verification is actively underway. |
| Blocked | Work cannot continue until a documented dependency, decision, approval, or external condition is resolved. |
| Test | Implementation and verification are complete; awaiting user acceptance before Done. |
| Done | All verification and completion criteria in the task file have passed. |
| Cancelled | The task was intentionally removed from scope and the reason is recorded. |

## Tasks

Row order is display and priority order; top row is highest priority.

| Task | Title | Status | Depends on | Updated | Notes |
| --- | --- | --- | --- | --- | --- |
| [FORM-00](tasks/00-remove-add-task-form.md) | Remove board add-task form from UI | Done | — | 2026-09-17 | go vet/test green, validate OK, rebuilt binary: no form at root or drill-down, header/cards/crumbs intact. |

## Decision log

- 2026-09-17: UI-only removal per user choice — `POST /changes/{id}/tasks` stays as API surface (mirrors 2026-09-12-2 keeping `POST /changes`).
