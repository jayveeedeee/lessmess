# MAC-28: Message-first composer with a single + action sheet

## Objective

Make the composer message-first: full-width textarea and primary Send action,
with infrequent file/reference/skill actions behind one `+` menu and Interrupt
visible only while busy.

## Dependencies

- None.

## Scope

- Preserve existing input and API hooks while restructuring presentation.
- Add one action sheet for Attach file, Add reference, and Add skill.
- Keep selected items visible as removable chips.
- Preserve busy delivery behavior and attachment limits.

## Implementation steps

1. Restructure the compose row around `+`, textarea, and Send.
2. Add accessible action-sheet open/close and focus return behavior.
3. Gate Interrupt on authoritative busy state.
4. Extend render/UI tests for the new composer contract.

## Verification

- Check idle, busy, attachment, reference, skill, send, and clear flows at
  375px/390px and desktop.

## Completion criteria

The text field is visually primary and uncommon actions remain one tap away
without permanent toolbar clutter.

## Files affected

- `web/templates/layout.html`
- `web/static/app.js`
- `web/static/app.css`
- `internal/server/render_test.go`

## Notes

Do not change existing file-count or size limits.
