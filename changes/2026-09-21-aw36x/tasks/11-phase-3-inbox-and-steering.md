# MAC-11: Phase 3 inbox and steering

## Objective

Allow useful follow-ups while an agent is busy through OpenCode's inbox instead of forcing interruption.

## Dependencies

- MAC-10

## Scope

- List queued inbox items and their delivery mode.
- Queue or steer a follow-up, update/cancel pending input, and show delivery state.

## Implementation steps

1. Add inbox list/create/update/cancel adapters with explicit delivery enums.
2. Let the composer choose immediate steering versus queued delivery when busy.
3. Render pending items separately from committed transcript messages.
4. Test ordering, cancellation races, reconnect, duplicate submit, and idle transitions.

## Verification

- Fake-service contract tests and real busy-session queue/steer smoke checks.

## Completion criteria

A phone user can direct ongoing work without losing messages or unnecessarily stopping the agent.

## Files affected

- `internal/opencode/`
- Chat handlers/templates/static assets
- Focused tests

## Notes

Use OpenCode's delivery semantics directly; do not create a second lessmess queue.
