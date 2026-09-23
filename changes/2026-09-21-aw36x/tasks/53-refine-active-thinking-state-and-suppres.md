# MAC-53: Refine active thinking state and suppress chat update notices

## Objective

Make active reasoning visually clear without reviving activity from the
previous turn, and remove routine conversation-update notices.

## Dependencies

- MAC-52.

## Scope

- Render the active thinking/tool summary and spinner in the configured accent.
- Mark activity as running only when it is the newest transcript block.
- Do not show a "Conversation updated" status after polling changes.
- Preserve errors, sending state, lifecycle feedback, and accessibility status.

## Implementation steps

1. Tighten server-side running-activity projection to the latest block.
2. Add an explicit active class and accent styling.
3. Remove the routine polling update notice.
4. Add regression and rendering contracts, then run full verification.

## Verification

- Snapshot regression for a busy session whose newest message is from the user.
- Render/CSS/JavaScript contracts for active accent and suppressed notices.
- Full tests, vet, JavaScript syntax, build, validation, and diff check.

## Completion criteria

Submitting a follow-up never reactivates the prior turn's activity, active
thinking uses the highlight color, and ordinary poll updates are silent.

## Files affected

- `internal/server/chat.go`
- `internal/server/chat_test.go`
- `internal/server/render_test.go`
- `web/templates/partials.html`
- `web/static/app.js`
- `web/static/app.css`

## Notes

- Running activity is projected only when the newest transcript block is an
  activity block; a newer user message leaves prior activity settled.
- Prompt submission records the current user/activity boundary and suppresses
  an unchanged historical spinner until OpenCode publishes either the new user
  message or a genuinely new activity group.
- Active activity summaries, markers, and spinners use `var(--accent)`.
- Routine polling updates no longer display "Conversation updated"; errors and
  explicit operation feedback remain.
- Verified with the stale-activity snapshot regression, focused render
  contracts, `go vet ./...`, `go test ./...`, JavaScript syntax, static build,
  workflow validation, diff check, and served-asset checks.
