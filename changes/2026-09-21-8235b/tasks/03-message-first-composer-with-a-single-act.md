# MCL-03: Message-first composer with a single + action sheet

Status: see [../ledger.md](../ledger.md).

## Objective

Make the chat composer message-first: full-width textarea with Send as the
primary action, Interrupt visible only while the session is busy, and one
compact `+` action sheet hosting attach, reference, and skill entry points,
with selections shown as removable chips.

## Dependencies

- None (independent of the shell tasks; lands on the existing composer).

## Scope

- `web/templates/layout.html`: composer DOM — replace the permanent
  Attach/Reference buttons with a single `+` button and an action-sheet
  container; Interrupt moves behind the busy state; Send stays the primary
  submit. Keep every existing id contract (`chat-file-input`,
  `chat-reference-btn`, `chat-reference-picker`, `chat-prompt`,
  `chat-send-btn`, `chat-interrupt-btn`, `chat-skill-chips`,
  `chat-draft-files`, `chat-delivery-controls`) — `app.js` and the render
  tests wire by id.
- `web/static/app.js`: `+` opens/closes the action sheet (files, references,
  skills); Interrupt is shown only when `chatBusy()` is true (today it always
  renders); busy-delivery gating and the 10-item/20 MiB caps unchanged.
- `web/static/app.css`: full-width textarea row, chips row, action-sheet
  styling with safe-area padding and 44px touch targets, both standalone and
  under the compact breakpoint.

## Implementation steps

1. Restructure `.chat-compose-row`: `+` button, textarea, Send. The `+`
   sheet lists Attach files, Reference…, and Attach skill (the skills entry
   point today reachable only from the controls sheet) and closes on
   selection, Escape, or backdrop tap.
2. Reference picking keeps the existing `#chat-reference-picker` inline flow
   (opened from the sheet on wide screens; the sheet simply focuses it) — no
   duplicate picker is created.
3. Gate Interrupt visibility on busy state: hide the button when the
   snapshot is idle, show it while `data-busy="true"` or a mutation is in
   flight; keep the existing interrupt POST handler unchanged.
4. Selected files, references, and skills remain visible as removable chips
   above the compose row (`#chat-draft-files`/`#chat-skill-chips` renderers
   reused; add removable skill chips if the current sheet path lacks them).
5. Delivery controls row (`#chat-delivery-controls`) keeps its busy-only
   visibility and behavior.

## Verification

- `node --check web/static/app.js`; render contract tests updated for the
  `+` sheet and pass (`go test ./internal/server/...`).
- Manual pass at 375px/390px and desktop: composer is message-first
  (textarea + Send), `+` opens one sheet with files/references/skills,
  choices appear as removable chips, sending clears them as today; Interrupt
  appears exactly while the session is busy and never when idle.
- Caps still enforced: >10 items and >20 MiB rejections, busy-delivery
  guardrails.

## Completion criteria

The composer reads as a message field first, with one compact menu for all
attachments, references, and skills, removable chips, and busy-only
Interrupt — on every width.

## Files affected

`web/templates/layout.html`, `web/static/app.js`, `web/static/app.css`,
`internal/server/render_test.go`.
