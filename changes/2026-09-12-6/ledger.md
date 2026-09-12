# Ledger — 2026-09-12-6

- Change ID: 2026-09-12-6
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
| [SCF-00](tasks/00-unassigned-bucket.md) | Unassigned bucket in mapping + discussions list endpoint | Done | — | 2026-09-12 | Verified: bucket add/move/persist/idempotency + enriched list endpoint; tests pass. |
| [SCF-01](tasks/01-discussion-endpoint.md) | Discussion-session endpoint and prime prompt | Done | SCF-00 | 2026-09-12 | Verified: endpoint creates unassigned discussion; prompt assertions (discussion-only, injected curl + session ID, AGENTS.md); 503 path. |
| [SCF-02](tasks/02-scaffold-endpoint.md) | Scaffold trigger endpoint | Done | SCF-00, SCF-01 | 2026-09-12 | Verified: full effect (change+root row+rename+mapping move), validation matrix, idempotent direct-link, validate clean post-scaffold. |
| [SCF-03](tasks/03-discussions-ui.md) | Index discussions UI | Done | SCF-01 | 2026-09-12 | Verified: Discussions section renders live entries (screenshot), Open/unlink wired, overlay shared from layout, xterm on all pages. |
| [SCF-04](tasks/04-cleanup-tests.md) | Remove placeholder machinery, rewrite tests | Done | SCF-01, SCF-02 | 2026-09-12 | Code deletion absorbed into the SCF-01 rewrite; old-flow tests fully replaced (see SCF-01/02 notes). |
| [SCF-05](tasks/05-dogfood.md) | Dogfood full discussion flow | Done | SCF-02, SCF-03, SCF-04 | 2026-09-12 | All five criteria verified live (see task notes); cleanup complete; validate OK. |

## Decision log

- 2026-09-12: Discussion-first model replaces immediate-scaffold flow (user): button creates a session only; the change is scaffolded later via API trigger.
- 2026-09-12: Agent fires the scaffold trigger only with explicit user approval (user).
- 2026-09-12: Title/prefix are chosen in the discussion and passed in the scaffold call; the placeholder mechanism from 2026-09-12-3 is removed entirely (user).
- 2026-09-12: Server renames the session at scaffold time; unassigned sessions tracked in an `_unassigned` mapping bucket and listed on the index (user).
- 2026-09-12: Task-ID prefix for this change registered as `SCF`.
