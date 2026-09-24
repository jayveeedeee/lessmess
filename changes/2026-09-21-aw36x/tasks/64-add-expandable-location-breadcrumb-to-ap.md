# MAC-64: Add expandable location breadcrumb to app bar

## Objective

Turn the persistent app bar into an expandable vertical breadcrumb that shows
the user's actual navigation path through changes, Chat views, and documents.

## Dependencies

- MAC-63

## Scope

- Keep the active location visible in the collapsed app bar.
- Slide down a vertical root-to-current breadcrumb on activation.
- Track actual navigation, including Chat, Work, Agents, Controls, Plan, and
  task/detail destinations.
- Let ancestor selections unwind to that location.
- Keep the bar available above Chat and detail overlays on desktop and mobile.

## Implementation steps

1. Render board/change context into an accessible app-bar location control.
2. Maintain an actual-path location stack as views open and close.
3. Wire ancestor actions to close or restore overlays and auxiliary views.
4. Resolve overlay stacking, inert ownership, viewport, and compact layout.
5. Add rendering and JavaScript/CSS contract tests.

## Verification

- Run focused navigation and rendering tests.
- Run full tests, vet, JavaScript syntax, validation, build, and served checks.
- Verify compact and desktop breadcrumb behavior against active Chat/detail
  locations.

## Completion criteria

The collapsed app bar names the active breadcrumb item, expands into the actual path
used to reach it, and every ancestor returns to that level without losing
relevant Chat state.

## Files affected

- `web/templates/layout.html`
- `web/static/app.js`
- `web/static/app.css`
- `internal/server/render_test.go`

## Notes

The breadcrumb is path-based rather than a fixed content hierarchy: opening a
task from Chat Work produces `Changes → Change → Chat → Work → Task`.
