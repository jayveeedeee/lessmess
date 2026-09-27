# PILL-04: Select pending message for cancel pill

## Why

The inline pending message currently carries its own Cancel button. The user
wants the pending bubble to be selectable instead: selecting it highlights its
dotted accent outline and turns the composer action into a two-segment Stop | ×
pill; selecting it again deselects it.

## What

- Make pending message rows click- and keyboard-selectable (`aria-pressed`),
  using a brighter accent border when selected. The selected ID is local UI
  state and clears on session switch, typing a new draft, or when the item is
  delivered/cancelled. Preserve selection across inbox refreshes and transcript
  snapshot swaps; preserve keyboard focus when rerendering.
- Remove the inline Cancel button. Put an icon-only × action next to Stop
  when a pending message is selected, calling the existing inbox DELETE for
  that ID; Stop remains on the left. Clicking the same bubble again returns
  to the normal Stop or Stop | Steer | Queue composer state without losing a
  draft.
- Keep the action-pill mechanism reusable: model 1-, 2-, and 3-segment modes
  with shared sizing and segment transitions rather than building a separate
  cancel control. Keep fixed 22px end radii and reduced-motion behavior.
- Update the embedded UI contract and verify live UI on desktop/mobile.

## Files affected

- `web/templates/layout.html`
- `web/static/app.js`
- `web/static/app.css`
- `internal/server/render_test.go`

## Verification

- Clicking a pending message highlights it and expands Stop | ×; clicking it
  again deselects it; pressing Enter/Space toggles selection too.
- × cancels the selected item, removes its pending bubble, and collapses the
  control. Stop still interrupts. Draft text survives select/deselect; typing
  deselects and returns to the three-segment composer while busy.
- Selection survives poll/inbox rerenders, clears if the item disappears, and
  works at mobile widths. `go vet ./...`, `go test ./...`, workflow validation,
  rebuilt-service UI check pass.
