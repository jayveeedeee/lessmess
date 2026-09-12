# Ledger — 2026-09-12-7

- Change ID: 2026-09-12-7
- Plan: [plan.md](plan.md)
- Branch: —
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
| [DOC-00](tasks/00-doc-file-format-model.md) | Doc file format model | Done | — | 2026-09-12 | model/docfile.go + tests; byte-preservation proven; full suite green |
| [DOC-01](tasks/01-coverage-config.md) | Coverage configuration (agentsdocs.json) | Done | — | 2026-09-12 | internal/docs/config.go + repo agentsdocs.json (excludes web/static); TestRepoConfig asserts covered set |
| [DOC-02](tasks/02-structure-generator.md) | Deterministic STRUCTURE.md generator | Done | DOC-00, DOC-01 | 2026-09-12 | walk.go + structure.go; byte-identical idempotence and hash semantics proven by tests |
| [DOC-03](tasks/03-init-command.md) | tasktracker init command | Done | DOC-00, DOC-01 | 2026-09-12 | init.go + embedded workflow asset; manual run: init → validate OK → rerun all-skipped |
| [DOC-04](tasks/04-seed-command.md) | tasktracker docs seed command | Done | DOC-02 | 2026-09-12 | Three-phase seed + snapshot/verify/restore; stub-tested; dry-run verified on this repo |
| [DOC-05](tasks/05-closeout-hook-queue.md) | Close-out hook and serialized refresh queue | Done | DOC-00, DOC-01 | 2026-09-12 | docsqueue.go + touched.go; close→job, stale fallback, refresh reconcile all httptest-covered |
| [DOC-06](tasks/06-gardener-session.md) | Doc gardener session and confinement guard | Done | DOC-05 | 2026-09-12 | docssession.go; skeleton refresh → session → verify/revert → settle; confinement proven by tests |
| [DOC-07](tasks/07-validation-board-surfacing.md) | Validation and board staleness surfacing | Done | DOC-02, DOC-05 | 2026-09-12 | docs.ValidateDocs + /api/validate docs array + amber banner; banner key-mismatch bugfix locked by wire-format test |
| [DOC-08](tasks/08-dogfood-documentation.md) | Dogfood on this repo and documentation | Done | DOC-03, DOC-04, DOC-05, DOC-06, DOC-07 | 2026-09-12 | Seed 13/14 LLM + root hand-seeded; close-out and stale drills passed; two real bugs fixed (abs path, WaitDone); docs merged |

## Decision log

- 2026-09-12 — Design agreed with user in discussion session: two doc files per covered folder (`AGENTS.md` curated + `STRUCTURE.md` machine-owned), close-out hook trigger with stale-flag fallback, unattended auto subagent (no human approval step), committed `agentsdocs.json` coverage config with significant-dirs scope, `init` as full bootstrap with no git assumption, seed designed for mixed repo sizes (token budget + resumable cursor). Task-ID prefix DOC registered in root ledger.
- 2026-09-12 — Subdir rows in STRUCTURE.md always quote the child dir's own purpose (single source of truth); no separate Subdirectories section (DOC-02).
- 2026-09-12 — One `.tasktracker/docs-queue.json` holds pending + stale (not two files): single atomic write (DOC-05).
- 2026-09-12 — Dogfooding: CLI must absolutize `--dir` for opencode sessions; `Client.WaitDone` retries transport timeouts (service /wait blocks past the 30s HTTP cap); seed verification raised to gardener-grade confinement for both doc files; queue state is startup-loaded (one-process-one-repo model, not live-shared); manual reconciliation jobs use a dedicated "(manual)" prompt branch; canonical text must never embed a literal marker pair in prose; root AGENTS.md auto section hand-seeded after three deterministic LLM confinement failures on the 270-line workflow canon.
