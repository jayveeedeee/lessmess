# MAC-74: Preserve rounded quick-action container

## Objective

Keep the composer's rounded outer container unchanged when quick actions open
while retaining square menu cells inside it.

## Dependencies

- MAC-73

## Scope

- Preserve the existing 22px composer radius in both input and menu modes.
- Clip square quick-action cells and dividers to the rounded outer boundary.
- Keep all internal action corners at zero radius.

## Implementation steps

1. Remove the menu-mode outer-radius override.
2. Clip composer contents to the established rounded container.
3. Update the composer visual contract.

## Verification

- Run focused composer tests.
- Run full tests, vet, validation, build, and served checks.

## Completion criteria

Opening quick actions does not alter the outer composer silhouette, while the
menu cells remain square within it.

## Files affected

- `web/static/app.css`
- `internal/server/render_test.go`

## Notes
