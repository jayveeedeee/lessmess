---
id: KAN-04
title: HTTP API and SSE
---

# KAN-04: HTTP API and SSE

Status: see [../ledger.md](../ledger.md).

## Objective

Implement `internal/server`: all HTTP routes from the plan on top of the store, including the SSE stream for live updates.

## Dependencies

KAN-03.

## Scope

In scope: routes `GET /`, `GET /changes/{id}`, `GET /changes/{id}/tasks/{task}`, `POST /changes/{id}/tasks`, `POST /changes/{id}/move`, `POST /changes`, `GET /events` (SSE hub broadcasting store events), `GET /api/validate`; JSON request/response for POSTs; error mapping (404 unknown change/task, 409 freshness conflict, 422 validation refusal, 400 bad input); `log/slog` request and write logging.
Out of scope: HTML templates (KAN-05 — this task returns JSON or minimal placeholder HTML so it is testable independently).

## Implementation steps

1. `Server` type holding `*store.Store`; route table on stdlib mux with method+path patterns.
2. Handlers decoding/validating input, calling store ops, mapping errors to status codes.
3. SSE hub: subscribe on connect, rebroadcast store events, heartbeat, client cleanup.
4. `httptest` coverage: every route happy path + error paths; SSE event delivery after a store write.

## Verification

- `go test ./internal/server` passes for all routes and error mappings, including a move operation end-to-end against a tempdir store and an SSE event observed via test client.

## Completion criteria

- API behaves per route contract with correct status codes; SSE delivers store events.

## Files affected

- `internal/server/` (new)

## Notes

- HTML rendering lands in KAN-05; keep handlers content-negotiation-ready (`Accept: text/html` vs JSON).
