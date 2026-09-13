# Ledger — 2026-09-13-2

- Change ID: 2026-09-13-2
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
| [SET-00](tasks/00-settings-storage-and-merge.md) | Settings storage, layering, and merge | Done | — | 2026-09-13 | Settings tests + go vet + full suite pass |
| [SET-09](tasks/09-task-panel-for-bound-terminals.md) | Task panel for change-bound terminals on any page | Done | SET-08 | 2026-09-13 | Binding endpoint live; fragment serves cards; suite green |
| [SET-08](tasks/08-per-setting-change-button.md) | Per-setting Change button with session reuse | Done | SET-04, SET-06 | 2026-09-13 | Fresh/reuse verified live; suite green |
| [SET-07](tasks/07-settings-nav-explorer-style.md) | Settings nav in explorer selection style | Done | SET-06 | 2026-09-13 | Explorer idiom applied; live on :9090 |
| [SET-06](tasks/06-settings-page-grouped-nav.md) | Settings page grouped left navigation | Done | SET-04 | 2026-09-13 | Nav live on :9090; render test + suite green |
| [SET-05](tasks/05-end-to-end-verification.md) | End-to-end verification and docs | Done | SET-04 | 2026-09-13 | Live smoke passed; save-time validation added after service finding |
| [SET-04](tasks/04-settings-page-ui.md) | Settings page UI and client wiring | Done | SET-02, SET-03 | 2026-09-13 | Live smoke on throwaway repo passed; README added |
| [SET-03](tasks/03-settings-http-api.md) | Settings HTTP API | Done | SET-00, SET-01 | 2026-09-13 | Handler tests pass; full suite green |
| [SET-02](tasks/02-server-settings-wiring.md) | Wire settings into server behavior | Done | SET-00, SET-01 | 2026-09-13 | All wiring tests pass; full suite green |
| [SET-01](tasks/01-opencode-session-defaults.md) | opencode client session defaults support | Done | — | 2026-09-13 | Client tests pass; fallback retry moved to server helper (SET-02) |

## Decision log

- 2026-09-13: Layered storage agreed with user: committed `lessmess.json`
  (project policy, shared via git) + gitignored `.lessmess/settings.json`
  (personal override, wins). Settings edits flow through normal commits.
- 2026-09-13: Prompt customization is append-only addendum per prompt; base
  prompts stay immutable so scaffold/binding triggers cannot be broken.
- 2026-09-13: Agent/model applied at session creation via `CreateSessionWith`
  (opencode V2 create body supports `agent` and `model` — verified against
  `/v2/openapi.json`); plain-retry fallback on 400; never retroactive.
- 2026-09-13: Change-directory naming stays contract-fixed
  (`changes/YYYY-MM-DD-N/`) and is not a setting; landing-page setting cut
  (redirect would orphan the change list); no model variants, no per-role
  overrides, no init/validate changes in v1.
- 2026-09-13: Save-time validation added (SET-03): the live opencode
  service accepts unknown agent/model values at session creation without
  an error and the session then never runs, so the 400-retry fallback
  cannot fire for typos. `PUT /api/settings` validates submitted values
  against the live, repo-location-scoped agent/model lists (422 on
  unknown; skipped when the service is unreachable). The spawn-time 400
  fallback remains as a safety net.
