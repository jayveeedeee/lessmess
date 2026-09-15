# Ledger — 2026-09-15-lk9or

- Change ID: 2026-09-15-lk9or
- Plan: [plan.md](plan.md)
- Branch: —
- Overall status: Done
- Last updated: 2026-09-15

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
| [IDX-00](tasks/00-sortable-index-markup.md) | Sortable index table markup and server-emitted sort keys | Done | — | 2026-09-15 | `go vet ./...` + `go test ./...` pass; new `TestIndexSortableMarkup` and `TestStatusRankTemplateFunc` assert the six sort buttons, row `data-*` keys, and workflow-order ranks; `TestIndexNewestFirst` still passes. Visual rebuild check folds into IDX-01's verification (same binary). |
| [IDX-01](tasks/01-index-sort-behavior.md) | Client-side sorting, indicators, and persistence | Done | IDX-00 | 2026-09-15 | Implemented and agent-verified: `node --check`, `go vet`, full `go test`, rebuilt binary serves all six sort buttons + row keys + fresh hashed app.js, comparator simulation passes all 12 column/direction combos on the real 30-row dataset. In user's hands for the browser pass (clicks, indicators, SSE-reload persistence, keyboard). |

## Decision log

- 2026-09-15: Client-side sorting chosen over a server-side `?sort=` API; choice persisted in `localStorage` (`tt-index-sort`) to survive the index page's SSE full-reload. Default remains the server's newest-first order.
- 2026-09-15: Status sorts in workflow order (`model.TaskStatusOrder`) via a new `statusRank` template func; rows carry `data-tasks`/`data-status-rank`/`data-updated` so JS never scrapes display text.
- 2026-09-15: Status semantics per user: `Test` means implemented and agent-verified, handed to the user for testing/acceptance — it does not claim the user's testing already finished. IDX-01 moved to `Test` on that basis with the browser pass as the user's acceptance step.
