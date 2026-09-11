package model

import "fmt"

// RenderTaskFile renders a task file matching the AGENTS.md skeleton.
func RenderTaskFile(id, title string) []byte {
	return []byte(fmt.Sprintf(`---
id: %s
title: %s
---

# %s: %s

Status: see [../ledger.md](../ledger.md).

## Objective

## Dependencies

## Scope

## Implementation steps

## Verification

## Completion criteria

## Files affected

## Notes
`, id, title, id, title))
}

// RenderChangePlan renders a minimal plan.md for a new change.
func RenderChangePlan(id, title, date string) []byte {
	return []byte(fmt.Sprintf(`# %s: %s

- Change ID: %s
- Created: %s
- Branch: —
- Status: see [ledger.md](ledger.md)

## Objective and context

## Current behavior

## Target behavior

## Scope

## Non-goals

## Design decisions

## Acceptance criteria

## Tasks

1. (add task files under tasks/ and link them here)
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
| Done | All verification and completion criteria in the task file have passed. |
| Cancelled | The task was intentionally removed from scope and the reason is recorded. |

## Tasks

Row order is display and priority order; top row is highest priority.

| Task | Title | Status | Depends on | Updated | Notes |
| --- | --- | --- | --- | --- | --- |

## Decision log
`, id, id, date))
}
