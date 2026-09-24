# MAC-73: Square and normalize Chat quick actions

## Objective

Make the open Chat quick actions read as a dedicated, square-edged menu with
uniform cells and no Play/Stop control.

## Dependencies

- MAC-72

## Scope

- Hide Play/Stop completely while quick actions are open.
- Replace the circular context-ring close state with a flat Close menu row.
- Give every quick action an identical fixed height.
- Remove tile and open-container rounding.
- Use only shared one-pixel dividers between menu cells.
- Keep Compact's icon clearly visible while disabled.

## Implementation steps

1. Add a Close label to the existing Options control's open state.
2. Hide non-menu composer actions and context ring while open.
3. Normalize the action grid geometry and divider treatment.
4. Correct disabled Compact icon visibility.
5. Update behavior and styling contracts.

## Verification

- Run focused composer and navigation tests.
- Run full tests, vet, JavaScript syntax, validation, build, and served checks.

## Completion criteria

The open composer consists only of equal-height square menu cells and a flat
Close row, separated by one-pixel rules, with no Play/Stop control visible.

## Files affected

- `web/templates/layout.html`
- `web/static/app.css`
- `internal/server/render_test.go`

## Notes
