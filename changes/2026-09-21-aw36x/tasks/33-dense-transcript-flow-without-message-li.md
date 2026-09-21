# MAC-33: Dense transcript flow without message lifecycle actions

## Objective

Make routine transcript activity read as a dense flow instead of a sequence of
large message and action rows.

## Dependencies

- MAC-32.

## Scope

- Remove the per-message Fork and Preview revert controls from the transcript.
- Reduce message, header, and collapsed activity spacing.
- Suppress the redundant outer heading for shell activity.
- Preserve expanded tool, diff, reasoning, and shell details.

## Implementation steps

1. Simplify message headers to labels only.
2. Tighten passive message and activity-line CSS.
3. Update contracts that previously required lifecycle buttons on every row.

## Verification

- Focused Chat tests, full Go tests/vet, JavaScript syntax, build, validation,
  and diff check.

## Completion criteria

Collapsed activity occupies approximately one normal text line and transcript
rows no longer contain Fork or Preview revert buttons.

## Files affected

- `web/templates/partials.html`
- `web/static/app.css`
- `internal/server/chat_test.go`

## Notes

This intentionally removes transcript entry points for fork/revert at the
user's request; it does not alter the server endpoints.
