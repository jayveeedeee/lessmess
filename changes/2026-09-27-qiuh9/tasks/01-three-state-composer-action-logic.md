# PILL-01: Three-state composer action logic

## Why

The current single-button logic morphs send/stop and hardcodes
`delivery=queue` on busy submits, so steering is only reachable after the fact
through the inbox. The behavior must expose all three actions at typing time
and thread the delivery mode through the existing deliver flow.

## What

- `web/static/app.js`: rework `updateChatActionButton()` to a three-state
  model — idle-send (single send button), busy-stop (round stop only), and
  busy-pill (stop + steer + queue expanded) keyed on "composer has any
  content" (text or files/references/skills chips).
- Thread a delivery parameter through the busy-submit path so the queue
  segment/Enter submit with `delivery=queue` and the steer segment submits
  with `delivery=steer`, replacing the hardcoded `"queue"`.
- Both send modes share the existing capability validation
  (`promptDelivery`, files, skills) and error surfacing; stop keeps its
  interrupt handler.
- All three segments disable while `cstate.mutation` is in flight; a poll
  flipping busy to false reverts the pill to the idle send button.

## Files affected

- `web/static/app.js`

## Verification

- Manual pass (after rebuild): Enter and queue segment deliver with
  `delivery=queue`, steer segment with `delivery=steer`; drafts clear, inbox
  refreshes, pill collapses; stop clickable in every busy state including
  while typing; old-service refusal path still message-based.
