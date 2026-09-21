# MAC-29: Slim mobile chat header with overflow menu

## Objective

Reduce the compact Chat header to navigation, title/state, relevant Work or
Agents actions, and one overflow menu for infrequent controls.

## Dependencies

- MAC-25.
- MAC-26.

## Scope

- Move Terminal, Controls, lifecycle, and management entry points into overflow.
- Keep Work/Agents visible only when relevant.
- Preserve title/state space, 44px targets, Escape/backdrop, and focus return.
- Keep desktop explicit controls unchanged.

## Implementation steps

1. Add overflow structural hooks while preserving current button IDs.
2. Route overflow choices through existing handlers/view stack.
3. Apply contextual visibility and compact header CSS.
4. Pin accessible labels and open/close behavior in tests.

## Verification

- Check bound, unbound, child-agent, busy, and idle headers at compact widths.
- Verify all existing controls remain reachable.

## Completion criteria

The compact header remains a calm single row and gives the title/state priority.

## Files affected

- `web/templates/layout.html`
- `web/static/app.js`
- `web/static/app.css`
- `internal/server/render_test.go`

## Notes

Desktop header layout is unchanged.
