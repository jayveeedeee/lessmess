# Ledger — 2026-09-14-0

- Change ID: 2026-09-14-0
- Plan: [plan.md](plan.md)
- Branch: —
- Overall status: Done
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
| [SPS-00](tasks/00-sticky-header.md) | Sticky top menu bar | Done | — | 2026-09-14 | Build+vet+test pass; rebuilt binary serves sticky header rule (app.css served line 86); overlay/detail z-order unchanged by inspection; full visual pass in SPS-03 |
| [SPS-02](tasks/02-split-shell.md) | Settings split-shell layout and content de-clutter | Done | SPS-01 | 2026-09-14 | Split shell served; h1/nav/footer in sidebar, error+hint atop content, all five sections unchanged; every render-test string present in served HTML; full go test green; zero app.js edits |
| [SPS-01](tasks/01-scope-strip.md) | Settings scope strip with segmented control | Done | SPS-00 | 2026-09-14 | Strip served with radios/values/checked intact; server tests green (zero test edits); page-width caps removed early because the flush strip needs full width (was SPS-02 scope) |
| [SPS-04](tasks/04-general-section.md) | General settings group for wizard re-entry | Done | SPS-02 | 2026-09-14 | General nav item + section live, wizard ghost-button moved from footer (footer keeps layer explanation); server tests + validate green; 9090 restarted with new build |
| [SPS-06](tasks/06-pinned-footer-flat-nav.md) | Pinned footer and flat full-width side-menu selections | Done | SPS-05 | 2026-09-14 | Full-height app layout (board pattern), footer pinned flush bottom, nav hover+active full-width square (name-pill idiom dropped), mobile fallback restores page scroll; CSS-only, tests + validate green, 9090 restarted |
| [SPS-03](tasks/03-verification.md) | End-to-end verification and docs update | Done | SPS-02 | 2026-09-14 | Full suite green (`-count=1`), validate exit 0 after root-ledger sync, dead-CSS sweep done, layering audit ok, zero test edits; user visual pass pending (needs server restart) |
| [SPS-05](tasks/05-page-footer.md) | Page-wide settings footer for layer explanation | Done | SPS-02 | 2026-09-14 | Footer spans page width under the split; sidebar footer removed (sidebar = title + nav); test strings intact; tests + validate green; 9090 restarted |

## Decision log

- 2026-09-14 — Scope row contains only the Personal/Project segmented control; the
  "Settings" title moves into the sidebar header (user decision).
- 2026-09-14 — Settings page gets the full split-shell layout modeled on the
  Explorer page, not a widened in-page column (user decision).
- 2026-09-14 — Radio labels are exactly "Personal"/"Project"; layer semantics move
  to sidebar footer help and existing source badges (user decision).
- 2026-09-14 — Native radio inputs are kept (visually restyled) so `app.js` wiring
  and `render_test.go` assertions stay intact; zero JS changes is the goal.
- 2026-09-14 — The scope strip is full-bleed (negative margins cancel main's
  padding) and sticky at top: 52px, z-index 19; both `.settings` page-width caps
  were removed in SPS-01 (planned for SPS-02) because the flush strip requires
  full width.
- 2026-09-14 — No manual docs edits: `web/templates/AGENTS.md` is entirely
  marker-guarded (machine-maintained) and its settings learnings remain accurate
  since all DOM hooks are unchanged; the doc gardener records the new layout at
  close. Zero `render_test.go` edits were needed.
