# Ledger — 2026-09-12-12

- Change ID: 2026-09-12-12
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
| [NAV-00](tasks/00-top-menu-active-state.md) | Top menu with active route highlight | Done | — | 2026-09-12 | Screenshot-verified: Changes active on / and board, Explorer active on /explorer; vet/test/build green; root-ledger status synced. |
| [NAV-01](tasks/01-flat-bell-icon.md) | Flat SVG bell icon | Done | — | 2026-09-12 | Stroke-SVG bell + repositioned badge screenshot-verified in dark and light; hidden semantics preserved via `#notif-bell[hidden]`; vet/test green. |
| [NAV-02](tasks/02-newest-change-first.md) | Newest change at top of index | Done | — | 2026-09-12 | `changeIDLess` (lexicographic date + numeric counter); test covers same-date 0/2/10; live JSON shows 2026-09-12-13 on top. |
| [NAV-03](tasks/03-continue-session-button.md) | Prominent Continue/Start session button | Done | — | 2026-09-12 | Screenshots: Continue on has-session board, Start on no-session board; Node DOM harness: 12/12 assertions (resume last-opened, fallback to newest, create+open). |
| [NAV-05](tasks/05-remove-header-crumb.md) | Remove obsolete header crumb | Done | NAV-00 | 2026-09-12 | Span + CSS rule removed; vet/test/validate green; :9090 rebuilt+restarted; screenshot confirms clean header on board page. |
| [NAV-04](tasks/04-verify-and-docs.md) | End-to-end verification and README update | Done | NAV-00, NAV-01, NAV-02, NAV-03 | 2026-09-12 | vet/test/build/validate green; README updated; final live walk + index screenshot confirm all acceptance criteria. Ready for user close. |

## Decision log

- 2026-09-12 — Active nav = orange text + 2px underline; Continue button always visible and dual-mode (Continue session / Start session); "last session" = most recently opened, tracked in localStorage per change with fallback to newest created; index sorted by change ID descending. All four from user discussion before scaffold.
