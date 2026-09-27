# PILL-05: Show selected message actions in composer pill

## Why

The current Copy/Fork/Revert menu opens next to a clicked transcript message.
Long messages can extend beyond the viewport, leaving that menu out of sight.
The composer stays visible and should host actions for the selected message.

## What

- Select/deselect a copyable transcript message by click or Enter/Space, with
  an accent highlight and accessible selected state; no inline popup menu.
- Show message-specific icon segments in the composer action pill: Copy for
  assistant/user messages, plus Fork and Revert for user messages. Keep the
  left Send (idle) or Stop (busy) segment; preserve the existing action flows.
- Make selection mutually exclusive with a pending follow-up. Selecting a
  different message switches actions; typing restores normal composer actions
  without changing the draft. Escape and clicking outside dismiss selection.
- Preserve selected state and keyboard focus through transcript polls/history
  swaps; clear selection when the message disappears or the session changes.
- Use the existing shared segment/width/animation mechanism for all modes;
  do not make a second message-specific toolbar.

## Files affected

- `web/templates/partials.html`
- `web/templates/layout.html`
- `web/static/app.js`
- `web/static/app.css`
- `internal/server/render_test.go`

## Verification

- Desktop and mobile selection shows the right controls at the composer,
  including when the selected message is taller than the viewport. Copy uses
  the message markdown; Fork opens a child session; Revert opens the existing
  confirmation dialog. Pending selection and draft actions still work.
- Toggle, Escape, typing, session switch, and poll behavior preserve draft,
  focus, and mutual exclusion. `go vet ./...`, `go test ./...`, workflow
  validation, and rebuilt-service browser checks pass.
