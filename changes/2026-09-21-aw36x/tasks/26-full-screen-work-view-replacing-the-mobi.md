# MAC-26: Full-screen Work view replacing the mobile tasks drawer

## Objective

Replace the compact-width Tasks drawer with a full-screen Work view containing
the plan entry point and live grouped task list.

## Dependencies

- MAC-25.

## Scope

- Relabel the contextual Tasks action as Work.
- Reuse the existing board/task-panel source and live updates.
- Promote Plan above grouped tasks.
- Open plan/task reading views while preserving Work as the return view.

## Implementation steps

1. Route Work through the compact view stack instead of drawer classes.
2. Render plan-first, full-width task groups from the existing source.
3. Preserve bound/unbound visibility behavior and SSE refreshes.
4. Add contract tests for Work, Plan, and task navigation hooks.

## Verification

- Phone-width navigation: Chat → Work → Plan/Task → Work → Chat.
- Confirm no right drawer remains at compact widths.

## Completion criteria

Bound sessions expose a full-screen, live Work view; unbound sessions hide it.

## Files affected

- `web/templates/layout.html`
- `web/static/app.js`
- `web/static/app.css`
- Chat UI contract tests

## Notes

Desktop task-panel behavior is retained until MAC-30.
