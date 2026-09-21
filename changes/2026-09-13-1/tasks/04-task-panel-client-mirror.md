
# TUI-04: Task panel client mirror and sync

Status: see [../ledger.md](../ledger.md).

## Objective

Fill the panel from the board DOM and keep it live: render grouped task rows
when a change terminal opens, re-sync after every board swap (SSE-driven),
and open the task detail modal when a row is clicked — all client-side, no
new endpoints.

## Dependencies

- TUI-03 (panel markup, styles, and z-index exist).

## Scope

- `web/static/app.js` only.

## Implementation steps

1. `syncTerminalTasks()`: read `#board .cards[data-status]` columns in DOM
   order; for each non-empty column emit a group (status pill + count) and
   one row per `.card` (task chip from `data-task`, title text from
   `.card-title`). Empty board / no `#board` → empty state ("No tasks" or
   nothing, panel hidden).
2. Each row is an anchor copying the card title's `hx-get` URL with
   `hx-target="#detail"`, `hx-swap="innerHTML"`, `Accept: text/html` headers
   (same contract as board cards); after filling the panel call
   `htmx.process(panel)` so the new anchors work (same pattern as the
   explorer tree refresh).
3. Show the panel only for change terminals: in `openTerminal`, unhide
   `#terminal-tasks` when `page === "board"` and `#board[data-change]` exist,
   then `syncTerminalTasks()`; hide it otherwise and in `closeTerminal`.
4. Live sync: extend the existing `htmx:afterSwap` listener (or add one) so a
   `#board` swap re-runs `syncTerminalTasks()` when the panel is visible —
   this rides the existing SSE debounce, no new listeners or timers.
5. Keep the Sortable/drag handlers off the panel; rows are links, not cards.

## Verification

- `go build ./...` and full `go test ./...` green (no Go changes expected,
  but assets are embedded).
- Manual checks live in TUI-05; here, verify by inspection that:
  - non-board terminals (index Discussions, explorer chat) never unhide the
    panel;
  - panel re-sync cannot fire without a visible panel (guard);
  - repeated `openTerminal`/`closeTerminal` cycles leave no duplicate
    listeners.

## Completion criteria

- Panel renders grouped tasks on terminal open, re-syncs on board swaps, and
  row clicks open `#detail` above the terminal; nothing changes for
  unassigned terminals.

## Files affected

- `web/static/app.js`

## Notes

- The board DOM is the data contract: `.cards[data-status]`, `.card[data-task]`,
  `.card-title[hx-get]` — all from `boardFragment` in `partials.html`.
- Implemented: `syncTerminalTasks()` (+ `esc()`, client `statusClass` mirroring
  `render.go`) renders `.ttp-head` + per-status `.ttp-group`s (pill, count,
  chip + title rows carrying the card's own `hx-get`/`hx-headers` to
  `#detail`), then `htmx.process(panel)`. `openTerminal` unhides the panel
  only when `page === "board"` and `#board[data-change]` exists;
  `closeTerminal` hides+clears it; the existing `htmx:afterSwap` `#board`
  hook also calls `syncTerminalTasks()` (guard: returns early when hidden).
- One deliberate behavior change found during implementation: the Escape key
  handler now closes the top-most layer — a visible `#detail` before the
  terminal — because `#detail` now stacks above the overlay.
- Verified: full `go test ./...`, `node --check web/static/app.js`, live
  server serves the new app.js/app.css/markup (2026-09-13). Browser checks
  live in TUI-05.
- Iteration 3 (user request, 2026-09-13): the sync renders
  `.ttp-scroll` (groups) plus a fixed `.ttp-foot` containing a full-width
  Plan anchor that mirrors the board's Plan button request exactly
  (`hx-get="/changes/<id>/plan"`, `hx-target="#detail"`, `hx-swap="innerHTML"`)
  using `board.dataset.change`; the footer renders whenever a board exists,
  including the zero-task empty state. `GET /changes/{id}/plan` confirmed to
  return the `planDetail` modal fragment for htmx requests.
