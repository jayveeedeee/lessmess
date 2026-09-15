# 2026-09-15-lk9or: Sortable change index columns

- Change ID: 2026-09-15-lk9or
- Created: 2026-09-15
- Branch: —
- Status: see [ledger.md](ledger.md)

## Objective and context

The change index table on `/` lists all changes with a fixed server-side order (newest first). With many changes, users want to reorder the list in place — by title, status, task count, or update date — without leaving the page. This change makes all six data columns of the index table sortable by clicking their headers.

## Current behavior

- `web/templates/index.html` renders `.Data.Changes` into a plain `<table class="change-table">` with headers `Change | Title | Prefix | Status | Tasks | Updated | (Plan button)`. Headers are static text.
- `Server.index` (`internal/server/server.go`) sorts rows newest-first: date prefix desc → root-ledger row position desc → ID asc. This order is pinned by `TestIndexNewestFirst` and is the documented default.
- The index page live-refreshes via SSE: `scheduleRefresh()` in `web/static/app.js` performs a full `location.reload()` on `changes/` writes when no terminal is open. Any purely client-side UI state is lost on such a reload unless persisted.
- `changeSummary` fields: `id`, `title`, `prefix`, `status` (string), `updated` (YYYY-MM-DD string or `—`), `tasks` (int). Custom template funcs (`statusClass`, `base`, `markdown`) are registered in `render.go`.

## Target behavior

- Each of the six data columns has a clickable header control that sorts the table rows client-side, in place, with a visible direction indicator.
- Clicking a header sorts by that column; clicking it again flips the direction. Default first-click direction: ascending for Change, Title, Prefix, Status; descending for Updated and Tasks.
- Status sorts in workflow order (Not started → In progress → Blocked → Test → Done → Cancelled, per `model.TaskStatusOrder`), not alphabetically.
- The chosen sort persists in `localStorage` under a `tt-` key and is re-applied after SSE-triggered reloads. Absent or corrupt stored state falls back to the server's newest-first order.
- The default page load (no stored sort) is unchanged: newest-first.

## Scope

- `web/templates/index.html`: header cells become sort buttons with direction indicators; rows carry `data-*` sort keys.
- `internal/server/render.go`: new `statusRank` template func mirroring `statusClass` conventions.
- `internal/server/render_test.go`: assertions for the new markup/attributes.
- `web/static/app.js`: `initIndexSort` — click handling, row sorting via data keys, direction toggle, indicator/`aria-sort` updates, localStorage persistence and re-application.
- `web/static/app.css`: styling for the sort buttons and direction indicators.
- `README.md`: one-line mention if user-visible behavior warrants it.

## Non-goals

- No server-side sort API: no `?sort=`/`dir=` query params, no changes to `Server.index` ordering, no JSON API behavior change.
- No sortable Discussions list, board columns, or settings tables.
- No sorting on the trailing Plan-button column.
- No change to the archived-row filtering or to `TestIndexNewestFirst`'s contract.

## Design decisions

- **Client-side sorting over server-side:** no extra round-trips; the SSE reload pattern (full `location.reload()`) is handled by persisting the sort to `localStorage` (key `tt-index-sort`, JSON `{col, dir}`), matching the existing `tt-` client-state convention (`tt-last-session:<id>`). Corrupt/missing values fail open to the default order.
- **Server-emitted sort keys over DOM text scraping:** rows get `data-tasks` (int), `data-status-rank` (int via new `statusRank` func, fallback large rank for unknown statuses), and `data-updated` (raw string). Text columns (Change, Title, Prefix) sort on cell text. This keeps JS decoupled from display formatting and keeps workflow order server-owned.
- **`statusRank` as a template func** mirrors how `statusClass` already maps status strings for presentation; no `changeSummary` schema change, so the JSON API is untouched.
- **Accessibility:** header buttons keep a stable accessible name (column name); the active sort is announced via `aria-sort` on the `<th>`.
- **Compatibility:** templates are embedded at build time, so verification requires a binary rebuild (`CGO_ENABLED=0 go build -o lessmess ./cmd/lessmess`) and server restart.

## Implementation approach

1. IDX-00 renders sortable markup: `<th>` headers wrap `<button class="sort-btn" data-sort-col="…">` with an indicator span; `<tr>` gains `data-tasks`, `data-status-rank`, `data-updated`. `render_test.go` asserts attributes and that unknown statuses still render (rank fallback).
2. IDX-01 wires behavior: `initIndexSort` reads stored state, applies it after DOM ready, handles clicks (determine column → compute direction → sort `<tbody>` rows by key → update indicators and `aria-sort` → persist), and styles indicators in `app.css`.

## Testing and verification

- `go vet ./...` and `go test ./...` from the repo root; `TestIndexNewestFirst` and `TestIndexHTML` must keep passing, with new assertions for sort markup.
- Manual UI check against the rebuilt binary: sort each column both directions, confirm persistence across a simulated SSE reload (touch a `changes/` file), confirm default order with cleared storage, and confirm keyboard activation of header buttons.

## Risks and mitigations

- **Stale stored sort after future column renames:** unknown `col` values are ignored (fall back to default), so schema drift cannot break the page.
- **Row set changes between renders (SSE reload):** sorting is idempotent over whatever rows exist post-reload; persistence re-applies on load only.
- **Status vocabulary change:** `statusRank` derives from `model.TaskStatusOrder`, the same single source the board uses; unknown statuses sort last deterministically.

## Acceptance criteria

- All six data columns sort asc/desc from header clicks, with a visible direction indicator and correct `aria-sort`.
- Status sorts in workflow order; Tasks numerically; Updated by date string.
- Sort choice survives an SSE-triggered page reload; cleared storage restores newest-first default.
- Default page load order is unchanged; `go vet ./...`, `go test ./...`, and `lessmess validate` pass.

## Tasks

1. [IDX-00](tasks/00-sortable-index-markup.md): Sortable index table markup and server-emitted sort keys.
2. [IDX-01](tasks/01-index-sort-behavior.md): Client-side sorting, indicators, and persistence.
