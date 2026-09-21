
# TUI-05: Panel docs and manual verification

Status: see [../ledger.md](../ledger.md).

## Objective

Document the task panel in the README and verify it end-to-end in the browser
against the live server.

## Dependencies

- TUI-04 (panel fully implemented).

## Scope

- `README.md`: short addition to the board/terminal docs describing the task
  panel (change terminals only, grouped live mirror of the board, click opens
  task details).
- Manual verification evidence recorded in this task's Notes.

## Implementation steps

1. Update `README.md` (keep it to a sentence or two near the embedded
   terminal bullets).
2. Rebuild and restart the local server.
3. Browser checks on a change board with several tasks in mixed statuses:
   - terminal opens with the panel at ~20% on the right; xterm refits and the
     TUI remains usable (columns actually resize);
   - groups follow board column order, empty groups hidden, counts correct;
   - drag a card on the (underlying) board or let an agent edit the ledger:
     the panel updates within the debounce without a page reload and the
     terminal session is undisturbed;
   - click a row: task detail opens above the terminal; close returns to the
     session;
   - open a Discussion terminal from the index and an explorer chat: no panel;
   - narrow the window: min-width keeps the panel usable, xterm shrinks.
4. `lessmess validate` clean (docs warnings aside).

## Verification

- Panel acceptance criteria in [../plan.md](../plan.md) (items 5–7) confirmed
  with observations recorded below.

## Completion criteria

- README updated; checks in step 3 observed and recorded.

## Files affected

- `README.md`

## Notes

- Manual verification evidence goes here during execution.
- 2026-09-13: user completed the browser pass on the live server — panel at
  ~20% with live sync on board drags and ledger edits, full-bleed status
  bands, pointer cursor on rows, task detail and plan modals opening above
  the terminal, no panel on unassigned terminals. README bullet "Task panel"
  added under opencode integration. Full `go vet`/`go test` green and
  `lessmess validate` clean. Accepted by user → Done.
