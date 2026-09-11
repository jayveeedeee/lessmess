# Ledger — 2026-09-11-1

- Change ID: 2026-09-11-1
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
| [KAN-00](tasks/00-project-scaffold.md) | Project scaffold and CLI entry | Done | — | 2026-09-11 | Verified: go build + vet clean; serve returned HTTP 200 on :18099; validate stub printed message, exit 2. |
| [KAN-01](tasks/01-ledger-and-task-parsers.md) | Ledger and task-file parsers | Done | KAN-00 | 2026-09-11 | Verified: go test ./internal/model passes (11 tests incl. malformation classes); test fixture expectations corrected. |
| [KAN-02](tasks/02-serializers-and-writers.md) | Serializers and atomic writers | Done | KAN-01 | 2026-09-11 | Verified: go test passes — round-trip, byte-preservation, move semantics, templates, atomic write. |
| [KAN-03](tasks/03-store-watch-validate.md) | Store — scan, cache, watch, validate | Done | KAN-02 | 2026-09-11 | Verified: 14 tests pass incl. real fsnotify round trip, external-edit preservation, gap-rule allocation. |
| [KAN-04](tasks/04-http-api-and-sse.md) | HTTP API and SSE | Done | KAN-03 | 2026-09-11 | Verified: 11 handler tests pass (routes, error mapping 404/400/422, SSE delivery); smoke run against real repo served index/board/task/validate correctly. |
| [KAN-06](tasks/06-validate-command.md) | validate command and UI validation banner | Done | KAN-03 | 2026-09-11 | Verified: validate exits 0/OK on real repo; broken fixture printed per-rule violations, exit 1; serve startup logs violations; banner wired to /api/validate + SSE. |
| [KAN-05](tasks/05-board-ui.md) | Board UI | Done | KAN-04 | 2026-09-11 | Verified: render tests, embedded-asset smoke, live move rewrite; user confirmed the interactive click-through (hover, modal, drag region, forms) on 2026-09-11. |
| [KAN-07](tasks/07-dogfood-and-readme.md) | Dogfood, end-to-end verification, README | Done | KAN-05, KAN-06 | 2026-09-11 | All plan acceptance criteria confirmed: 1–8 verified (env evidence + user browser confirmation 2026-09-11); README written; validate clean. |
| [KAN-08](tasks/08-ui-polish.md) | UI polish — cleaner board and task detail | Done | KAN-05 | 2026-09-11 | User approved the final design on 2026-09-11 ("looks good") after hover-lift fix, square corners, full-height columns, reference-matched palette, slim card font. |
| [KAN-09](tasks/09-show-plan-in-ui.md) | Show plan content in the UI | Done | KAN-08 | 2026-09-11 | Verified: route tests (JSON/HTML/404) pass; screenshots confirm Plan buttons on index rows + board header and the plan modal rendering. |

## Decision log

- 2026-09-11: Frontend is htmx + SortableJS with server-rendered templates; no JS build step; API kept separate for a possible later SPA.
- 2026-09-11: Server consumes the existing `changes/` format; any format gap becomes a new change instead of silent drift.
- 2026-09-11: External deps limited to fsnotify, yaml.v3, goldmark; stdlib HTTP mux.
- 2026-09-11: Atomic writes + freshness checks; writes refused (422) on unparseable ledgers; 409 on persistent external conflict.
- 2026-09-11: Server performs create + update only; never deletes files.
- 2026-09-11: Task-ID prefix for this change registered as `KAN`.
- 2026-09-11: Simplified write-conflict design (KAN-03): every write is read-modify-write against fresh on-disk content, so external edits are preserved by construction instead of detected via freshness checks. Conflicts surface as 404 (row vanished) or 422 (file unparseable); no 409 path needed.
- 2026-09-11: First validator dogfood (KAN-04 smoke test): caught root-ledger drift — this change's root row still said `Planned` while its ledger said `In progress`. Fixed the row. Evidence the seven-rule contract works.
- 2026-09-11: KAN-08 added during KAN-05 verification (user feedback on visual quality). Design: dual theme (refined dark default + clean light) with header toggle persisted to localStorage; task detail as centered modal.
