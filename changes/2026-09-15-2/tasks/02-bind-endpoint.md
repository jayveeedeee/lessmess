---
id: PSB-02
title: Bind endpoint for task sessions
---

# PSB-02: Bind endpoint for task sessions

## Objective

Give the user (or a bound session) a corrective way to record that a subagent
session belongs to task `TSK-NN` of a change. After PSB-00, this is not the agent's
primary path — the parent model cannot see the child session ID — so the endpoint
must work without a caller session (UI action) and fill `Parent` from the live
sub's `parentID`.

## Dependencies

- PSB-01 (schema fields).

## Scope

- New handler + route in `internal/server` (`changesession.go`, `server.go`), unit
  tests. No UI.

## Implementation steps

1. `POST /api/changes/{id}/task-sessions`, JSON body
   `{task, sub, session?}` — `task` is a task ID from the change's ledger, `sub`
   the subagent session ID, `session` an optional caller bound to `id`.
2. Validation order, mirroring scaffold conventions:
   - change exists (else `writeErr` 404 path),
   - mapping readable (503 on `mapErr`),
   - `task` exists among the change's ledger task IDs else 422,
   - live `GetSession(sub)` succeeds else 422; capture its `ParentID` and title,
   - when `session` is supplied: `changeOf(session) == id` else 409 naming the
     conflicting change,
   - `sub` already mapped to the same change+task → 200 `{reused: true}`;
     mapped to a different task or change → 409,
   - otherwise append `SessionEntry{Session: sub, Title: live title, Created: now,
     Task: task, Parent: live parentID}` and 201.
3. Register the route in `server.go` beside the other `/api/changes/{id}/...`
   routes; `slog.Info("task session bound", ...)` on success.
4. Unit tests with the fake opencode client: happy path (no caller); 409 wrong-change
   caller when `session` supplied; 422 unknown task; 422 dead sub; idempotent reuse;
   409 rebinding to another task; 503 unreadable mapping; `Parent` populated from
   the live session.

## Verification

- `go test ./internal/server/` green; `curl` against a dev server reproduces the
  guard matrix.

## Completion criteria

- Endpoint implements the full guard matrix with tests; route visible in the mux.

## Files affected

- `internal/server/changesession.go`, `internal/server/server.go`,
  `internal/server/changesession_test.go`.

## Notes

- The task-ID allowlist reuses the store's ledger task listing; keep the lookup in
  the store layer if a helper already exists, otherwise parse the change ledger the
  same way the board does.
