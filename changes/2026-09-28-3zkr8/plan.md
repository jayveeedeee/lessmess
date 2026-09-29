# 2026-09-28-3zkr8: Changes page status filter

- Change ID: 2026-09-28-3zkr8
- Created: 2026-09-28
- Branch: —
- Status: tracked in the tool-owned JSON state (.lessmess/workflow/)

## Objective and context

The changes page (`GET /{$}`, `web/templates/index.html`) lists every change as a
card with a status pill (Planned / In progress / Blocked / Done / Cancelled). The
page can already sort the cards (the `#change-sort` select, persisted in
localStorage), but it cannot filter them: with many changes, finding the ones
still in progress means scanning every pill by eye. This change adds a status
filter alongside the existing sort select.

## Current behavior

- `web/templates/index.html:29-36` renders one `#change-sort` select with the
  five fixed orders (default `updated:desc`, "Newest first"); sorting happens
  client-side in `web/static/app.js` (`initIndexSort`, `applyChangeSort`,
  `indexSortState`, localStorage key `tt-index-sort`, corrupt values fail open).
- Cards (`a.change-card`, index.html:38-49) carry `data-tasks`,
  `data-status-rank`, `data-updated`, `data-title`; `buildChangeCard` in app.js
  mirrors this markup for the SSE-driven rebuild (`refreshChangeCards` refetches
  the `GET /` JSON list and re-sorts).
- Each card shows its status as a `.pill.status-*` span, but there is no control
  that filters the list by it, and no labels/tags exist anywhere in the workflow
  state model (confirmed: `internal/model/state.go` has no label field).

## Target behavior

- A second select, `#change-filter-status`, sits beside `#change-sort` with the
  options: All (default), Planned, In progress, Blocked, Done, Cancelled — the
  fixed five overall statuses from `internal/model/model.go:36-40`.
- Choosing a status hides every card whose status differs (via a new
  `data-status` attribute on each card, carrying the raw status string).
- Option labels show live counts, e.g. "In progress (3)", recomputed on every
  list refresh; "All" shows the total.
- The selection persists across reloads in localStorage
  (`tt-index-filter-status`); corrupt values fail open to All — the same
  contract as `tt-index-sort`.
- The filter re-applies after the SSE refresh rebuilds the cards
  (`refreshChangeCards`), so live updates respect the active filter.
- When the active filter matches zero cards, a muted "no changes match" line
  shows in place of the list.

## Scope

- `web/templates/index.html` — new select; `data-status` on `a.change-card`.
- `web/static/app.js` — filter state, apply/wire logic beside the existing sort
  block (~lines 5248-5385); `buildChangeCard` gains `data-status`;
  `refreshChangeCards` re-applies filter + counts.
- `web/static/app.css` — seat the second select in the controls row.
- `internal/server/render_test.go` — assert the new select and card attribute.

## Non-goals

- No labels/tags feature: the workflow state schema is untouched.
- No server-side filtering or query parameters: the list endpoint keeps serving
  every (non-archived) change; filtering stays client-side like sorting.
- No change to the sort orders, the default sort, or the Discussions list.
- No filtering anywhere else (board, explorer, settings).

## Design decisions

- **Status, not labels.** The requested "filter by label types" landed on the
  existing status pills: no label concept exists in the model, and adding one
  was rejected as out of proportion (user decision).
- **Second dropdown, not chips.** User picked a `<select>` beside the existing
  sort select for compactness and consistency, over a chip row.
- **Client-side, mirroring the sort block.** The page already sorts entirely
  client-side from `data-*` attributes with localStorage persistence; the filter
  follows the same shape, so SSE refreshes and the JSON-rebuild path stay
  consistent and the server stays stateless.
- **Fixed five options, not dynamic.** The overall status set is a closed
  constant in the model; hardcoding the five options (like the five sort orders)
  keeps the template simple and matches the render-test contract.
- **Counts on option labels.** Cheap to compute client-side from the rendered
  cards, and makes the dropdown informative without extra UI.

## Acceptance criteria

- The changes page shows `#change-filter-status` beside `#change-sort` with
  All + the five statuses, each labeled with a count.
- Selecting a status leaves only matching cards visible; All restores the list.
- The choice survives a page reload and is re-applied after an SSE-driven card
  rebuild; corrupt localStorage values fall back to All.
- A filter that matches nothing shows the muted empty line.
- `render_test.go` covers the new select and the `data-status` attribute;
  `go vet ./... && go test ./...` pass and `lessmess validate` is clean.
- Rebuilt binary serves the feature (assets are `go:embed`-ed).

## Tasks

1. (task breakdown is maintained by the tool; see the board)
