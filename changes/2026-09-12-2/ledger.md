# Ledger — 2026-09-12-2

- Change ID: 2026-09-12-2
- Plan: [plan.md](plan.md)
- Branch: main
- Overall status: Done
- Last updated: 2026-09-12

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
| [UIX-00](tasks/00-remove-plain-form.md) | Remove plain new-change form from index template | Done | — | 2026-09-12 | Verified: served index has exactly one form (hx-post=/changes/session); tests green; validate OK; screenshot confirms single orange form. |

## Decision log

- 2026-09-12: Only the opencode-driven creation flow remains in the UI (user decision). The `POST /changes` endpoint is kept for API clients.
- 2026-09-12: Number `-2` allocated instead of `-1`: the dogfood throwaway change `2026-09-12-1` was created and removed during OCI-07, and AGENTS.md rule 5 forbids reusing numbers of removed directories.
- 2026-09-12: Task-ID prefix for this change registered as `UIX`.
