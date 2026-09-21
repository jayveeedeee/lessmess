# MAC-34: Remove redundant Assistant transcript headers

## Objective

Remove the redundant Assistant heading from transcript response rows.

## Dependencies

- MAC-33.

## Scope

- Do not render a header for assistant messages.
- Retain labels for user, system, compaction, and other distinct message types.
- Preserve assistant text, reasoning, tools, errors, and diffs.

## Implementation steps

1. Make the transcript message heading conditional by message type.
2. Add a render contract preventing the Assistant heading from returning.

## Verification

- Focused and full Go tests, vet, JavaScript syntax, build, validation, and
  diff check.

## Completion criteria

Assistant responses begin directly with their text or activity lines.

## Files affected

- `web/templates/partials.html`
- `internal/server/chat_test.go`

## Notes

The semantic article and message metadata remain unchanged.
