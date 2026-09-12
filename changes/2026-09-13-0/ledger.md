# Ledger — 2026-09-13-0

- Change ID: 2026-09-13-0
- Plan: [plan.md](plan.md)
- Branch: —
- Overall status: Done
- Last updated: 2026-09-13

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
| [CMT-00](tasks/00-git-status-helper-and-endpoint.md) | Read-only git helper and status endpoint | Done | — | 2026-09-13 | `gitStatus` + `GET /api/git/status`; helper/endpoint tests pass (non-repo, dirty, clean) |
| [CMT-01](tasks/01-commit-all-endpoint.md) | Repo-wide commit endpoint and prompt | Done | CMT-00 | 2026-09-13 | `POST /api/git/commit` + reused `commitStatus` route; tests: 201 mapped session, 422 clean, 422 non-repo, 503 no oc |
| [CMT-02](tasks/02-index-button.md) | Index page Commit all button | Done | CMT-00 | 2026-09-13 | Button renders enabled/disabled/hidden per repo state; TestIndexCommitAllButton passes |
| [CMT-05](tasks/05-watcher-chmod-loop.md) | Fix watcher CHMOD loop triggered by git scans | Done | CMT-02 | 2026-09-13 | Watchers now ignore attribute-only CHMOD events; loop dead (0 events per cycle), real writes still notify; TestWatchIgnoresChmod passes |
| [CMT-03](tasks/03-commit-modal.md) | Commit modal markup, CSS, and JS flow | Done | CMT-01, CMT-02 | 2026-09-13 | Live smoke test on :9099: button enabled (repo dirty), modal markup served, /api/git/status returns real porcelain list, commit-status 400 on bad id; node --check app.js OK |
| [CMT-04](tasks/04-verify-and-docs.md) | End-to-end verification and docs updates | Done | CMT-00, CMT-01, CMT-02, CMT-03 | 2026-09-13 | vet+tests+build pass; validate exit 0 (only gardener-owned staleness warnings); README documents the feature |

## Decision log

- 2026-09-13 — Commit mechanism: reuse an opencode session (LLM writes message and commits); server runs git read-only for preview only. Chosen by user over server-side deterministic commit and LLM-draft-then-edit.
- 2026-09-13 — Modal shows porcelain file list + `git diff --stat HEAD`; no full diff.
- 2026-09-13 — Progress UX: spinner in modal + 3 s polling, mirroring the board commit flow.
- 2026-09-13 — Button state: computed at page render, re-fetched on modal open, re-checked on confirm; no live polling/SSE.
- 2026-09-13 — Commit session maps to the unassigned Discussions bucket, titled `repo — git commit`.
- 2026-09-13 — Bug found in verification (recorded as CMT-05): the store and docs watchers treated attribute-only fsnotify `CHMOD` events (atime updates from `gitStatus` reading modified files) as changes, causing a self-sustaining index reload loop on dirty repos. Both watchers now ignore CHMOD-only events; fix verified live (0 events per cycle, real writes still notify).
- 2026-09-13 — Modal content revised on user feedback: two blocks (porcelain list + diffstat) read as duplicated; now one merged list (code + path + per-file `+/-` counts from `--numstat`, untracked without counts) plus a one-line `--shortstat` summary. API field `stat` replaced by per-row `added`/`deleted` + `summary`.
