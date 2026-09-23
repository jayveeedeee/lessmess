# MAC-56: Move context compaction into Chat options

## Objective

Place context compaction inside Chat Options and keep its availability state
independent of the Controls view.

## Dependencies

- MAC-55.

## Scope

- Move Compact context from Controls into the Options menu.
- Preserve capability, busy, usage, pending, and confirmation safeguards.
- Remove the unrequested file, reference, and skill entries from Options.

## Implementation steps

1. Place the existing compaction control in Options.
2. Decouple compaction availability from loading the Controls view.
3. Remove the unrequested file, reference, and skill menu entries.
4. Update contracts and run full verification.

## Verification

- Render and JavaScript contracts for placement and existing compaction gates.
- Full tests, vet, JavaScript syntax, build, validation, and diff check.

## Completion criteria

Compact is available from Options without first opening Controls, and Options
contains no file, reference, or skill entries.

## Files affected

- `web/templates/layout.html`
- `web/static/app.js`
- `web/static/app.css`
- `internal/server/render_test.go`

## Notes

- Compact is now an Options menu item.
- It retains capability, busy-session, context-availability, pending-action,
  confirmation, and lifecycle-action gates; its tooltip and accessible label
  expose the current availability reason.
- Fixed the disabled-state bug by sourcing compaction usage from the normal
  background usage poll rather than requiring Controls to be opened first.
- Fixed the click handler to use that same background usage state and surface
  endpoint failures in Chat instead of silently swallowing them.
- Removed the unrequested file, reference, and skill menu entries.
- Verified with focused render contracts, `go vet ./...`, `go test ./...`,
  JavaScript syntax, static build, workflow validation, diff check, and served
  HTML/asset checks.
