# Ledger — 2026-09-18-15tbl

- Change ID: 2026-09-18-15tbl
- Plan: [plan.md](plan.md)
- Branch: —
- Overall status: Planned
- Last updated: 2026-09-18

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
| [JSI-00](tasks/00-json-state-schema.md) | JSON state schema and model types | Not started | — | 2026-09-18 | Foundation; no wiring |
| [JSI-01](tasks/01-md-to-json-migration.md) | md→JSON migration | Not started | JSI-00 | 2026-09-18 | Round-trip verified before deleting md ledgers |
| [JSI-02](tasks/02-store-cutover.md) | Store cutover to JSON state | Not started | JSI-01 | 2026-09-18 | Cutover point; keep gap to JSI-04 short |
| [JSI-03](tasks/03-task-state-api.md) | Deterministic task-state API | Not started | JSI-02 | 2026-09-18 | Rule enforcement moves server-side |
| [JSI-04](tasks/04-instruction-injection.md) | Instruction modules and injection engine | Not started | JSI-02 | 2026-09-18 | Replaces hardcoded prompts; can follow JSI-03 in parallel |
| [JSI-05](tasks/05-board-views-from-json.md) | Board and detail views from JSON | Not started | JSI-02, JSI-03 | 2026-09-18 | Presentation only |
| [JSI-06](tasks/06-agents-shrink-init-docs.md) | AGENTS.md shrink, init rework, docs | Not started | JSI-02, JSI-03, JSI-04 | 2026-09-18 | Final contract migration |

## Decision log

| Date | Decision |
| --- | --- |
| 2026-09-18 | Tracking state moves to committed `.lessmess/workflow/` JSON (gitignore negation); rest of `.lessmess/` stays personal. User-confirmed baseline. |
| 2026-09-18 | Plan/task prose stays markdown under `changes/<id>/`, referenced from JSON; task frontmatter dropped (identity is JSON-only). |
| 2026-09-18 | Workflow instructions become binary-embedded JSON modules injected at spawn, deterministic selection f(session kind, change/task state); static selection v1; no per-repo overrides. |
| 2026-09-18 | Docs system (STRUCTURE.md/AGENTS.md learnings pairs) out of scope except the workflow_agents.md asset swap and natural gardening. |
