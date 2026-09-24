# MAC-66: Add breadcrumb back control to app bar

## Objective

Place a one-level breadcrumb back control on the left of the persistent app bar
and align the current-location dropdown on the right.

## Dependencies

- MAC-65

## Scope

- Add a tail-free left chevron to the collapsed location bar.
- Navigate to the immediately preceding actual-path breadcrumb item.
- Keep the current-location label and dropdown affordance right-aligned.
- Disable Back when no parent breadcrumb exists.

## Implementation steps

1. Add accessible Back markup to the location control.
2. Reuse breadcrumb ancestor activation for one-level navigation.
3. Synchronize Back labels and availability with every trail update.
4. Update responsive styling and contracts.

## Verification

- Run focused breadcrumb rendering/controller tests.
- Run full tests, vet, JavaScript syntax, validation, build, and served checks.

## Completion criteria

The app bar presents Back on the left and the active location on the right, and
Back reliably returns one level through the actual navigation path.

## Files affected

- `web/templates/layout.html`
- `web/static/app.js`
- `web/static/app.css`
- `internal/server/render_test.go`

## Notes
