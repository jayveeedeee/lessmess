# Ledger — 2026-09-11-0

- Change ID: 2026-09-11-0
- Plan: [plan.md](plan.md)
- Branch: — (repository is not yet under version control)
- Overall status: Done
- Last updated: 2026-09-11

## Status definitions

| Status | Meaning |
| --- | --- |
| Not started | Work has not begun. |
| In progress | Implementation or verification is actively underway. |
| Blocked | Work cannot continue until a documented dependency, decision, approval, or external condition is resolved. |
| Done | All verification and completion criteria in the task file have passed. |
| Cancelled | The task was intentionally removed from scope and the reason is recorded. |

## Tasks

Row order is display and priority order; top row is highest priority.

| Task | Title | Status | Depends on | Updated | Notes |
| --- | --- | --- | --- | --- | --- |
| [CHW-00](tasks/00-update-agents-md.md) | Update AGENTS.md with workflow extensions | Done | — | 2026-09-11 | Verified: grep confirms Root ledger, Archival, Validation, Tooling state sections, both pinned schemas, row-order and single-executor rules present. |
| [CHW-01](tasks/01-create-scaffolding.md) | Create changes/ scaffolding and tooling ignore file | Done | CHW-00 | 2026-09-11 | Verified: changes/ledger.md matches pinned root schema with row for this change; .gitignore contains .tasktracker/. |

## Decision log

- 2026-09-11: Root ledger is change-level only; task status remains solely in per-change ledgers.
- 2026-09-11: Ledger row order encodes display/priority order; no priority fields.
- 2026-09-11: Task frontmatter limited to `id` + `title`.
- 2026-09-11: Archival keeps root-ledger rows with updated links; statuses unchanged.
- 2026-09-11: Task-ID prefix for this change registered as `CHW`.
