# Ledger — 2026-09-17-39avm

- Change ID: 2026-09-17-39avm
- Plan: [plan.md](plan.md)
- Branch: —
- Overall status: In progress
- Last updated: 2026-09-17

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
| [HOF-00](tasks/00-spawn-endpoint.md) | spawn-change endpoint, changePrompt session ID + handoff step, SpawnedFrom field | Test | — | 2026-09-17 | Implemented + verified: vet clean, all 6 packages pass, handler matrix green. Awaiting acceptance. |
| [HOF-01](tasks/01-board-ui.md) | Handoffs listing endpoint and board "Spawn new change" action | Test | HOF-00 | 2026-09-17 | Implemented + verified: unit tests green, scratch smoke test passed (board form, listing, 404/422), stray opencode session cleaned up. Awaiting acceptance. |
| [HOF-02](tasks/02-docs-validation.md) | README documentation and full validation pass | Not started | HOF-00, HOF-01 | 2026-09-17 | — |

## Decision log

- 2026-09-17 — Handoff context is a `handoff*.md` artifact inside the source change directory (workflow-legal supporting artifact), referenced by the request rather than carried inline: survives retries, stays with the source change, readable before firing.
- 2026-09-17 — New change B is created and its session bound immediately; the prime instructs distilling the artifact into plan.md + tasks before implementation. The user's approval of the handoff replaces the discussion-phase gate.
- 2026-09-17 — `CreateChange` happens before spawn/prime; the change directory is canonical and never rolled back, while a failed spawn/prime deletes the session (no unbound leaks) and reports B's ID.
- 2026-09-17 — `changePrompt` gains the session's own ID (mirroring `discussionPrompt`) so the bound session can call spawn-change itself; scaffold's 409 guard and prohibition are untouched.
