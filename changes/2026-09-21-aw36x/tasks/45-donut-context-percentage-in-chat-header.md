# MAC-45: Donut context percentage in Chat header

## Objective

Replace the textual Chat context indicator with a compact percentage donut.

## Dependencies

- MAC-44.

## Scope

- Show only the percentage inside a circular progress ring.
- Omit the visible word `Context`.
- Preserve exact token counts in the tooltip and accessible label.
- Show a dash when active usage is unavailable and retain warning color.

## Implementation steps

1. Add semantic donut markup to the Chat header.
2. Drive the ring and center label from authoritative usage data.
3. Add compact desktop/mobile and unavailable/warning styles.

## Verification

- Render/JavaScript/CSS contracts, full tests/vet, syntax, build, validation,
  and diff check.

## Completion criteria

The header displays a compact donut with the numeric percentage and no visible
context label or percent sign.

## Files affected

- `web/templates/layout.html`
- `web/static/app.js`
- `web/static/app.css`
- `internal/server/render_test.go`

## Notes

The ring fill clamps at 100%; the numeric label remains authoritative.
