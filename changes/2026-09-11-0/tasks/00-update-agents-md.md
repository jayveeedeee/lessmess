
# CHW-00: Update AGENTS.md with workflow extensions

Status: see [../ledger.md](../ledger.md).

## Objective

Amend `AGENTS.md` so the change-management workflow is machine-checkable and self-indexing, per the design decisions agreed in discussion and recorded in [../plan.md](../plan.md).

## Dependencies

None.

## Scope

In scope:

- Required-structure tree updated to show root ledger and optional `archive/`.
- New "Root ledger" section with pinned schema and update rules.
- Task-file format rules: frontmatter (`id`, `title`) and fixed heading skeleton.
- Per-change ledger: pinned table schema, `|` constraint, row-order-as-priority rule.
- Task-ID prefix registration rule.
- Single-executor rule under "Dependencies and execution order".
- New "Archival", "Validation", and "Tooling state" sections.

Out of scope: implementing the validator or web server; initializing git.

## Implementation steps

1. Rewrite `AGENTS.md`, preserving existing wording wherever content is unchanged.
2. Insert the root-ledger specification after "Required structure".
3. Add task-file format rules (frontmatter + skeleton) to "Task files".
4. Pin the per-change ledger schema and add the row-ordering rule to "`ledger.md`".
5. Add the single-executor bullet, and the "Archival", "Validation", and "Tooling state" sections.

## Verification

- Read `AGENTS.md` and confirm each new section heading exists.
- Confirm the example schemas render as valid markdown tables.
- Confirm internal consistency (no rule contradicts another; vocabularies match the status table).

## Completion criteria

- All listed sections present and consistent; examples conform to the pinned schemas.

## Files affected

- `AGENTS.md`

## Notes

- None yet.
