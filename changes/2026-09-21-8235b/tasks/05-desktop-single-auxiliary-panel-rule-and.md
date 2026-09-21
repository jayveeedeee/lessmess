# MCL-05: Desktop single-auxiliary-panel rule and conversation min-width

Status: see [../ledger.md](../ledger.md).

## Objective

On desktop (above the compact breakpoint), allow at most one Chat auxiliary
panel open at a time and preserve a firm minimum width for the conversation,
so panels can never crowd the transcript. Terminal behavior remains
unchanged.

## Dependencies

- None (independent; desktop-only code paths).

## Scope

- `web/static/app.js`: panel arbitration when opening tasks, agents, or the
  controls sheet — opening one closes the others; ordering/precedence
  deterministic (newest wins).
- `web/static/app.css`: a `min-width` on `.chat-main` (or the transcript
  column) so the conversation keeps a readable width even with a panel open;
  drawer widths capped so overlay + panel fit common desktop sizes.

## Implementation steps

1. Extract the four open/close entry points (`chat-tasks-btn`,
   `chat-agents-btn`, `chat-controls-btn` handlers) through one arbiter that
   closes any other open panel first; keep each panel's own close/backdrop
   behavior intact.
2. Set the conversation minimum width in `app.css`; verify the transcript
   never drops below it with a panel open at 1024px, 1280px widths.
3. Confirm no interaction with the terminal overlay: Terminal opens its own
   overlay and is untouched by the arbiter.
4. Contract test: the arbiter function exists and is called from all three
   entry points (script-level assertion, matching the existing
   `strings.Contains` style).

## Verification

- `node --check web/static/app.js`; `go test ./internal/server/...`.
- Manual desktop pass: open Tasks, then Agents — Tasks closes; open Controls
  — Agents closes; closing a panel restores the full-width conversation;
  transcript column never squeezes below the minimum; Terminal open/close
  behaves exactly as before.

## Completion criteria

Desktop Chat shows at most one auxiliary panel at a time, the conversation
keeps a firm minimum width, and Terminal behavior is unchanged.

## Files affected

`web/static/app.js`, `web/static/app.css`, chat UI contract tests.
