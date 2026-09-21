# 2026-09-21-8235b: Mobile Chat Layout Hierarchy

- Change ID: 2026-09-21-8235b
- Created: 2026-09-21
- Branch: —
- Status: tracked in the tool-owned JSON state (.lessmess/workflow/)

## Objective and context

Distilled from the handoff authored by change 2026-09-21-aw36x
(`handoff-mobile-chat-layout.md`): rework the mobile Chat interface so the
conversation is the only persistent pane. Tasks, plans, task details, agents,
controls, and session management become temporary full-width views instead of
sidebars competing with Chat. Desktop keeps Chat's established behavior except
that at most one auxiliary panel may crowd the conversation at a time.

## Current behavior

- The chat overlay (`web/templates/layout.html`) renders a persistent
  multi-pane shell: `.chat-main` (family bar, transcript, inbox, status,
  composer) plus `#chat-agents` and `#chat-tasks` right drawers and a
  `#chat-controls-sheet` bottom sheet under `@media (max-width: 720px)`
  (`web/static/app.css` ~1178–1270). `#chat-tasks` reuses `renderTaskPanel`
  (board-mirroring status groups plus a Plan link into `#detail`).
- The composer permanently shows Attach, Reference (collapsed to `@` under
  720px), Interrupt, and Send; Interrupt renders regardless of busy state —
  only the delivery-controls row is busy-gated today.
- Plan/task documents open in the `#detail` modal, whose `buildDetailTOC()`
  fills a left `.modal-toc` rail; there is no mobile-specific contents UI.
- The chat header always shows Controls and Terminal (Agents and Tasks appear
  only when relevant via `hidden`), so compact widths spend scarce header
  space on rarely used actions.

## Target behavior

1. Single-pane navigation at compact widths (≤ ~840px, including phone
   landscape and split views): Chat is the only persistent pane; every other
   surface (Work, plan, task, agents, controls) becomes a temporary
   full-width view with explicit Back to Chat; drafts, transcript scroll,
   selected attachments, and navigation return position survive the moves.
2. Work view: the mobile tasks drawer is replaced by a full-screen view
   holding the plan entry point and the grouped task list.
3. Reading views: plan and task details open full-width; on mobile the left
   TOC rail is replaced by a sticky top Contents disclosure that stays usable
   with long documents and the virtual keyboard.
4. Message-first composer: full-width textarea, primary Send, Interrupt
   visible only while busy, and a single `+` action sheet hosting files,
   references, and skills; selected items remain visible as removable chips.
5. Slim mobile header: navigation, title, and state stay visible; Terminal,
   Controls, lifecycle, and session management move into an overflow menu;
   Work or Agents appear only when relevant.
6. Desktop: at most one auxiliary panel at a time and a firm minimum
   conversation width; Terminal behavior unchanged.

## Scope

- `web/templates/layout.html` and `web/templates/partials.html` (chat
  fragments as needed).
- `web/static/app.js` — view switching and return positions, header/overflow
  menu, composer action sheet, Interrupt busy gating, desktop panel
  arbitration, state preservation.
- `web/static/app.css` — compact-width shell, full-screen views, reading
  views with sticky Contents, action sheet, overflow menu, desktop
  min-width/arbitration.
- `internal/server/render_test.go` and the chat UI contract tests;
  phone-width checks via the existing render/contract suites.

## Non-goals

- No new frontend framework, bundler, or persistence layer — vanilla
  JavaScript and the existing design system only.
- No change to Terminal behavior and no OpenCode API adapter changes unless a
  small existing response field is required.
- Chat and Terminal stay attached to the same OpenCode session; no
  re-mapping of sessions.
- Desktop multi-panel layout is otherwise unchanged.

## Design decisions

- Breakpoint ~840px (handoff-agreed) as the single-pane threshold, covering
  phone landscape and split views; the existing 720px chat block folds into
  it.
- View switch with a return position rather than stacked drawers: each
  temporary view records where to go back to; per-session state (drafts,
  scroll, chips) stays keyed in `cstate` as today.
- One `+` action sheet hosts the attach/reference/skill entry points;
  existing caps (10 items, 20 MiB) and busy-delivery gating are preserved.
- Desktop arbitration: opening a second auxiliary panel closes the first
  (tasks or agents drawer vs controls sheet), with a firm conversation
  min-width.

## Acceptance criteria

- At 375px, 390px, phone landscape, and compact split widths, no task list,
  controls panel, agent list, or table of contents sits beside Chat or
  document content.
- Chat transcript and composer occupy the full available width when
  auxiliary views are closed.
- Work, Plan, Task, Agents, Controls, and management surfaces have clear
  Back to Chat navigation and do not discard Chat state.
- Mobile plan/task contents navigation appears above content and remains
  usable with long documents and the virtual keyboard.
- Attach/reference/skill actions are available from one compact composer
  menu; selected items remain visible as removable chips.
- Interrupt appears only while a session is busy.
- Desktop Chat and the existing Terminal retain their established behavior,
  except that multiple Chat side panels cannot crowd the conversation at
  once.
- Keyboard focus, Escape/backdrop behavior, touch targets, safe-area
  padding, and screen-reader labels are covered by focused tests and
  phone-width checks.

## Tasks

1. (task breakdown is maintained by the tool; see the board)
