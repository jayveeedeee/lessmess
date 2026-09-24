# MAC-77: Right-align desktop breadcrumb menu

## Objective

Align the desktop breadcrumb dropdown with its right-aligned trigger while
preserving the full-width mobile menu.

## Dependencies

- MAC-76

## Scope

- Anchor the desktop location menu to the right edge of the app bar.
- Keep the compact breakpoint menu fixed edge-to-edge.

## Implementation steps

1. Change the base dropdown anchor from left to right.
2. Retain explicit left and right mobile overrides.
3. Update the responsive styling contract.

## Verification

- Run focused breadcrumb tests.
- Run full tests, vet, validation, build, and served checks.

## Completion criteria

The desktop dropdown opens beneath the right-side trigger and mobile is
unchanged.

## Files affected

- `web/static/app.css`
- `internal/server/render_test.go`

## Notes
