# MAC-75: Restore visible Compact action icon

## Objective

Ensure the Compact quick action displays a clear icon in both enabled and
disabled states on mobile browsers.

## Dependencies

- MAC-74

## Scope

- Replace the unreliable dense SVG path with a simple compression glyph.
- Explicitly preserve the disabled icon's stroke and opacity.

## Implementation steps

1. Replace the Compact SVG path with a simple line-and-chevron symbol.
2. Add a disabled-state SVG visibility rule.
3. Update the rendering and styling contract.

## Verification

- Run focused composer tests.
- Run full tests, vet, validation, build, and served checks.

## Completion criteria

Compact always shows a recognizable icon beside its label.

## Files affected

- `web/templates/layout.html`
- `web/static/app.css`
- `internal/server/render_test.go`

## Notes
