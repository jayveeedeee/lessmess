# MAC-39: Contextual message actions on click and press

## Objective

Expose message lifecycle and copy actions only when a user selects a message.

## Dependencies

- MAC-38.

## Scope

- Clicking, tapping, or keyboard-activating a user/assistant message opens a
  floating contextual menu.
- Offer Copy, Fork, and Revert to here without reserving transcript space.
- Close on outside click, Escape, action selection, or another message opening.
- Reuse existing fork and staged-revert flows.

## Implementation steps

1. Render hidden per-message action menus with source message IDs.
2. Add click, touch-compatible click, and keyboard open/close behavior.
3. Add message-content copying and reuse lifecycle action hooks.
4. Style the menu as a non-flow popover and add contracts.

## Verification

- Focused/full Go tests, JavaScript syntax/contracts, vet, build, validation,
  and diff check.

## Completion criteria

Actions appear only for the selected message and disappear without leaving
permanent transcript chrome.

## Files affected

- `web/templates/partials.html`
- `web/static/app.js`
- `web/static/app.css`
- `internal/server/chat_test.go`
- `internal/server/render_test.go`

## Notes

Revert remains a preview-first staged operation rather than an immediate edit.
