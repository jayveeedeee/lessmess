# Ledger — 2026-09-13-3

- Change ID: 2026-09-13-3
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
| [DQP-00](tasks/00-write-project-addendum.md) | Write discussion question policy to project settings | Done | — | 2026-09-13 | Verified: PUT 200, read-back byte-identical, source=project, lessmess.json correct, vet/test clean. |

## Notes

- Implemented and verified 2026-09-13 (DQP-00 → Test). Evidence: `PUT /api/settings?scope=project` returned 200; `GET /api/settings` reports `sources["prompts.discussion"] = "project"` with the value byte-identical to the plan's authoritative block (626 chars); root `lessmess.json` gained `prompts.discussion` with `session.model` and the empty `git`/`ui`/`docs` sections preserved; `go vet ./...` clean and `go test ./...` passing; the `/settings` page has the `prompts.discussion` field, populated client-side from the verified API (`app.js` fetches `/api/settings`).
- Remaining: user acceptance — the next newly created discussion session should open with no questions and ask only multiple-choice questions.
- Plan and task files are complete; the settings write in DQP-00 is done.

## Decision log

- 2026-09-13 — Project layer chosen over personal: matches the Settings-page scope where the discussion was opened, and a committed `lessmess.json` makes the policy govern every discussion in this checkout.
- 2026-09-13 — Addendum wording opens by explicitly overriding base-prompt step 1 ("Ask questions"), mandates stated assumptions instead of stalls, and requires multiple-choice-only questions (never open-ended).
- 2026-09-13 — Configuration-only change: no Go/template/static edits and no README update; existing `settingswiring_test.go` already covers addendum application to discussion sessions.
