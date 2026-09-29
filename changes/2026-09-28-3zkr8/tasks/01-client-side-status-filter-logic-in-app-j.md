# CSF-01: Client-side status filter logic in app.js

## Why

The markup from CSF-00 needs behavior: hide non-matching cards, persist the
choice, and keep it applied across the SSE-driven list rebuild — following the
same client-side pattern the sort select already uses.

## What

In `web/static/app.js`, beside the existing index sort block (~lines 5248-5385):

- `indexFilterState()` — read/persist localStorage key `tt-index-filter-status`;
  corrupt values fail open to `""` (All), mirroring `indexSortState`'s contract.
- Apply logic: hide `a.change-card` elements whose `data-status` differs from
  the active filter; re-apply after `refreshChangeCards` rebuilds the list.
- Wire `#change-filter-status` in the same init path as `initIndexSort`.
- `buildChangeCard` emits `data-status` so rebuilt cards stay filterable.
- Recompute per-status counts and update option labels (e.g. "In progress
  (3)"; "All (12)") on initial render and every refresh.
- When the active filter matches zero cards, show a muted "no changes match"
  line in place of the list.

## Files affected

- `web/static/app.js`

## Verification

- With the rebuilt binary: select each status and confirm only matching cards
  remain; reload the page and confirm the selection persisted; corrupt the
  localStorage value and confirm it falls back to All; trigger an SSE refresh
  and confirm the filter still applies; select a status with no changes and
  confirm the muted empty line appears.
