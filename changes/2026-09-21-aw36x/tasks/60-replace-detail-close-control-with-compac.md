# MAC-60: Use compact back arrows in Chat and details

## Objective

Replace the prominent Chat and detail-reader close buttons with compact back
affordances.

## Dependencies

- MAC-59.

## Scope

- Use a back arrow for task, plan, review, and ledger details.
- Use the same back-arrow language for the main Chat header.
- Place the control at the start of the detail header.
- Keep the button chrome compact and use a clear tail-free left chevron.

## Implementation steps

1. Update every detail-reader header to use one consistent back control.
2. Replace Chat's top close icon with the same left-aligned arrow language.
3. Draw one consistent tail-free left chevron for both controls.
4. Update rendering contracts and run full verification.

## Verification

- Detail reader render contracts.
- Full tests, vet, JavaScript syntax, build, workflow validation, and diff check.

## Completion criteria

Every document detail and Chat have a compact left-aligned chevron back control
instead of a top-right close icon.

## Files affected

- `web/templates/partials.html`
- `web/templates/layout.html`
- `web/static/app.css`
- `internal/server/render_test.go`

## Notes

- The existing `data-close-detail` behavior and focus restoration are reused.
- The controls are borderless and compact; the icon is drawn from two borders
  so it reads as a simple `<` shape without an arrow tail.
- Verified with focused detail contracts, full tests, vet, JavaScript syntax,
  static build, workflow validation, diff check, and served HTML/CSS checks.
