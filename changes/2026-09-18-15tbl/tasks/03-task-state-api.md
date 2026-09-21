
# JSI-03: Deterministic task-state API

Status: see [../ledger.md](../ledger.md).

## Objective

Provide the HTTP endpoints that make every task-level state mutation deterministic and rule-enforced, so agents and the board never edit state files: task status transitions, task metadata updates, priority reordering, and decision-log appends.

## Dependencies

JSI-02

## Scope

- `POST /changes/{id}/tasks/{task}/status` — status transition with enforcement: `Done` is user-gated (agent-representing callers rejected with a clear error), `Test` requires recorded verification evidence note, invalid vocabulary → 422, unknown IDs → 404.
- `POST /changes/{id}/tasks/{task}/update` — title, notes, dependencies (existence-checked), with dependency-cycle rejection.
- `POST /changes/{id}/tasks/reorder` — priority order within a governing level.
- `POST /changes/{id}/decisions` — append decision-log entries.
- Adapt the existing change-level status endpoint and board interactions to the JSON store; the board's task drag/select flows call the new endpoints.
- All endpoints write through `store` (atomic, conflict-safe), return the refreshed task/change state, and emit SSE/update events consistent with current behavior.
- Update route table and JSON/HTML branching per server conventions.

## Implementation steps

1. Design request/response payloads; extend the route table.
2. Implement each endpoint delegating to new store methods; centralize transition-rule checks (single enforcement point per rule).
3. Wire board UI forms/actions to the endpoints (minimal: existing interactions keep working; new capabilities exposed where the board already has affordances).
4. Endpoint tests: happy paths, rule violations (agent `Done`, bad deps, unknown task, reorder across levels), and event emission.

## Verification

`go vet ./... && go test ./...` green; manual pass on the migrated repo: move a task through statuses from the board, verify `Done` rejection for agent-representing calls, verify SSE board refresh.

## Completion criteria

Every task-level mutation is available over the API with server-side rule enforcement and test coverage; no workflow-state mutation requires file edits.

## Files affected

- `internal/server/` (new handlers, route table, lifecycle adaptations, tests), `web/templates/` + `web/static/` (board wiring), `internal/store/` (new mutation methods)

## Notes

- Landed 2026-09-18 in `internal/server/taskstate.go` (+ `taskstate_test.go`) and `internal/store/mutations.go` additions (`SetTaskStatus`, `UpdateTask`+`TaskUpdate`, `ReorderTasks`, `AppendDecision`).
- Caller identity: the board's mutation fetches carry `X-Lessmess-UI: 1` (`app.js`: drag `/move`, `postLifecycle` close/reopen); `uiClient(r)` gates Done on both `/tasks/{task}/status` and `/move` — 403 with guidance to stop at Test. Best-effort by design (an agent could fake the header), but the sanctioned agent path never does.
- Test transitions require verification evidence (request `evidence`, appended to notes, or existing non-empty notes) — the AGENTS "record concise verification evidence" rule is now a 422.
- Dependency existence/cycles enforced by `ChangeState.Validate` on every write; reorder demands an exact permutation of the level.
- Store-level Done/None gating intentionally absent: identity is a transport concern; the store enforces content rules only.
- Verification: `go vet ./... && go test ./...` green; endpoint tests cover happy paths, Done gate (agent 403 / UI 200 on both endpoints), evidence gate, unknown deps/title/task, reorder permutations, decision log round-trip into the generated ledger view.
