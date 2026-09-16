---
id: STS-00
title: Status endpoint and handler tests
---

# STS-00: Status endpoint and handler tests

Status: see [../ledger.md](../ledger.md).

## Objective

Add `POST /changes/{id}/status` so every allowed overall-status transition goes
through `store.SetChangeStatus` — updating the change ledger and root-ledger row
atomically — instead of agent hand-edits.

## Dependencies

None. First task of the change.

## Scope

- One route + handler in `internal/server`, reusing `store.SetChangeStatus`.
- JSON body `{"status":"Planned"|"In progress"|"Blocked"}`.
- Reject `Done` with 409 (point at `/changes/{id}/close`); reject `Cancelled` and
  any unknown/missing value with 400 or 422; unknown change → 404.
- Handler tests only; no store changes.

## Implementation steps

1. In `internal/server/lifecycle.go` (or a sibling `status.go`), add a
   `changeStatus` handler: parse the change ID, decode the JSON body, map the
   string to `model.OverallStatus`, and enforce the accepted set
   (`OverallPlanned`, `OverallInProgress`, `OverallBlocked`).
2. Return 409 for `OverallDone` with a message naming `/changes/{id}/close`;
   400/422 for anything else invalid; 404 via `store.Change` miss.
3. Call `s.st.SetChangeStatus(id, status)` on success and respond 200 with the
   resulting status JSON (shape consistent with close/reopen responses).
4. Register the route in `server.go` next to close/reopen:
   `mux.HandleFunc("POST /changes/{id}/status", s.changeStatus)`.
5. Add handler tests modeled on `lifecycle_test.go`: success per accepted status,
   both-files agreement after each call (parse root + change ledger), 409 Done,
   invalid value, missing body, unknown change.

## Verification

- `go vet ./... && go test ./...` green from the repo root.
- New tests pass, including the both-ledger agreement assertions.
- Manual: `go run ./cmd/lessmess serve` against a scratch repo (or the live dev
  server), POST each status, confirm `changes/ledger.md` and the change ledger
  update together and `lessmess validate` stays clean.

## Completion criteria

- Endpoint live on the mux with the accepted-value contract above.
- Tests demonstrate rule-6 agreement holds across a sequence of calls.
- No store-layer code changed.

## Files affected

- `internal/server/lifecycle.go` (or new `status.go`)
- `internal/server/server.go` (route)
- `internal/server/lifecycle_test.go` (or new `status_test.go`)

## Notes

- `SetChangeStatus` already validates the overall vocabulary
  (`internal/store/store.go:589`); the handler's pre-validation exists to give
  clean 4xx semantics and to keep `Done` unreachable through this path.
- Same-status calls are allowed and simply rewrite both files with today's date.
