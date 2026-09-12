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
   change ID descending: date prefix lexicographic, unpadded counter numeric
   (`changeIDLess` helper; `2026-09-12-9` sorts after `2026-09-12-12` as a
   string, so plain string compare is wrong).
2. Keep task counts and all other fields untouched.
3. Add a test: a fixture store whose root ledger lists changes in append
   order, including same-date counters 0, 2, 10; assert `GET /` (JSON)
   returns them newest-first.

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
- Found during live verification: the counter is unpadded, so a plain
  lexicographic descending sort put `2026-09-12-9` above `2026-09-12-13`.
  Fixed by comparing the date prefix lexicographically and the counter
  numerically (`changeIDLess`); test covers same-date 0/2/10 counters.
- Verified 2026-09-12: `TestIndexNewestFirst` passes; live JSON on the repo
  returns `2026-09-12-13, -12, -11, -10, …`; index screenshot shows the same
  order. HTML and JSON share the one sort.
