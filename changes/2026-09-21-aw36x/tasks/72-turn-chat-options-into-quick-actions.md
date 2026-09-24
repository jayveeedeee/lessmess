# MAC-72: Turn Chat options into quick actions

## Objective

Replace the generic Chat Options list with a responsive, icon-led quick-action
grid for the session's useful destinations.

## Dependencies

- MAC-71

## Scope

- Offer Plan and Tasks only for change-bound chats.
- Replace generic Controls with Runtime for Agent, Model, and Variant.
- Retain Compact as a first-class quick action.
- Remove Child Agents from the visible menu.
- Use flat icons and a four-column desktop/two-column mobile grid.

## Implementation steps

1. Replace menu rows with Plan, Tasks, Runtime, and Compact action tiles.
2. Bind Plan to the current change and reuse the existing Tasks panel.
3. Open Controls in a focused Runtime-only mode from the quick action.
4. Keep extended Controls available for internal lifecycle workflows.
5. Add responsive icon and grid styling and update contracts.

## Verification

- Run focused composer, navigation, controls, and work-panel tests.
- Run full tests, vet, JavaScript syntax, validation, build, and served checks.

## Completion criteria

The composer menu is an icon-led responsive quick-action grid with direct Plan,
Tasks, Runtime, and Compact access and no visible Child Agents or Controls row.

## Files affected

- `web/templates/layout.html`
- `web/static/app.js`
- `web/static/app.css`
- `internal/server/render_test.go`

## Notes
