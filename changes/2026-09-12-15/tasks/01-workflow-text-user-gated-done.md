---
id: TST-01
title: Workflow text — Test vocabulary and user-gated Done
---

# TST-01: Workflow text — Test vocabulary and user-gated Done

Status: see [../ledger.md](../ledger.md).

## Objective

Encode the new gate in the canonical workflow text: agents stop at `Test`;
`Done` happens only by the user's hand or explicit instruction; close-out
readiness becomes "all non-cancelled tasks `Test` or `Done`".

## Dependencies

— (none; independent of the code changes, though both must land before
verification in TST-03)

## Scope

- Root `AGENTS.md` (above the `tasktracker:begin` marker only).
- `internal/docs/assets/workflow_agents.md` — must end up byte-identical to
  the root file's curated portion (drift test in `internal/docs/init_test.go`).

## Implementation steps

1. In the task-status vocabulary table ("Use these task statuses exactly"),
   add `Test` between `Blocked` and `Done`:
   `| Test | Implementation and verification are complete; awaiting user acceptance before Done. |`
2. Status workflow step 6: replace "mark a task `Done` after verification"
   with: mark it `Test` once verification and completion criteria pass, and
   record the evidence. Add the rule that an agent sets `Done` **only on
   explicit user instruction**, while the user may move tasks to `Done`
   manually at any time (mirroring the user-closes-change rule in step 8).
3. Step 7 stays conceptually (failed verification ⇒ stay `In progress` or
   `Blocked`), retargeted to `Test` where it referenced `Done`.
4. "Verification and handoff": readiness for close-out is now "every
   non-cancelled task is `Test` or `Done`"; the handoff list tells the agent
   to leave tasks at `Test`, report evidence, and let the user promote to
   `Done` and close the change. Update the matching sentence that today says
   tasks must be `Done`.
5. Sync the embedded copy:
   `awk '/^<!-- tasktracker:begin/{exit} {print}' AGENTS.md > internal/docs/assets/workflow_agents.md`
   and diff to confirm only the intended lines changed.

## Verification

1. `go test ./internal/docs` passes (drift test green).
2. Read the rendered root `AGENTS.md`: vocabulary, workflow steps 6–7, and
   handoff rules are internally consistent (no leftover instruction telling
   agents to self-mark `Done`).

## Completion criteria

- Both pinned files agree and state: `Test` vocabulary, agent-stops-at-`Test`,
  user-gated `Done`, and `Test`-or-`Done` readiness; drift test passes.

## Files affected

- `AGENTS.md`
- `internal/docs/assets/workflow_agents.md`

## Notes

- Existing change ledgers (including this change's own) keep their scaffolded
  five-row definitions tables; the workflow text is authoritative and applies
  going forward. Do not rewrite historical ledgers.
- 2026-09-13: edits applied to vocabulary, status workflow steps 6–9
  (renumbered to 12), and the handoff section; embedded copy re-synced with
  the documented awk command, `diff` confirms byte-identical, and
  `go test ./internal/docs` (drift test) is green.
