# MAC-55: Consolidate Chat actions into one options menu

## Objective

Replace the separate Chat composer action buttons with one compact Options
menu represented by a vertical ellipsis.

## Dependencies

- MAC-54.

## Scope

- Consolidate Attach files, Add reference, Add skill, Work, Agents, and
  Controls into one menu on desktop and mobile.
- Remove the separate plus/action menu and direct Work, Agents, and Controls
  toolbar buttons.
- Preserve contextual hiding, disabled states, keyboard navigation, and focus.

## Implementation steps

1. Merge composer actions into the existing overflow menu.
2. Simplify JavaScript to one menu lifecycle and action path.
3. Unify desktop/mobile menu styling around a vertical-ellipsis trigger.
4. Update render contracts and run full verification.

## Verification

- Render and JavaScript contracts for the single menu and removed plus menu.
- Full tests, vet, JavaScript syntax, build, validation, and diff check.

## Completion criteria

The composer shows one vertical-ellipsis Options button for message additions,
Work, Agents, and Controls.

## Files affected

- `web/templates/layout.html`
- `web/static/app.js`
- `web/static/app.css`
- `internal/server/render_test.go`

## Notes

- The composer now exposes one 44px vertical-ellipsis button labeled Chat
  options on desktop and compact layouts.
- Its menu contains Attach files, Add reference, Add skill, Work, Agents, and
  Controls; Work and Agents retain their contextual hidden state.
- Removed the separate plus trigger, action sheet, backdrop, JavaScript
  lifecycle, and direct toolbar buttons.
- Keyboard menu navigation, Escape/backdrop dismissal, disabled attachment
  behavior, auxiliary focus restoration, and touch targets remain intact.
- Verified with focused render contracts, `go vet ./...`, `go test ./...`,
  JavaScript syntax, static build, workflow validation, diff check, and served
  HTML/asset checks.
