---
id: CID-02
title: Server newest-first by ledger position
---

# CID-02: Server newest-first by ledger position

Status: see [../ledger.md](../ledger.md).

## Objective

Keep the index page newest-first now that the ID suffix no longer encodes
allocation order: order by date descending, then root-ledger row position
descending, then ID ascending as a fallback.

## Dependencies

- CID-01 (mixed-format IDs exist end to end, so ordering must not rely on
  numeric suffixes).

## Scope

`internal/server/server.go` (index sort) and
`internal/server/server_test.go` (`TestIndexNewestFirst`). JSON and HTML
responses keep sharing one sort.

## Implementation steps

1. In `Server.index`, carry each root-ledger row's position (its index in
   the parsed ledger) alongside the row summary.
2. Replace `changeIDLess`/`splitChangeID` (numeric-counter comparator) with
   a comparator over (date from the ID prefix, descending; ledger position,
   descending; ID, ascending). Parse the date from the ID's first 10
   characters; rows whose IDs fail the union regex fall back to ID order.
3. Update `TestIndexNewestFirst`:
   - Same-date changes append their root rows in creation order; the newest
     (last row) must render first, regardless of suffix values — include
     random-looking suffixes and a legacy numeric ID together.
   - Drop the unpadded-counter assertion (`-10` vs `-2`); its premise is
     gone.
   - Cross-date ordering (date descending) stays asserted.
4. Confirm `Store.Changes`/`Archived` (plain ID sort) are acceptable for
   their surfaces; no chronology promises there — note it in the code
   comment on `sortedChanges` if wording helps.

## Verification

- `go test ./internal/server/...` passes with the reworked ordering test.
- Manual check: build and load the index page of this repo (which has
  numeric-ID changes and, after CID-01's smoke test, a random-ID change);
  order is newest-first with the newest same-date change on top.

## Completion criteria

- No numeric counter parsing remains in `internal/server`.
- `TestIndexNewestFirst` covers mixed legacy/new suffixes and append order.

## Files affected

- `internal/server/server.go`
- `internal/server/server_test.go`

## Notes

- For legacy repositories this reproduces today's order exactly, because
  counters were allocated in root-ledger append order.
- The store's SSE `fs`/`write` behavior on scaffold is untouched.
