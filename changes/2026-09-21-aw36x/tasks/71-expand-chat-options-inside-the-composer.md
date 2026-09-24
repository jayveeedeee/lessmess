# MAC-71: Expand Chat options inside the composer

## Objective

Replace the Chat Options popover with a menu mode contained inside the rounded
composer field.

## Dependencies

- MAC-70

## Scope

- Replace the textarea region with Options while the menu is open.
- Expand the floating composer to fit the menu.
- Change the context-ring dots control into a Close control while open.
- Keep Play/Stop visible in the bottom-right action row.
- Preserve the current draft and existing menu navigation behavior.

## Implementation steps

1. Move the Options menu into the compose field's content flow.
2. Remove the obsolete popover backdrop and styling.
3. Add explicit open/closed composer mode and control labeling.
4. Keep dynamic floating-composer height synchronization intact.
5. Update rendering, behavior, and styling contracts.

## Verification

- Run focused composer and navigation tests.
- Run full tests, vet, JavaScript syntax, validation, build, and served checks.

## Completion criteria

Options opens within the rounded input field, the left control becomes Close,
and Play/Stop remains visible while the field expands to contain the menu.

## Files affected

- `web/templates/layout.html`
- `web/static/app.js`
- `web/static/app.css`
- `internal/server/render_test.go`

## Notes
