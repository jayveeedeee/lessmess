# MAC-52: Simplify compact Chat headers and auxiliary close behavior

## Objective

Remove redundant compact Chat headers and make the persistent header close
the active auxiliary view before closing Chat.

## Dependencies

- MAC-51.

## Scope

- Use one header bar in compact Chat, including Work, Agents, and Controls.
- Reflect the active auxiliary view in the persistent header title.
- Make the persistent close button return from an auxiliary view to Chat.
- Remove the separate Back to Chat controls without changing desktop drawers.

## Implementation steps

1. Centralize the compact header title and close-label state.
2. Route the Chat header close action through auxiliary-view dismissal.
3. Remove redundant compact auxiliary headers and back buttons.
4. Update UI contracts and verify responsive behavior.

## Verification

- Render-contract tests for the single-header and hierarchical-close behavior.
- Full tests, vet, JavaScript syntax, build, validation, and diff check.

## Completion criteria

Compact auxiliary views have one header, no Back to Chat button, and their
close button returns to the Chat transcript without closing the session UI.

## Files affected

- `web/templates/layout.html`
- `web/static/app.js`
- `web/static/app.css`
- `internal/server/render_test.go`

## Notes

- Compact Work, Child activity, and Session controls now reuse the persistent
  Chat header, which displays the active view name and hides session state.
- The persistent X dismisses the active auxiliary view first; only a second X
  from the transcript closes Chat. Escape and backdrop behavior use the same
  hierarchy.
- Removed all Back to Chat controls and compact-only secondary header rows.
- Follow-up visual feedback identified the root session ancestry row as another
  duplicate header. Root sessions now omit that row entirely; child sessions
  show only actual parent-session navigation, without repeating the current
  title, idle state, or Return to change link.
- Verified with focused render contracts, `go vet ./...`, `go test ./...`,
  `node --check web/static/app.js`, `git diff --check`, a static build,
  `lessmess validate`, and served-asset checks.
