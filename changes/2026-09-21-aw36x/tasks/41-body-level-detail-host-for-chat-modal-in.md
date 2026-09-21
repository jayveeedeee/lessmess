# MAC-41: Body-level detail host for Chat modal interaction

## Objective

Make task and plan details a body-level overlay so browser inert behavior cannot
disable the modal through a page-content ancestor.

## Dependencies

- MAC-40.

## Scope

- Move the shared `#detail` target out of board/index content and directly
  beneath the layout's `<main>`.
- Keep one detail target on pages that open task, plan, review, or ledger views.
- Preserve existing HTMX targets and modal behavior.

## Implementation steps

1. Render the detail host at layout level for index and board pages.
2. Remove nested duplicate hosts from page templates.
3. Pin body-level placement with render tests and live pointer checks.

## Verification

- Desktop and 390px Chrome pointer checks for plan/task close and Contents,
  focused/full tests, vet, JavaScript syntax, build, validation, and diff check.

## Completion criteria

The detail overlay has no inert page-content ancestor and all modal controls
receive pointer events while Chat remains open.

## Files affected

- `web/templates/layout.html`
- `web/templates/board.html`
- `web/templates/index.html`
- `internal/server/render_test.go`

## Notes

This removes reliance on nested inert interoperability across browsers.
