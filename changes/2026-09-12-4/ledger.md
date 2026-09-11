# Ledger — 2026-09-12-4

- Change ID: 2026-09-12-4
- Plan: [plan.md](plan.md)
- Branch: main
- Overall status: In progress
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
| [LIF-00](tasks/00-workflow-amendment.md) | AGENTS.md user-only close + reopen amendment | Done | — | 2026-09-12 | Verified: rule 8 replaced (user-only close), rule 9 added (reopen), handoff aligned; validate OK. |
| [LIF-01](tasks/01-status-endpoints.md) | Store status transitions + close/reopen endpoints | Done | LIF-00 | 2026-09-12 | Verified: SetChangeStatus writes both ledgers, revalidates clean, reopen round trip, 404 unknown; endpoint tests pass. |
| [LIF-02](tasks/02-commit-endpoint.md) | Commit-session endpoint | Done | LIF-00 | 2026-09-12 | Verified: endpoint creates+primes+maps commit session (prompt content asserted), 503 without service; tests pass. |
| [LIF-03](tasks/03-lifecycle-buttons.md) | Board lifecycle buttons | Done | LIF-01, LIF-02 | 2026-09-12 | Verified: render tests for both button states; screenshots show Close change (In progress) and Reopen (Done); app.js handlers wired. |
| [LIF-04](tasks/04-dogfood.md) | Dogfood | Done | LIF-03 | 2026-09-12 | All criteria verified incl. live agent-authored commit (82b0c1f) in throwaway repo; cleanup done; validate OK. |
| [LIF-05](tasks/05-content-hashed-assets.md) | Content-hashed asset versioning (fix stale JS) | Done | — | 2026-09-12 | Verified: hashed URLs served (?v=32a39467), fresh script has commit handler, unit test passes. |

## Decision log

- 2026-09-12: Only the user closes/reopens a change (user); agents report "ready for close-out" instead of setting `Done` — AGENTS.md amended accordingly.
- 2026-09-12: Commit flow delegates git to the primed agent session (watched live in the terminal); tasktracker itself never runs git mutations, keeping its no-git-mutations posture.
- 2026-09-12: Close is allowed with incomplete tasks but carries a UI confirm; the user is the authority.
- 2026-09-12: Task-ID prefix for this change registered as `LIF`.
- 2026-09-12: LIF-05 added (user report): Commit button inert because app.js was served immutable under an un-bumped manual `?v=`. Fix: content-hashed asset URLs computed at startup — manual bumps eliminated.
