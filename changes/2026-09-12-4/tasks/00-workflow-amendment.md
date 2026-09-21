
# LIF-00: AGENTS.md user-only close + reopen amendment

Status: see [../ledger.md](../ledger.md).

## Objective

Amend the status workflow so only the user closes a change (`Done`) and reopens one (`Done` → `In progress`); agents report readiness for close-out instead of closing.

## Dependencies

None.

## Scope

In scope: status workflow rule 8 replacement, reopen note, "Verification and handoff" alignment.
Out of scope: server/UI (other tasks).

## Implementation steps

1. Replace status workflow rule 8: when every non-cancelled task is `Done` and acceptance criteria pass, agents leave the change `In progress` and report it ready for close-out; only the user sets overall `Done` (board Close button or explicit instruction).
2. Add a reopen rule: the user may reopen a closed change (`Done` → `In progress`); agents resume from ledger state.
3. Align the "Verification and handoff" section (report ready-for-close-out instead of marking Done).

## Verification

- Read back AGENTS.md; rules consistent (no remaining instruction for agents to self-close).
- `tasktracker validate` clean.

## Completion criteria

- Workflow text matches the plan's target behavior.

## Files affected

- `AGENTS.md`

## Notes

- None yet.
