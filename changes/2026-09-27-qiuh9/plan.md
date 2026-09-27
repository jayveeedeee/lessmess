# 2026-09-27-qiuh9: Busy composer queue/steer pill

- Change ID: 2026-09-27-qiuh9
- Created: 2026-09-27
- Branch: —
- Status: tracked in the tool-owned JSON state (.lessmess/workflow/)

## Objective and context

While a session is running, the chat composer offers no way to queue or steer a
follow-up from the composer itself: the single `#chat-send-btn` morphs between
send and stop, busy sends hardcode `delivery=queue`, and steering requires
sending first, then flipping the item's delivery select in the `#chat-inbox`
strip afterwards. Worse, typing during a run hides the stop affordance (the
button flips to send while the composer is focused with text).

This change turns the composer's action button into a segmented pill while a
session is busy and the composer holds content: stop (persists), steer, and
queue, expandable with an animation, so all three actions are reachable at the
moment the user starts typing. Pending follow-ups also belong in the thread,
not in the old fixed strip above the composer.

## Current behavior

- `updateChatActionButton()` (web/static/app.js) flips `#chat-send-btn` between
  send and stop from busy state plus composer focus/text; while typing during a
  run the stop affordance is hidden.
- Submitting while busy posts `/api/sessions/{id}/deliver` with
  `delivery=queue` hardcoded; steering happens only after the fact via the
  per-item delivery select in `#chat-inbox`.
- The server (internal/server/sessionlifecycle.go `sessionDeliver`) already
  validates `delivery` as `queue|steer` and passes it to
  `opencode.DeliverPrompt`, which supports both the rich PATCH body and legacy
  `/queue`|`/steer` install paths.

## Target behavior

- Busy + empty composer: the action area is the round accent stop button,
  visually identical to today.
- Busy + composer has content (text or any attached chips): the stop button
  expands into a pill of three accent segments separated by 2px vertical
  dividers — left stop (persisted), middle steer (directional glyph), right
  queue (stacked lines ending in a right-pointing arrow). No visible words: all three controls show only icons,
  with `aria-label`/`title` for accessibility and tooltips.
- Enter and the queue segment submit the draft as a queued durable follow-up
  (queue is `type=submit`; steer and stop are `type=button`); the steer segment
  runs the identical deliver flow with `delivery=steer`.
- Successful send (either mode) clears the draft and collapses the pill back to
  the round stop button. While pending, the follow-up appears at the end of
  the scrollable thread as a user-shaped bubble with a dotted outline and a
  Queued or Steering label. There is no separate pending strip, inline Cancel
  button, or post-send delivery selector.
- Clicking a pending bubble selects it, brightens its dotted accent border,
  and changes the composer control to a two-segment Stop | × pill, where ×
  cancels that item. Clicking the bubble again deselects it; its existing draft
  survives selection. Typing deselects it and restores the normal composer
  actions. Keyboard Enter/Space can toggle selection too.
- Selecting a regular copyable message highlights it and puts its actions in
  that same always-visible composer pill, not a popup beside the bubble:
  Copy for assistant/user messages and additionally Fork/Revert for user
  messages. The first segment remains Send when idle or Stop when busy.
  Re-selecting, Escape, clicking outside, or typing dismisses selection;
  pending-message selection is mutually exclusive. Drafts are preserved.
- Pending bubbles are sourced from OpenCode's inbox, survive transcript
  snapshot swaps, and disappear when cancelled or delivered; if a delivered
  transcript message and stale inbox item share an ID, only the delivered
  message is shown.
- The expansion animates width with a fixed 22px radius at both ends (the
  44px collapsed width is still a circle), while segments fade/slide in and
  dividers scale; it collapses in reverse without stretched-circle ends.
  `prefers-reduced-motion: reduce` swaps instantly.
- When the run ends mid-typing (busy flips false on a poll), the pill reverts
  to the single idle send button.

## Scope

- `web/templates/layout.html` — wrap `#chat-send-btn` in a `#chat-action-pill`
  container and add `#chat-steer-btn` / `#chat-queue-btn` (ids wired in
  app.js, per the template contract); remove the separate `#chat-inbox` strip.
- `web/static/app.js` — derive action-pill segments from idle/busy,
  draft, pending-item, or selected-message state; thread a delivery parameter
  through the busy-submit path instead of the hardcoded `"queue"`; shared
  capability validation for both send modes; disable all segments while a
  mutation is in flight. Render pending follow-ups from the inbox inside the
  transcript, including after snapshot swaps, with selection and cancellation
  via the composer pill rather than inline controls or a delivery selector.
- `web/templates/partials.html` — keep selectable transcript message metadata
  and raw Markdown sources, but remove the inline Copy/Fork/Revert menu.
- `web/templates/layout.html` — add icon-only Copy, Fork, and Revert segments
  to the existing action pill, shown by selected-message context.
- `web/static/app.css` — pill container, segments, dividers, expansion
  animation, reduced-motion fallback, ≤840px sizing; dotted pending-message
  bubble and brighter selected accent border instead of fixed inbox-strip
  styles. Generalize pill sizing for one, two, or three segments.
- `internal/server/render_test.go` — update the embedded composer contract
  assertions to pin the three segments, queue/steer wiring, and inline
  pending-message rendering instead of the superseded inbox strip.

## Non-goals

- No Go server behavior changes: `/deliver` and the opencode client already
  carry `delivery` end to end (the embedded UI contract test does change).
- No change to OpenCode's inbox API, capability detection, or the polling
  model; only the browser presentation and controls change.
- No persistence of a "last used delivery mode"; Enter always queues.

## Design decisions

- Enter = Queue (user decision): queue is the current default behavior and the
  least surprising keyboard action; steer is click-only.
- The pill keys expansion off "composer has any content" (text **or**
  files/references/skills chips) rather than the current text-only check, which
  also fixes files-only drafts not surfacing send affordances.
- Steer rides the same `promptDelivery` capability gates as queue; on services
  without it, both segments surface the existing refusal message.
- The user chose inline, cancellable queued messages rather than the old
  pending strip. The delivery choice is made at send time; changing delivery
  afterward is no longer offered in Chat.
- The user then chose bubble selection and an icon-only Stop | × composer
  action in place of an inline Cancel button. Keep the same segmented pill
  mechanism reusable for other combinations of actions.
- Message actions belong in the fixed composer rather than alongside long
  messages that may extend beyond the viewport; the same segmented control
  supports Copy, Fork, and Revert segments selected by message type.

## Acceptance criteria

- Busy + empty composer shows only the round stop button (unchanged look).
- Busy + content expands the pill with stop / steer / queue segments and 2px
  dividers; the width animation keeps both end radii fixed at 22px and
  respects `prefers-reduced-motion`.
- Enter and the queue segment deliver with `delivery=queue`; the steer segment
  delivers with `delivery=steer`; both clear the draft and refresh pending
  thread messages.
- Pending user follow-ups have dotted outlines and status labels in the
  scrollable thread; selecting one brightens its border and shows Stop | ×
  in the composer (no inline Cancel, fixed strip, or mode selector).
- Clicking a regular message highlights it and shows Copy (assistant/user),
  plus Fork/Revert (user only), beside Send/Stop in the fixed composer.
  Selection survives transcript refreshes, or clears on reselect, Escape,
  outside click, typing, disappearance, or session switch. Pending selection
  and regular-message selection remain mutually exclusive.
- Pending messages persist through polling, disappear on successful cancel or
  delivery, and never duplicate a delivered transcript message.
- Stop remains clickable in all busy states, including while typing.
- Idle composer shows the single send button exactly as today.
- All three segments disable while a chat mutation is in flight.
- At ≤840px the pill fits the composer row without overflow.
- `go vet ./...` and `go test ./...` pass; the rebuilt binary serves the new
  composer (assets are embedded).

## Tasks

1. (task breakdown is maintained by the tool; see the board)
