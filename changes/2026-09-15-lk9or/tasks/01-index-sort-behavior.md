
# IDX-01: Client-side sorting, indicators, and persistence

Status: see [../ledger.md](../ledger.md).

## Objective

Wire the sortable headers built in IDX-00: clicking sorts the table in place with visible direction feedback, and the choice survives SSE-triggered reloads via `localStorage`.

## Dependencies

- IDX-00 (markup and `data-*` sort keys must exist).

## Scope

- `web/static/app.js`: `initIndexSort` — restore stored sort, click handling, row sorting, direction toggle, indicator and `aria-sort` updates, persistence.
- `web/static/app.css`: sort button/indicator styling (idle, asc, desc states).
- `README.md`: brief mention of the sortable index, if the feature warrants a line.

## Implementation steps

1. Add `initIndexSort` (invoked from the existing index-page init path alongside `loadDiscussions`/SSE wiring):
   - Storage: read `tt-index-sort` (`{col, dir}` JSON); ignore corrupt or unknown values and fall back to no override (server's newest-first order).
   - Click: determine column from `data-sort-col`; first click direction is ascending for Change/Title/Prefix/Status and descending for Updated/Tasks; clicking the active column again flips direction; persist `{col, dir}`.
   - Sort: reorder `<tbody>` rows by key — numeric for `data-tasks`/`data-status-rank`, string compare for `data-updated` and the text cells of Change/Title/Prefix — stable enough that equal keys keep DOM order.
   - Feedback: set `aria-sort` on the active `<th>` (`ascending`/`descending`, remove elsewhere) and toggle indicator classes (e.g. `sort-asc`/`sort-desc`) for CSS.
2. Style `.sort-btn` and the indicator span in `app.css` for idle/asc/desc, keeping the existing table look (follow current pill/btn-ghost conventions).
3. Optionally add one README line documenting the sortable columns.
4. Verify per the change plan's manual checklist.

## Verification

- Rebuild the binary and restart; on the index page:
  - Sort every column in both directions; verify Status follows workflow order (Not started → … → Cancelled), Tasks is numeric, Updated by date.
  - Set a non-default sort, touch a file under `changes/` to fire SSE, confirm the page reloads with the sort still applied.
  - Clear `localStorage` (or use a fresh profile): default order is newest-first with no indicators active.
  - Keyboard: header buttons activate via Enter/Space and are focus-reachable.
- `go vet ./... && go test ./...` still pass (no Go behavior change expected).

## Completion criteria

- All six columns sort in place per the plan's interaction rules, indicators and `aria-sort` are correct, persistence works across reloads, and the default order is unchanged.

## Files affected

- `web/static/app.js`
- `web/static/app.css`
- `README.md` (optional)

## Notes

- The index refresh is a full `location.reload()` (`scheduleRefresh`), so persistence must be read on every page load, not kept in memory.
- Follow the `tt-` client-state key convention (cf. `tt-last-session:<id>`).
- 2026-09-15: Implementation complete. Machine checks pass: `node --check web/static/app.js`, `go vet ./...`, `go test ./...`, `lessmess validate` (after syncing the root-ledger status to In progress), and a rebuilt binary on port 19231 served all six `data-sort-col` buttons, row `data-tasks`/`data-status-rank`/`data-updated` keys (ranks match workflow order across the 30 real changes), and the fresh content-hashed app.js.
- 2026-09-15: Comparator logic verified against the real served dataset (30 rows) by replicating `indexCellKey`/`applyIndexSort` exactly in a throwaway node script (repo `/tmp` untouched): all 12 column/direction combos monotonic, status asc yields rank sequence 1,1,1,…,4,…,5 (workflow order); data non-vacuous (11 distinct tasks, 5 distinct updated, 3 distinct statuses). Script kept out of the repo (no JS test infra by design).
- 2026-09-15: Remaining verification is strictly browser-bound (event wiring, indicator/`aria-sort` visuals, localStorage persistence across an SSE reload, keyboard) — no browser automation is available in the planning session, so those items await the user's restart-and-click pass or explicit acceptance on automated evidence.
