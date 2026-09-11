---
id: LIF-01
title: Store status transitions + close/reopen endpoints
---

# LIF-01: Store status transitions + close/reopen endpoints

Status: see [../ledger.md](../ledger.md).

## Objective

Implement `SetChangeStatus` in the store (per-change ledger + root ledger row, atomic writes, reload, notify) and the `POST /changes/{id}/close` / `POST /changes/{id}/reopen` endpoints.

## Dependencies

LIF-00 (semantics settled).

## Scope

In scope: store method + two endpoints + tests.
Out of scope: UI buttons (LIF-03), commit flow (LIF-02).

## Implementation steps

1. `Store.SetChangeStatus(id, status)`: fresh parse change ledger → `SetOverall(status, today)` → atomic write; fresh parse root ledger → `Update(id, status, today)` → atomic write; reload + notify.
2. Endpoints mapping close → `Done`, reopen → `In progress`.
3. Tests: close writes both files correctly and revalidates clean; reopen round trip; unknown change 404.

## Verification

- Store/endpoint tests pass; validate OK on fixture.

## Completion criteria

- Both transitions persist to both ledgers via endpoints.

## Files affected

- `internal/store/store.go`, `internal/server/server.go` (+ tests)

## Notes

- None yet.
