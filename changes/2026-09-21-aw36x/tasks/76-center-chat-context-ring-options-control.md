# MAC-76: Center Chat context-ring options control

## Objective

Geometrically center the Chat Options glyph and context ring so the usage
border has uniform thickness on every side.

## Dependencies

- MAC-75

## Scope

- Remove the redundant native button border beneath the context ring.
- Force the Options control to an exact 44px square.
- Size the context ring from the button box rather than fixed offsets.
- Replace the font-rendered vertical ellipsis with centered CSS dots.
- Preserve the flat Close treatment in menu mode.

## Implementation steps

1. Normalize the closed Options button geometry.
2. Make the context indicator fill the normalized box exactly.
3. Draw three centered dots without font metrics.
4. Draw the open-state Close mark with the same CSS glyph element.
5. Update visual contracts.

## Verification

- Run focused composer tests.
- Run full tests, vet, validation, build, and served checks.

## Completion criteria

The context ring is uniformly thick and the three dots are optically centered.

## Files affected

- `web/static/app.js`
- `web/static/app.css`
- `internal/server/render_test.go`

## Notes
