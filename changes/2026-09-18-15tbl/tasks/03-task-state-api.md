---
id: JSI-03
title: Deterministic task-state API
---

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

Decide how "user vs agent caller" is distinguished (board-origin vs session-origin) and record it here; keep enforcement in one place per rule.
