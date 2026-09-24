# MAC-67: Integrate context ring into Chat options

## Objective

Merge active-context usage into a circular Chat Options button matching the
Play/Stop control's overall size.

## Dependencies

- MAC-66

## Scope

- Make Options a 44px circular control matching Play/Stop.
- Render context usage as the Options button's outer conic ring.
- Keep only the vertical-options symbol in the center, with no visible number.
- Preserve exact context details in accessible labels and Controls.

## Implementation steps

1. Move the usage indicator inside the Options button.
2. Refactor donut CSS into a full-size border ring around the symbol.
3. Remove visible percentage text and update accessible Options labeling.
4. Update rendering and CSS contracts.

## Verification

- Run focused Chat composer and context-usage tests.
- Run full tests, vet, JavaScript syntax, validation, build, and served checks.

## Completion criteria

Options and Play/Stop are the same 44px diameter, and Options visually fills an
outer context ring without displaying a number.

## Files affected

- `web/templates/layout.html`
- `web/static/app.js`
- `web/static/app.css`
- `internal/server/render_test.go`

## Notes
