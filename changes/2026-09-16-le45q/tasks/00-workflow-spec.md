---
id: NTD-00
title: Extend workflow spec
---

# NTD-00: Extend workflow spec

Status: see [../ledger.md](../ledger.md).

## Objective

Encode nested task decomposition into the authoritative workflow text (AGENTS.md) and regenerate the embedded copy so the drift test stays green, pinning the contract the code will implement.

## Dependencies

— (first task; everything else implements this spec)

## Scope

- AGENTS.md sections: task files (containers, dotted IDs, naming), ledgers (container ledger format, governing-ledger rule, root-ledger wording), validation rules 2/3 (recursive container checks), status workflow / close-out readiness (recursive), decomposition governance (user-instructed only; agents propose).
- Regenerate `internal/docs/assets/workflow_agents.md` with the documented `awk` command; drift test must pass unchanged in mechanism.

## Implementation steps

1. Draft the new task-structure rules: `NN-slug.md` + sibling `NN-slug/` container holding `ledger.md` + `tasks/`; exact name match; loose artifacts allowed; no per-container plan.md.
2. Specify dotted IDs (`PREFIX-NN`, children `PREFIX-NN.MM` from `00` per level) and href nesting.
3. Specify the container ledger: minimal headers (`- Task: <id> (change <change-id>)`, `- Last updated:`), same pinned task table, same status vocabulary, row-order-as-priority per level.
4. Extend validation rules 2 and 3 recursively; state the close-out readiness recursion and the display-only rollup rule (no tooling writes statuses on rollup's behalf).
5. Write the governance rule: decomposition is user-instructed only (board action, explicit instruction, or approved plan); agents propose, never act.
6. Reword the root-ledger line "Task statuses live exclusively in each change's ledger.md" to the governing-ledger formulation.
7. Regenerate the embedded copy: `awk '/^<!-- tasktracker:begin/{exit} {print}' AGENTS.md > internal/docs/assets/workflow_agents.md`.
8. Run the drift test.

## Verification

- `go test ./internal/docs/` green (drift test).
- `lessmess validate` still clean on this repository's own tree (spec change adds no new violations for existing flat changes).
- Manual read-through: the spec matches the design decisions recorded in [../plan.md](../plan.md).

## Completion criteria

AGENTS.md and the embedded copy describe the nested structure, IDs, container ledgers, recursive validation, recursive close-out, display-only rollup, and user-instructed governance; drift test green.

## Files affected

- `AGENTS.md`
- `internal/docs/assets/workflow_agents.md`

## Notes

The spec is the contract for NTD-01…NTD-07; any later deviation must come back through this task or a scope note in the ledger decision log.
