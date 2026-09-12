# Ledger — 2026-09-13-1

- Change ID: 2026-09-13-1
- Plan: [plan.md](plan.md)
- Branch: —
- Overall status: In progress
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
| [TUI-00](tasks/00-merged-tui-config-generator.md) | Merged TUI cli.json generator | Done | — | 2026-09-13 | Accepted by user 2026-09-13; `go vet` + full suite green, 7 tests cover merge/precedence/JSONC/fallback/untouched-user-file |
| [TUI-01](tasks/01-spawn-env-injection-and-wiring.md) | Spawn env injection and terminalWS wiring | Done | TUI-00 | 2026-09-13 | Accepted by user 2026-09-13; WS tests prove injected XDG + fail-open fallback; `debug config` byte-identical with/without override |
| [TUI-02](tasks/02-docs-and-manual-verification.md) | Docs and manual verification | Done | TUI-01 | 2026-09-13 | Accepted by user 2026-09-13 after browser pass (no tabs/sidebar, theme intact, standalone unaffected); live probe evidence + README bullet recorded in task notes |
| [TUI-03](tasks/03-task-panel-layout-and-styling.md) | Task panel layout and styling | Done | — | 2026-09-13 | Accepted by user 2026-09-13 after three styling iterations (full-bleed status bands, pointer cursor, fixed Plan footer) |
| [TUI-04](tasks/04-task-panel-client-mirror.md) | Task panel client mirror and sync | Done | TUI-03 | 2026-09-13 | Accepted by user 2026-09-13; mirror rides SSE board swaps, click-to-detail and Plan anchor verified live, Escape closes top-most layer |
| [TUI-05](tasks/05-panel-docs-and-manual-verification.md) | Panel docs and manual verification | Done | TUI-04 | 2026-09-13 | Accepted by user 2026-09-13 ("I have tested"); README "Task panel" bullet added; suite + validate green |

## Decision log

- 2026-09-13: Approach chosen with user — lessmess manages a generated CLI config for embedded PTYs (`XDG_CONFIG_HOME` → `.lessmess/xdg/`) merging `tabs.enabled=false` + `session.sidebar="hide"` over the user's own `cli.json`; the user's real config is never written; fail-open fallback to today's spawn on any generation error. Alternatives rejected: editing the user's global `cli.json` (changes their standalone TUI too), `opencode2 mini` (bigger UX change than asked), runtime key injection (fragile; tab strip has no runtime toggle). Field names verified against `https://opencode.ai/v2/cli.json`.
- 2026-09-13: Scope broadened — the task-panel work could not scaffold its own change (this session is bound to 2026-09-13-1 per the BSB binding), and the user chose to fold it in; change retitled "Embedded terminal improvements" and TUI-03…TUI-05 added. Panel design chosen with user: grouped by status (board column order), click opens task detail (`#detail` z-index raised above the overlay), always-on (no collapse toggle), ~20% width with min-width floor; data mirrored client-side from the board DOM riding the existing SSE/refreshBoard flow — zero new server surface.
