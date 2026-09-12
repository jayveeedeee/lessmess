# Ledger — 2026-09-12-10

- Change ID: 2026-09-12-10
- Plan: [plan.md](plan.md)
- Branch: —
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
| [BSB-00](tasks/00-change-session-prompt.md) | Change-scoped prime prompt for change sessions | Done | — | 2026-09-12 | changePrompt + prime-on-create (fail deletes session); go vet + go test green |
| [BSB-01](tasks/01-scaffold-guard.md) | Scaffold guard for bound sessions | Done | — | 2026-09-12 | mapping.changeOf + 409 guard; bound/unassigned/unmapped paths tested; go vet + go test green |
| [BSB-02](tasks/02-dogfood.md) | Live dogfood of session binding | Done | BSB-00, BSB-01 | 2026-09-12 | Bound-session curl → 409 with redirect; new board session primed and answered scoped to 2026-09-12-10; validate clean; artifacts removed |

## Decision log

- 2026-09-12: Two-layer design (prime prompt + deterministic 409 guard) approved by user; binding is absolute (no escape hatch); only new sessions are primed.
