# Ledger — 2026-09-17-4esfh

- Change ID: 2026-09-17-4esfh
- Plan: [plan.md](plan.md)
- Branch: —
- Overall status: Done
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
| [PROJ-00](tasks/00-general-section-plumbing.md) | General settings section and project-name plumbing | Done | — | 2026-09-17 | settingsgeneral_test.go: fallback, layering, patch rejection, allowlist; full `go test ./...` green |
| [PROJ-01](tasks/01-project-name-chrome.md) | Project name in header, terminal head, and tab title | Done | PROJ-00 | 2026-09-17 | projectnamechrome_test.go: title+2 spans on 5 pages, fallback, escaping; live-verified via scratch server |
| [PROJ-02](tasks/02-settings-general-ui.md) | Settings General project-name field | Done | PROJ-00 | 2026-09-17 | Field + save-reload wired; PUT verified live (project+personal, badge source); interactive pass awaits user |
| [PROJ-03](tasks/03-onboarding-name-step.md) | Onboarding wizard Project name step | Done | PROJ-00 | 2026-09-17 | Step, prefill, save/skip, finish summary wired; step markup live-verified on /setup; click-through awaits user |

## Decision log

- 2026-09-17 — Verified end-to-end on a scratch repo (`lessmess serve --port 9191`): unset name
  renders the directory basename in title + both brand spans; PUT project "Atlas" and personal
  "Mine" layer correctly (title follows personal, `defaultProjectName` exposes "repo",
  badge source personal); `lessmess.json` gains the general section cleanly.

- 2026-09-17 — Tab title is the project name only, identical on every page (user choice).
- 2026-09-17 — Onboarding collects the name in a dedicated step after Prerequisites (user choice);
  scope radios default to Project because the name is shared repo branding, unlike the
  personal-defaulted agent step.
- 2026-09-17 — Explorer `<h1>` keeps the raw directory basename (user choice).
- 2026-09-17 — Default name is computed per read (`filepath.Base(repoDir)`), never persisted, so
  folder renames propagate until the user overrides the setting.
