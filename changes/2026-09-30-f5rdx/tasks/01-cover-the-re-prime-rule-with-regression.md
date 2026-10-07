# RP-01: Render the restoration row and fix the usage ceiling

## Why

The injected synthetic must not trade one wall for another: synthetics are
deliberately skipped in transcript rendering (`makeChatTranscriptBlocks`),
but the restoration event should be visible as a slim, honest row. Separately,
the usage panel computes percent against `model.Limit.Context` (400k on the
affected models) while the service auto-compacts at ~88–91% of
`model.Limit.Input` (272k) — measured 240,001 / 248,044 tokens before live
compactions — so the panel reads ~60% when compaction fires and its 80%
warning can never fire.

## What

- `internal/server/chat.go`: in `makeChatTranscriptBlocks`, synthetic
  messages carrying the restoration marker render as a one-line
  message block; all other synthetics remain skipped.
  `makeChatMessageView` maps the marker synthetic to a slim summary
  ("Conversation context compacted — session binding and state were
  restored for the next turn.") — the full ledger snapshot never
  renders.
- No template or CSS changes were needed after all: the generic
  non-user/non-assistant row styling already renders the block as a
  muted one-liner, visually consistent with compaction rows, and the
  message template renders text without a header.
- `internal/server/chat.go` (`chatUsage`): `ContextLimit` from
  `model.Limit.Input` with fallback to `Limit.Context` when Input is
  absent or zero; percent and the ≥80% warning now track the compaction
  trigger.

## Files affected

- `internal/server/chat.go`
- `internal/server/chat_test.go`
- `web/templates/partials.html`
- `web/static/app.css`

## Verification

- Render tests: marker synthetic → one-line row; other synthetics still
  skipped; usage percent uses the input limit and warns at 80% of it.
- Rebuild + restart, confirm the Runtime panel reads ~88–91% just before a
  real compaction on a 272k-input model.
