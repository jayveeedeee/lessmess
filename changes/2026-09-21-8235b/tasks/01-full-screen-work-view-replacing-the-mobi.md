# MCL-01: Full-screen Work view replacing the mobile tasks drawer

Status: see [../ledger.md](../ledger.md).

## Objective

At compact widths, replace the right-hand `#chat-tasks` drawer with a
full-screen Work view inside the MCL-00 shell: the change's plan entry point
at top, the grouped task list beneath it, both as reading-first content.

## Dependencies

- MCL-00 (the view stack this view lives in).

## Scope

- `web/static/app.js`: render the Work view's contents from the same source
  data `renderTaskPanel` uses (board fragment mirror + plan link), adapted to
  full-width layout; keep `#chat-tasks-btn` as the Work entry point (relabel
  it "Work") and route it through the view controller instead of the drawer
  toggle.
- `web/static/app.css`: full-screen Work view styling (safe-area padding,
  sticky group headers optional, Back row from the shell).
- `web/templates/layout.html`: only if the Work view needs its own container
  distinct from the old drawer aside.

## Implementation steps

1. Route the Work button through `openChatView("work")` (MCL-00) at compact
   widths; keep the desktop drawer behavior unchanged when the media query
   does not apply.
2. Build the Work view body: the plan entry (currently the `.ttp-foot` Plan
   link) promoted to a prominent header row, then the grouped task list
   (status groups with counts, per-task rows opening the task detail).
3. Reuse `renderTaskPanel`'s data path (`syncTerminalTasks`/board fragment
   sources) so SSE-driven updates keep working; only the rendering target and
   layout differ at compact widths.
4. Opening a task or the plan from Work pushes the reading view (MCL-02)
   while Work stays as the return position.
5. Update the chat UI contract tests: Work button id/label, full-screen view
   container, plan entry present.

## Verification

- `node --check web/static/app.js`; chat contract tests pass.
- Phone-width manual pass: no drawer slides in; Work fills the screen; plan
  and task rows open full-width reading views; Back returns to Work, then to
  Chat; the transcript and composer are untouched behind it.
- A session bound to a change shows Work; an unbound session hides it, as
  today (`setChatTasks` visibility rules preserved).

## Completion criteria

The mobile tasks drawer no longer exists; Work is a full-screen view with
plan-first navigation and live task groups, reachable only when the session is
bound to a change.

## Files affected

`web/static/app.js`, `web/static/app.css`, `web/templates/layout.html`,
chat UI contract tests.
