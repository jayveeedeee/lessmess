# MAC-47: Restrict message actions to user messages

## Objective

Show the contextual Copy/Fork/Revert menu only on the user's messages.

## Dependencies

- MAC-46.

## Scope

- Keep user messages keyboard- and pointer-activatable.
- Remove action targets and menus from assistant and non-conversation activity.
- Preserve the existing copy, fork, and staged-revert behavior for user messages.

## Implementation steps

1. Restrict action attributes and menu markup to `user` messages.
2. Add rendering assertions that assistant messages have no lifecycle menu.

## Verification

- Focused/full tests, vet, JavaScript syntax, build, validation, and diff check.

## Completion criteria

Selecting an assistant, system, tool, reasoning, or shell entry does not open
Copy/Fork/Revert; selecting a user message does.

## Files affected

- `web/templates/partials.html`
- `internal/server/chat_test.go`

## Notes
