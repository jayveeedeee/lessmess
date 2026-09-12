---
id: NAV-02
title: Newest change at top of index
---

# NAV-02: Newest change at top of index

Status: see [../ledger.md](../ledger.md).

## Objective

Show the newest change at the top of the index change list instead of the
current oldest-first ledger order.

## Dependencies

— (none)

## Scope

- `internal/server/server.go` `index` handler ordering.
- `internal/server/server_test.go` ordering coverage.
- Applies to the HTML page and the JSON response alike.

## Implementation steps

1. In `Server.index`, after building `rows` from the root ledger, sort by
   change ID descending (`sort.Slice` with `rows[i].ID > rows[j].ID`). IDs are
   `YYYY-MM-DD-N`, so lexicographic descending is chronological newest-first.
2. Keep task counts and all other fields untouched.
3. Add a test: a fixture store whose root ledger lists two or three changes in
   append order; assert `GET /` (JSON) returns them newest-first.

## Verification

1. `go vet ./... && go test ./...` pass, including the new test.
2. Run the server against this repository: `/` shows `2026-09-12-12` (or the
   highest ID) at the top.

## Completion criteria

- HTML table and JSON array are both newest-first; existing tests still pass;
  no change to the on-disk ledger (read-only ordering).

## Files affected

- `internal/server/server.go`
- `internal/server/server_test.go`

## Notes

- Sorting by ID descending was chosen over reversing ledger row order: the
  root ledger is append-mostly, but ID sort stays correct even if rows are
  ever edited out of order.
