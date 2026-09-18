# Ledger — 2026-09-16-j2g58

- Change ID: 2026-09-16-j2g58
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
| [STS-00](tasks/00-status-endpoint.md) | Status endpoint and handler tests | Done | — | 2026-09-16 | `TestStatusEndpoint` covers all three allowed transitions with both-file agreement, 409 Done→/close, 422 Cancelled/unknown/missing, 400 malformed, 404 unknown change. `go vet ./...` + full `go test ./...` green. |
| [STS-01](tasks/01-board-status-control.md) | Board status control | Done | STS-00 | 2026-09-16 | Status select on the board head (rendered only for Planned/In progress/Blocked, current preselected) posting to the endpoint; failure repaints from server state. `TestBoardLifecycleButtons` extended (select present + preselected for In progress, absent for Done); suite green. Browser interaction = user visual pass. |
| [STS-03](tasks/03-auto-derive.md) | Replace manual status control with automatic derivation | Done | — | 2026-09-17 | User-directed reversal of STS-00/01. Select+endpoint+CSS removed; `internal/store/overall.go` derives Planned/Blocked/In-progress from the task tree and syncs both ledgers after moves/creates and on watcher rescans (Done preserved while complete, implicit reopen on new work); derivation + convergence tests, render tests updated, AGENTS.md rule 3 reworded + embed regen (drift green), README synced; full suite green |
| [STS-02](tasks/02-docs-workflow-sync.md) | Workflow wording, embedded asset, and README | Done | STS-00 | 2026-09-16 | AGENTS.md rule 3 now mandates the deterministic path (endpoint/board; Done stays user-gated; Cancelled hand-edited with reason). Embedded asset regenerated via the awk recipe — docs tests green (drift test). README board section documents the status select + endpoint. |

## Dependencies

- STS-01 and STS-02 depend on STS-00; STS-01 and STS-02 are independent of each other.

## Decision log

- 2026-09-17: user direction — remove the manual status switcher entirely; the overall status is derived automatically from the task statuses ("as it was always before"). STS-00/01 (endpoint + select) reverted; the atomic two-ledger write survives inside the derivation sync (`store.syncOverall`). Derivation semantics: Planned while nothing started, Blocked when all open work is blocked, otherwise In progress; Done user-gated and never downgraded while the tree is complete.

- 2026-09-16: Reuse `store.SetChangeStatus` unchanged; the handler adds only input validation and 4xx semantics. Root cause addressed: no deterministic path existed for start/block transitions, so agents hand-edited the change ledger and missed the root row (rule-6 drift on 2026-09-16-7ueiv).
- 2026-09-16: `Done` is rejected (409 → `/changes/{id}/close`) to keep closing user-gated; `Cancelled` is out of scope (no cancellation flow exists today).
