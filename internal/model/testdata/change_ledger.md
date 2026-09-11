# Ledger — 2026-09-11-1

- Change ID: 2026-09-11-1
- Plan: [plan.md](plan.md)
- Branch: — (repository is not yet under version control)
- Overall status: In progress
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
| [KAN-00](tasks/00-project-scaffold.md) | Project scaffold and CLI entry | Done | — | 2026-09-11 | Verified: go build + vet clean; serve returned HTTP 200 on :18099; validate stub printed message, exit 2. |
| [KAN-01](tasks/01-ledger-and-task-parsers.md) | Ledger and task-file parsers | In progress | KAN-00 | 2026-09-11 | — |
| [KAN-02](tasks/02-serializers-and-writers.md) | Serializers and atomic writers | Not started | KAN-01 | 2026-09-11 | — |
| [KAN-03](tasks/03-store-watch-validate.md) | Store — scan, cache, watch, validate | Not started | KAN-02 | 2026-09-11 | — |
| [KAN-04](tasks/04-http-api-and-sse.md) | HTTP API and SSE | Not started | KAN-03 | 2026-09-11 | — |
| [KAN-05](tasks/05-board-ui.md) | Board UI | Not started | KAN-04 | 2026-09-11 | — |
| [KAN-06](tasks/06-validate-command.md) | validate command and UI validation banner | Not started | KAN-03 | 2026-09-11 | Can proceed in parallel with KAN-04/KAN-05. |
| [KAN-07](tasks/07-dogfood-and-readme.md) | Dogfood, end-to-end verification, README | Not started | KAN-05, KAN-06 | 2026-09-11 | — |

## Decision log

- 2026-09-11: Frontend is htmx + SortableJS with server-rendered templates; no JS build step; API kept separate for a possible later SPA.
- 2026-09-11: Server consumes the existing `changes/` format; any format gap becomes a new change instead of silent drift.
- 2026-09-11: External deps limited to fsnotify, yaml.v3, goldmark; stdlib HTTP mux.
- 2026-09-11: Atomic writes + freshness checks; writes refused (422) on unparseable ledgers; 409 on persistent external conflict.
- 2026-09-11: Server performs create + update only; never deletes files.
- 2026-09-11: Task-ID prefix for this change registered as `KAN`.
