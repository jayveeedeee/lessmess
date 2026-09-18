---
id: STS-03
title: Replace manual status control with automatic derivation
---

# STS-03: Replace manual status control with automatic derivation

Status: see [../ledger.md](../ledger.md).

## Objective

Remove the manual change-status UI and endpoint (STS-00/01) and derive the overall status automatically from the task tree, as the user directs: the pill should always reflect the board, with no hand-switching.

## Dependencies

—

## Scope

- Remove the board status `<select>` and its app.js handler.
- Remove `POST /changes/{id}/status` and its handler/tests.
- Store derivation: Planned while nothing started, Blocked when all open work is blocked, otherwise In progress; `Done` stays user-gated (close/reopen) and is never downgraded while the tree is complete.
- Converge external (agent) ledger edits via the watcher.
- AGENTS.md rule 3 rewording + embedded asset regen; README sync.

## Implementation steps

1. Delete the select from `board.html`, the handler from `app.js`, the route and `changeStatus` handler, and the endpoint tests.
2. Add `internal/store/overall.go`: `DeriveOverall` (from the task tree), `syncOverall` (writes both ledgers via `SetChangeStatus` only on drift, preserving `Done` while close-out-ready), `SyncOverallStatuses` (all active changes).
3. Call the sync after task-mutating writes (`MoveTask`, `CreateTask`) and on watcher reload.
4. Reword AGENTS.md rule 3; regenerate the embedded asset; keep the drift test green.
5. Update README (status select → automatic pill).

## Verification

- Store tests: derivation table (Planned/In progress/Blocked/ready-stays-In progress), Done preservation, auto-reopen on new work, both-ledger agreement, watcher convergence.
- `go vet ./... && go test ./...` green; drift test green; live board check.

## Completion criteria

No manual status control anywhere; the pill always equals the derived status; external edits converge; Done still only via Close.

## Files affected

- `web/templates/board.html`, `web/static/app.js`
- `internal/server/server.go`, `internal/server/lifecycle.go`, `internal/server/lifecycle_test.go`
- `internal/store/overall.go` (new), `internal/store/store.go`, `internal/store/watch.go`
- `AGENTS.md`, `internal/docs/assets/workflow_agents.md`, `README.md`

## Notes

- 2026-09-17: user-directed reversal of STS-00/01 ("remove the manual status switcher, derive automatically from the tickets as before"). The endpoint's atomic two-ledger write survives inside the derivation sync.
