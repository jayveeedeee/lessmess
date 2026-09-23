# MAC-62: Retire stale System activity around Chat forms

## Objective

Keep stale System activity from lingering in live Chat or competing visually
with permission and multiple-choice interactions.

## Dependencies

- MAC-61

## Scope

- Show only current System progress in the live snapshot while preserving shell
  records as durable command output.
- Hide all System activity while Chat is waiting for a permission or form.
- Preserve settled System activity in paged transcript history.
- Base the decision on OpenCode lifecycle state, not elapsed time.

## Implementation steps

1. Carry explicit running state from reasoning, tool, and shell activity.
2. Filter settled reasoning and tool activity from live snapshots.
3. Suppress even running activity while a user interaction is pending.
4. Add regression coverage for stale turns, pending forms, and history.

## Verification

- Run focused Chat server tests.
- Run the full Go test and vet suites, JavaScript syntax check, validation, and
  served-page checks.

## Completion criteria

Pending forms are never visually competed with by a System block, stale
reasoning and tool activity is absent from the live transcript, shell records
remain readable, and historical activity remains available through
older-message pages.

## Files affected

- `internal/server/chat.go`
- `internal/server/chat_test.go`

## Notes

Clock time is deliberately excluded from stale detection, so day changes,
device clock corrections, and long-running sessions cannot revive old blocks.
