# Ledger — 2026-09-15-2

- Change ID: 2026-09-15-2
- Plan: [plan.md](plan.md)
- Branch: —
- Overall status: Done
- Last updated: 2026-09-16

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
| [PSB-00](tasks/00-spike-subagent-session-basics.md) | Spike: subagent child promptability and ID visibility | Done | — | 2026-09-16 | Q1 YES (prompted finished explore child, replied OK under its agent config). Q2 NO (parent model never sees child ID). Title==description confirmed. Evidence in task notes. |
| [PSB-01](tasks/01-mapping-task-fields.md) | Mapping schema: task and parent fields | Done | — | 2026-09-16 | `go vet` clean; full `go test ./...` green. Old-shaped files load without empty keys; new fields round-trip; listByTask verified; client parses parentID. |
| [PSB-05](tasks/05-board-sub-list.md) | Board UI: per-task sub list with talk button | Done | PSB-00, PSB-01, PSB-02, PSB-03 | 2026-09-16 | `go vet` + full suite green; binary rebuilt. Live (scratch repo, port 9099): board HTML serves #board-subs, task-sessions bind 201, sessions payload carries task/parent, served app.js has the chip renderer. Browser-visual check awaits the user's 9090 server restart. |
| [PSB-04](tasks/04-change-prompt-delegation.md) | changePrompt delegation and bind instructions | Done | PSB-00, PSB-02 | 2026-09-16 | `go vet` clean; full suite green; prompt pins extended (Delegation is optional, description prefix, title verbatim). The new prime reaches only sessions created after the 9090 server restart (user step). |
| [PSB-02](tasks/02-bind-endpoint.md) | Bind endpoint for task sessions | Done | PSB-00, PSB-01 | 2026-09-16 | `go vet` clean; full suite green. Guard matrix tested: 201 happy (Parent from live session), 200 idempotent retry, 409 rebind/wrong-change caller/taken sub, 422 unknown task + dead sub, 404 unknown change, 503 no service. |
| [PSB-03](tasks/03-parentid-reconciler.md) | parentID reconciler on session list | Done | PSB-01 | 2026-09-16 | `go vet` clean; full suite green. Children of bound sessions map on list serve: task from title prefix (unknown/absent prefixes map taskless), deduped across serves, foreign parents ignored, service failure fail-open. |

## Decision log

- 2026-09-15: Approach is "B + scan fallback" (user): explicit bind endpoint driven by a prompt convention, plus an automatic parentID reconciler as the safety net; approach C (server-orchestrated per-task sessions) rejected for now.
- 2026-09-15: Full-stack scope for the first change (user): mapping, API, prompt, and board UI in one change.
- 2026-09-15: Subs are stored in `.lessmess/sessions.json` via additive optional `SessionEntry` fields (`task`, `parent`); no new state file; `changes/` documents never record session IDs.
- 2026-09-15: Reconciler triggers on serving the change session list (one `ListSessions` call, fail-open), not on a background watcher.
- 2026-09-15: Spike question 2 (does the parent see the child session ID?) is open; PSB-04's prompt wording is finalized after PSB-00 records the answer.
- 2026-09-15: Delegation policy is "inline first" (user): the main change session does small work itself and spawns a subagent only when a task benefits from it; PSB-04 must not mandate one sub per task. PSB-04 also steers subs toward build-capable agents when delegation may be continued by the user later.
- 2026-09-15: Spike results (PSB-00): direct prompting of finished subagent children works (sub continues under its own agent config) — "talk to a sub" is viable. The parent model never sees the child session ID, so the reconciler is the primary mapper; the prompt teaches `TSK-NN: ` description prefixes (child title == description, verified) instead of a bind curl; the bind endpoint becomes corrective/user-facing with the caller `session` optional.
