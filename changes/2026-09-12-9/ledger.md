# Ledger — 2026-09-12-9

- Change ID: 2026-09-12-9
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
| [REF-00](tasks/00-staledirs-endpoint-union.md) | StaleDirs helper and /docs/refresh union | Done | — | 2026-09-12 | Union enqueued exactly the 6 live hash-stale dirs; tests cover bubbling/disabled/fresh |
| [REF-01](tasks/01-bell-modal-ui.md) | Notification bell, modal, banner re-scope | Done | REF-00 | 2026-09-12 | Bell+modal live-verified; banner violations-only; docs SSE re-checks findings |
| [REF-02](tasks/02-dogfood-readme.md) | Dogfood and README touch-up | Done | REF-00, REF-01 | 2026-09-12 | 6 findings → 0 after one manual job; README updated |

## Decision log

- 2026-09-12 — Agreed with user: refresh button reconciles the union of queue-stale and hash-stale dirs as one manual gardener job; docs findings move from the banner into a header notification bell opening a modal that hosts the button; banner re-scoped to changes/ violations. Prefix REF registered in root ledger.
