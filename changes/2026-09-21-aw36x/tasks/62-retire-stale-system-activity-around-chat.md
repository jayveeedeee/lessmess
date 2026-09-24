# MAC-62: Modernize standalone Chat event rows

## Objective

Remove the old block presentation from standalone Chat events without removing
expandable System activity or its reasoning, tool, and shell history.

## Dependencies

- MAC-61

## Scope

- Preserve every expandable System activity group in the live transcript.
- Mark activity as running only when its latest item is explicitly running.
- Render compaction, explicit system, and unknown top-level events as compact,
  transparent rows instead of legacy blocks.
- Keep pending forms visually dominant without deleting transcript history.

## Implementation steps

1. Carry explicit running state from reasoning, tool, and shell activity.
2. Preserve all activity groups while preventing stale running indicators.
3. Neutralize block chrome for all non-user/non-assistant event messages.
4. Scope global navigation-header styling to the page header so it cannot turn
   semantic transcript headers into sticky blocks.
5. Add regression coverage for prior turns, pending forms, and compaction.

## Verification

- Run focused Chat server tests.
- Run the full Go test and vet suites, JavaScript syntax check, validation, and
  served-page checks.

## Completion criteria

Reasoning, tools, and shell activity remain expandable; stale activity is not
shown as running; and standalone events use a compact transparent row rather
than the old gray block.

## Files affected

- `internal/server/chat.go`
- `internal/server/chat_test.go`
- `web/static/app.css`

## Notes

The event-row renderer covers three categories: compaction, explicit system,
and unknown future top-level message types. Clock time remains excluded from
running-state detection. The black compaction block was caused by the global
`header` selector applying page-navigation dimensions and background to a
semantic header inside the transcript.
