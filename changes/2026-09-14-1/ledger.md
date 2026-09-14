# Ledger — 2026-09-14-1

- Change ID: 2026-09-14-1
- Plan: [plan.md](plan.md)
- Branch: —
- Overall status: In progress
- Last updated: 2026-09-14

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
| [DSC-00](tasks/00-empty-state-prime-rule.md) | Add empty-state rule to discussion prompt | Done | — | 2026-09-14 | Prompt matches plan block byte-for-byte; `go vet ./... && go test ./...` green incl. extended assertions and settingswiring/settingschange regression guards |
| [DSC-01](tasks/01-rebuild-and-live-verify.md) | Rebuild and verify fresh opening live | Done | DSC-00 | 2026-09-14 | Rebuilt + restarted (PID 74327 owns :9090). Check A: new discussion replied exactly "What would you like to build?", 0 tool calls, reasoning cited instruction 0. Check B: fresh settings discussion (reused:false, old one bound to Done change) engaged on session.agent with 5 grounded tool calls, no invite stall. Existing sessions unaffected by construction (primes sent only at creation) |
| [DSC-02](tasks/02-keep-terminal-on-scaffold.md) | Keep the terminal open when a discussion scaffolds | Done | — | 2026-09-14 | app.js: index branch skips reload while terminal open, followSession() navigates to /changes/{id}?session= once bound, closeTerminal() re-arms refresh. node --check ok; rebuilt + restarted (PID 80084). Headless chain verified: unassigned binding {"change":""} → scaffold 2026-09-14-2 → binding returns change, board+?session= loads 200; throwaway change cancelled, test session deleted. `go vet ./... && go test ./...` green. Browser click-through awaits user acceptance |

## Decision log

- 2026-09-14 — Fix via the Go base prompt (`discussionPrompt`), not a `prompts.discussion` config tweak; user selected the base-prompt option. Empty-state rule is conditioned on "nothing below it" so the Settings Change flow keeps engaging immediately, and explicitly overrides appended addenda until the user's first message.
- 2026-09-14 — Scope extension (DSC-02): terminal persistence across scaffold folded into this change rather than a new one. Rationale: discovered during this change's acceptance, same new-change-session objective, and this session is bound to this change (the scaffold endpoint 409s bound sessions, so a separate change cannot be created from here). Fix is follow-the-session navigation (`/changes/{id}?session={sid}`) instead of the index full reload; if the user prefers a standalone change later, DSC-02 can be split out.
