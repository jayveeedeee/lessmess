# MAC-37: Hide synthetic OpenCode messages from Chat

## Objective

Hide OpenCode's internal synthetic messages from the visible Chat transcript.

## Dependencies

- MAC-36.

## Scope

- Filter `synthetic` messages while building transcript blocks.
- Leave inbox delivery and lifecycle parsing unchanged.
- Ensure synthetic messages do not split otherwise consecutive System activity.

## Implementation steps

1. Skip synthetic messages in the transcript block builder.
2. Add grouping and render regression coverage.

## Verification

- Focused and full Go tests, vet, JavaScript syntax, build, validation, and
  diff check.

## Completion criteria

No visible transcript block or label is produced for a synthetic message.

## Files affected

- `internal/server/chat.go`
- `internal/server/chat_test.go`

## Notes

Synthetic inbox items remain available to delivery controls when applicable.
