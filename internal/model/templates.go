package model

import "fmt"

// RenderTaskFile renders a task prose file matching the AGENTS.md
// skeleton. Identity (id, title) lives in the JSON state; the file is
// prose only, kept human-readable with the heading carrying the ID.
func RenderTaskFile(id, title string) []byte {
	return []byte(fmt.Sprintf(`# %s: %s

## Objective

## Dependencies

## Scope

## Implementation steps

## Verification

## Completion criteria

## Files affected

## Notes
`, id, title))
}

// RenderChangePlan renders a minimal plan.md for a new change.
func RenderChangePlan(id, title, date string) []byte {
	return []byte(fmt.Sprintf(`# %s: %s

- Change ID: %s
- Created: %s
- Branch: —
- Status: tracked in the tool-owned JSON state (.lessmess/workflow/)

## Objective and context

## Current behavior

## Target behavior

## Scope

## Non-goals

## Design decisions

## Acceptance criteria

## Tasks

1. (task breakdown is maintained by the tool; see the board)
`, id, title, id, date))
}

// RenderChangeLedger renders a minimal ledger.md for a new change with an
// empty task table in the pinned schema.
func RenderChangeLedger(id, date string) []byte {
	return []byte(fmt.Sprintf(`# Ledger — %s

- Change ID: %s
- Plan: [plan.md](plan.md)
- Branch: —
- Overall status: Planned
- Last updated: %s

## Status definitions

| Status | Meaning |
| --- | --- |
| Not started | Work has not begun. |
| In progress | Implementation or verification is actively underway. |
| Blocked | Work cannot continue until a documented dependency, decision, approval, or external condition is resolved. |
| Test | Implementation and verification are complete; awaiting user acceptance before Done. |
| Done | All verification and completion criteria in the task file have passed. |
| Cancelled | The task was intentionally removed from scope and the reason is recorded. |

## Tasks

Row order is display and priority order; top row is highest priority.

| Task | Title | Status | Depends on | Updated | Notes |
| --- | --- | --- | --- | --- | --- |

## Decision log
`, id, id, date))
}

// RenderTaskLedger renders a minimal ledger.md for a decomposed task's
// container (tasks/<NN-slug>/ledger.md): the minimal header set from
// AGENTS.md's decomposition rules and the same pinned task table as a
// change ledger.
func RenderTaskLedger(taskID, changeID, date string) []byte {
	return []byte(fmt.Sprintf(`# Ledger — %[1]s

- Task: %[1]s (change %[2]s)
- Last updated: %[3]s

## Status definitions

| Status | Meaning |
| --- | --- |
| Not started | Work has not begun. |
| In progress | Implementation or verification is actively underway. |
| Blocked | Work cannot continue until a documented dependency, decision, approval, or external condition is resolved. |
| Test | Implementation and verification are complete; awaiting user acceptance before Done. |
| Done | All verification and completion criteria in the task file have passed. |
| Cancelled | The task was intentionally removed from scope and the reason is recorded. |

## Tasks

Row order is display and priority order; top row is highest priority.

| Task | Title | Status | Depends on | Updated | Notes |
| --- | --- | --- | --- | --- | --- |
`, taskID, changeID, date))
}
