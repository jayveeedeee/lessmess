# Ledger — 2026-09-12-15

- Change ID: 2026-09-12-15
- Plan: [plan.md](plan.md)
- Branch: —
- Overall status: Done
- Last updated: 2026-09-13

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
| [TST-00](tasks/00-model-test-status.md) | Add Test status to model and ledger template | Done | — | 2026-09-13 | StatusTest added before Done in TaskStatusOrder; template ledger defines Test row; TestChangeLedgerMoveTaskToTest + template assertion pass; go test ./internal/model green |
| [TST-01](tasks/01-workflow-text-user-gated-done.md) | Workflow text — Test vocabulary and user-gated Done | Done | — | 2026-09-13 | Vocabulary, workflow steps 6–9, and handoff section updated; embedded copy re-synced byte-identical; go test ./internal/docs green (drift test) |
| [TST-02](tasks/02-test-column-styling.md) | Test pill styling in both themes | Done | TST-00 | 2026-09-13 | --st-test-* in both theme blocks + .status-test rule; smoke board rendered status-test pills |
| [TST-03](tasks/03-verify-tests-readme.md) | Update tests, README, and run full verification | Done | TST-00, TST-01, TST-02 | 2026-09-13 | go vet + go test green; validate OK; smoke on :9091 showed six columns in order, move endpoint persisted Test and reorder; README updated |

## Decision log

- 2026-09-12: Column order places `Test` immediately before `Done`; `Blocked` keeps its slot (user pick).
- 2026-09-12: `Done` is user-gated, not user-only — agents may set it on explicit user instruction; enforcement is workflow-text only, no server-side guard (user pick).
- 2026-09-12: Close-out readiness becomes "all non-cancelled tasks `Test` or `Done`" (user pick).
- 2026-09-12: This change's own ledger keeps the scaffolded five-row status-definitions table; only the template for future ledgers changes.
- 2026-09-13: `web/static/app.js` close-change confirm ("task(s) are not Done or Cancelled") intentionally left unchanged — with the new flow it correctly nudges the user to promote `Test` tasks before closing.
- 2026-09-13: Tasks on this change were moved to `Test` (not `Done`) per the new workflow — the built-in dogfood; the user promotes them after review.
