# 2026-09-11-0: Bootstrap change-management workflow extensions

- Change ID: 2026-09-11-0
- Created: 2026-09-11
- Branch: — (repository is not yet under version control)
- Status: see [ledger.md](ledger.md)

## Objective and context

Extend the change-management workflow in `AGENTS.md` with the format decisions required for machine processing (a future web kanban server) and faster agent orientation, then create the repository scaffolding to match.

## Current behavior

`AGENTS.md` defines per-change `plan.md`/`ledger.md`/`tasks/` with loose formatting: no fixed table schemas, no task-file skeleton, no change-level index, no archival, validation, or tooling-state rules.

## Target behavior

`AGENTS.md` pins machine-checkable formats; `changes/` contains a root ledger indexing all changes; tooling state is isolated in a gitignored `.tasktracker/` directory.

## Scope

- Amend `AGENTS.md`: root ledger, pinned ledger schemas, task-file frontmatter + skeleton, row-order-as-priority, single-executor rule, archival convention, validation contract, tooling-state location, task-ID prefix registry.
- Create `changes/ledger.md` and `.gitignore`.

## Non-goals

- Implementing the web kanban server or a validator binary (validation rules are specified here, enforced later).
- Initializing git or making commits.

## Design decisions

- Root ledger is change-level only; task status remains solely in per-change ledgers (avoids dual sources of truth).
- Ledger row order encodes display/priority order; no priority fields.
- Task frontmatter is limited to `id` + `title`; status/ordering/dependencies live only in the ledger.
- Archival moves directories to `changes/archive/` but keeps root-ledger rows with updated links.
- Per-change ledgers and the root ledger are single-writer files; a change has one active executor at a time.

## File-level impact

- `AGENTS.md` (modified)
- `changes/ledger.md` (created)
- `.gitignore` (created)
- `changes/2026-09-11-0/` (this change record)

## Testing and verification strategy

- Read back `AGENTS.md` and confirm every new section exists and is internally consistent.
- Confirm scaffolding files match the documented structure and schemas.
- Confirm this change record itself conforms to the new format (dogfooding).

## Risks and mitigations

- Chicken-and-egg: the workflow being defined is used to track its own creation. Mitigation: follow the new format as closely as possible; deviations are recorded in task notes.

## Acceptance criteria

1. `AGENTS.md` contains all agreed amendments, internally consistent.
2. `changes/ledger.md` exists with the pinned root schema and a row for this change.
3. `.gitignore` ignores `.tasktracker/`.
4. This change record follows the new format.

## Tasks

1. [CHW-00: Update AGENTS.md with workflow extensions](tasks/00-update-agents-md.md)
2. [CHW-01: Create changes/ scaffolding and tooling ignore file](tasks/01-create-scaffolding.md)
