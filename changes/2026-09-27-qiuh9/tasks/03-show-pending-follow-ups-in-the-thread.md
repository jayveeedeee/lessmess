# PILL-03: Show pending follow-ups in the thread

## Why

The old `#chat-inbox` strip showed queued messages above the composer, with a
delivery selector and Cancel. The desired UI is a pending user message in the
scrollable transcript itself, distinguished from delivered messages by a
dotted outline; PILL-04 moves cancellation into the selected-message pill.

## What

- Render inbox-backed pending follow-ups after the latest transcript snapshot,
  as safe text in user-shaped bubbles with a dotted outline and a Queued or
  Steering status. Keep unsupported or non-user pending items
  identifiable rather than inventing message text.
- Remove the separate fixed-position inbox strip and delivery-mode selector.
  Keep the existing DELETE cancellation endpoint and capability gate for the
  selection-based control in PILL-04.
- Reattach pending bubbles after whole-snapshot polling swaps without
  duplicating already-delivered messages; preserve scroll position, mobile
  layout, and the draft/stop/steer/queue composer behavior.
- Update the embedded UI contract test and verify against a rebuilt binary.

## Files affected

- `web/templates/layout.html`
- `web/static/app.js`
- `web/static/app.css`
- `internal/server/render_test.go`

## Verification

- A queued follow-up appears in the thread as a dotted user message; no
  pending strip or delivery selector remains. PILL-04 owns its cancel control.
- It survives a snapshot poll, disappears when cancelled or delivered, and
  does not duplicate a delivered transcript message during a stale inbox read.
- `go vet ./...`, `go test ./...`, and workflow validation pass; rebuilt
  running service serves the new UI.
