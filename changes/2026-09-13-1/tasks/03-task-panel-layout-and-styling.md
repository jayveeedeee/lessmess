---
id: TUI-03
title: Task panel layout and styling
---

# TUI-03: Task panel layout and styling

Status: see [../ledger.md](../ledger.md).

## Objective

Give the terminal window a lessmess-native right-hand column for the task
panel: markup in `layout.html`, styling in `app.css`, and the z-index fix that
lets the task detail modal open above the terminal.

## Dependencies

None (client-side markup/styles; TUI-04 fills the panel with data).

## Scope

- `web/templates/layout.html`: inside `.terminal-window`, wrap the terminal
  body in a flex row — `#terminal-container` (flex 1) plus a new
  `#terminal-tasks` aside (hidden by default; TUI-04 unhides it for change
  terminals).
- `web/static/app.css`: panel at 20% width with a min-width floor (~200px),
  own background/border separation from xterm, styles for group headers
  (status pill + count) and compact task rows (chip + title, hover affordance
  for the click-to-open-detail behavior added in TUI-04); an empty-state
  style.
- `#detail` z-index from 20 to above the terminal overlay (30) so task
  details open on top of the terminal.
- No JavaScript in this task beyond what templates already carry.

## Implementation steps

1. Restructure `.terminal-window` so head stays on top and a new
   `.terminal-body` flex row holds `#terminal-container` and
   `#terminal-tasks` (status line stays below, spanning full width).
2. Style `#terminal-tasks` (20% / min-width, border-left, scrolling for long
   lists, theme-aware colors via existing CSS variables).
3. Add `.ttp-group` / `.ttp-row` / empty-state styles reusing the existing
   `.pill status-*` classes and `--st-*` variables so panel statuses match
   the board exactly in both themes.
4. Bump `#detail` z-index above 30; confirm the notif modal (25) interplay is
   unaffected in practice.
5. Keep `#terminal-tasks` `hidden` in the template — it must not appear for
   unassigned terminals or before TUI-04 lands.

## Verification

- `go build ./...` (templates are embedded; build proves parse) and
  `go test ./internal/server/` (render tests) green.
- Manual spot check can be deferred to TUI-05; at minimum verify the board,
  index, and explorer terminals render unchanged with the panel hidden.

## Completion criteria

- Markup and styles merged; terminal renders identically to before when the
  panel is hidden; `#detail` layers above the terminal overlay.

## Files affected

- `web/templates/layout.html`
- `web/static/app.css`

## Notes

- z-index stack before this change: `#detail` 20, modals 25,
  `#terminal-overlay` 30.
- Implemented: `.terminal-body` flex row holds `#terminal-container` +
  `<aside id="terminal-tasks" hidden>`; panel is `flex: 0 0 20%` with
  200px min / 340px max, theme-variable styling, `.ttp-head/-group/-row/-empty`
  classes reusing `.pill status-*` and `.chip`; `#detail` z-index 20→40 (no
  practical interplay with the 25-layer modals — they are never stacked on
  the terminal). Verified: `go vet`, full `go test`, live 200s on
  index/board/explorer with the new markup served (2026-09-13).
- Iteration 2 (user visual feedback, 2026-09-13): the category highlight is
  now a full-bleed band — `.ttp-group-head` carries the `status-*` class
  itself (the shared status classes set background/fg, so colors stay
  identical to the board), negative horizontal margins cancel the panel's
  0.6rem padding, no radius/margins/borders of its own, count right-aligned
  inheriting the band foreground. Rows get `cursor: pointer` (htmx anchors
  have no href, so the default cursor was wrong for a clickable item). The JS
  emits the status class on the band div instead of an inner pill span.
- Iteration 3 (user request, 2026-09-13): fixed bottom footer with a Plan
  button. `#terminal-tasks` is now a flex column: `.ttp-scroll` holds the
  padding and owns `overflow-y: auto`, `.ttp-foot` (top border, surface bg)
  stays put, so scrolling happens only between the panel top and the footer
  and nothing is ever scrolled out of reach. The full-bleed band math is
  unchanged (same 0.6rem horizontal padding, now on `.ttp-scroll`).
