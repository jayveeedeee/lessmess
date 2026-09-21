# MAC-25: Single-pane mobile navigation shell for the chat overlay

## Objective

Make Chat the only persistent pane at compact widths (roughly 840px and below).
Work, plan, task, agents, controls, and management become temporary full-width
views with explicit return navigation.

## Dependencies

- None.

## Scope

- Add one compact-width Chat view stack.
- Convert existing mobile drawers/sheets into full-width views.
- Preserve transcript scroll, drafts, chips, and each view's return position.
- Back, Escape, and backdrop close only the top-most view.

## Implementation steps

1. Introduce structural view hooks and a `data-chat-view` state on Chat.
2. Add a small push/pop controller with focus and ARIA handoff.
3. Consolidate compact Chat CSS under the wider mobile breakpoint.
4. Pin structural and state-preservation behavior with tests.

## Verification

- `node --check web/static/app.js` and focused render tests.
- Check 375px, 390px, phone landscape, and compact split widths.

## Completion criteria

No auxiliary surface sits beside Chat at compact widths, and returning to Chat
restores its draft and reading position.

## Files affected

- `web/templates/layout.html`
- `web/static/app.js`
- `web/static/app.css`
- `internal/server/render_test.go`

## Notes

Desktop behavior remains unchanged in this foundation task.
