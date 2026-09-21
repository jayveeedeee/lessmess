# MAC-07: Phase 2 session controls

## Objective

Expose the common controls that change how an active OpenCode session works.

## Dependencies

- MAC-06

## Scope

- Agent/model switching using project-scoped options.
- Command and skill discovery and invocation.
- Context/token usage, retry/provider errors, and compaction warnings.

## Implementation steps

1. Add narrow client and server mutations for agent/model, commands, and skills.
2. Reuse existing settings option semantics and model references.
3. Add compact searchable mobile pickers and clear active selections.
4. Surface context pressure and actionable provider/retry states without exposing credentials.
5. Test unavailable options, API races, invalid selections, and service fallback behavior.

## Verification

- Focused API/UI tests and real-service switching plus command/skill smoke checks.

## Completion criteria

A mobile user can select the agent/model and invoke normal commands or skills with visible success/failure state.

## Files affected

- `internal/opencode/`
- `internal/server/chat.go`
- `web/templates/`
- `web/static/`

## Notes

Use simple pickers, not a recreation of the TUI command palette.
