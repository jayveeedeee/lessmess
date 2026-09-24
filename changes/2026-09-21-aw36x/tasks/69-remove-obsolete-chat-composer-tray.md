# MAC-69: Remove obsolete Chat composer tray

## Objective

Remove the obsolete horizontal tray behind the unified Chat composer so the
field sits directly on the conversation background.

## Dependencies

- MAC-68

## Scope

- Remove the composer's top divider.
- Remove its separate surface background.
- Preserve field spacing and bottom safe-area padding.

## Implementation steps

1. Make the outer composer form transparent and borderless.
2. Keep the unified input field as the sole visible composer surface.
3. Pin the visual contract and verify responsive behavior.

## Verification

- Run focused Chat composer tests.
- Run full tests, vet, JavaScript syntax, validation, build, and served checks.

## Completion criteria

No horizontal background block or divider surrounds the rounded Chat input.

## Files affected

- `web/static/app.css`
- `internal/server/render_test.go`

## Notes
