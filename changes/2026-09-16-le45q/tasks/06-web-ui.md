---
id: NTD-06
title: Web UI badges, drill-down, modal links
---

# NTD-06: Web UI badges, drill-down, modal links

Status: see [../ledger.md](../ledger.md).

## Objective

Deliver the user-facing experience: recursive progress badges, expand affordance, drill-down navigation, level-aware task creation, nested in-modal links, and session surfaces on sub-boards.

## Dependencies

NTD-04, NTD-05

## Scope

- `web/templates/` + `web/static/`: board fragment, app.js wiring, CSS. Server markup hooks come from NTD-04.

## Implementation steps

1. Card rendering: rollup badge (`x/y ✓`) on any task with children, an expand affordance (icon button) posting to `/changes/{id}/expand`, and drill-down on card/title click for containers (`?task=` navigation or fragment swap with history push).
2. Breadcrumb above the drill-down board: change → ancestors → current node; each crumb navigates up.
3. Add-task form targets the viewed level (posts `parent`); label adapts ("Add subtask").
4. Sessions panel: sub-boards list sessions bound to the exact task with a per-task Continue (`tt-last-session:<change>/<task>` localStorage key); root board unchanged (full list). Start/Continue buttons remain the manual retry for failed auto-spawns.
5. Detail modal: generalize the `.md` link interception in `app.js` for nested hrefs — any `tasks/…/tasks/NN-x.md` path maps to the task detail endpoint, and `../ledger.md` resolved from a task file at depth N opens that container's ledger; keep cross-change plan links working.
6. Terminal task panel: verify it mirrors the drill-down board (it clones `#board` DOM) and that deep card clicks open the right detail.
7. CSS: badge, expand button, breadcrumb, sub-board header — inside the existing z-index ladder; no new fixed surfaces.
8. Board header: change-level progress indicator (`x/y ✓`) beside the status pill.

## Verification

- `render_test.go` green for new hooks (badge values, breadcrumb, data attributes).
- Manual browser pass over a three-level fixture change: expand (button and agent-written container), badge updates after each status change via SSE fragment refresh, drill-down/back navigation, add-subtask at depth, deep task detail + ledger links in-modal, terminal panel mirroring, sessions panel scoping, close blocked/allowed states.

## Completion criteria

The full interaction model from the plan's Day 1–4 walkthrough works in the browser against the rebuilt binary; no regressions on flat changes' boards.

## Files affected

- `web/templates/board.html`, `web/templates/partials.html`
- `web/static/app.js`, `web/static/app.css`
- `internal/server/render_test.go`

## Notes

- Server-verified: badge/crumb/expand/parent-form markup tests, `data-doc` modal context, container-ledger endpoint with path guard, per-task session scoping and Continue keying, SSE refresh preserving `?task=`, terminal panel mirrors the drill-down board (same `#board` DOM).
- User browser checklist (acceptance work in `Test`): expand a card via ⤢ → sub-board with breadcrumb; badge counts update after drags; add subtask at depth; nested task detail + its `../ledger.md` opens the container ledger in-modal; sessions panel scoped on sub-boards; Start/Continue creates a task-bound session; close blocked while a deep task is open.
