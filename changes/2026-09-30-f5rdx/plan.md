# 2026-09-30-f5rdx: Fix stale compaction re-prime detection

- Change ID: 2026-09-30-f5rdx
- Created: 2026-09-30
- Branch: —
- Status: tracked in the tool-owned JSON state (.lessmess/workflow/)

## Objective and context

The compaction re-prime restores a bound session's binding and state after its
context is compacted. As shipped (2026-09-27, `8efb17e`) and reworked
(2026-10-02, `7dab1fb` — change 2026-10-02-s4p21), the restoration **rides the
user's own prompt**: lessmess rewrites the next prompt into
"Context restoration: … [state ledger] ----- *user text**". Two consequences
remain user-visible:

1. **The user's message is merged with restoration text.** In the transcript —
   and, worse, in the pending queue, which renders the item text verbatim
   (`web/static/app.js`, `renderChatInbox`) — their message appears appended to
   the bottom of a ~5k-character "this conversation was compacted" wall. Users
   read this as the compaction message swallowing their message.
2. **Queue blindness loops the wrap.** `needsReprime` recognizes compensation
   only from *delivered transcript messages*. A queued follow-up does not land
   in the transcript, so every queue submission after a compaction re-wraps,
   and if queued items are cancelled the compaction is never compensated —
   the preamble re-attaches on every send and queue, indefinitely.

s4p21's stateless transcript rule is correct for delivered messages (verified
against live data: no misfires post-deployment) and stays as the detection
core. This change replaces the *delivery vehicle* and fixes one display lie.

## Current behavior

- `maybeReprime` (`internal/server/reprime.go`) rewrites the prompt text at
  both call sites (`chatPrompt` in `internal/server/chat.go`, `sessionDeliver`
  in `internal/server/sessionlifecycle.go`).
- `needsReprime` walks one descending `ListMessagesPage` (limit 50): re-prime
  iff the newest completed compaction is strictly newer than the newest user
  message. Queued-but-undelivered prompts are invisible to it.
- The pending queue renders the full wrapped text; the transcript renders the
  wrapped user message as one large bubble.
- The usage panel (`chatUsage`) computes percent against
  `model.Limit.Context` (400k on the affected models) and warns at 80% of it,
  but the service auto-compacts at ~88–91% of `model.Limit.Input` (272k) —
  measured 240,001 and 248,044 tokens before two live compactions. The panel
  reads ~60% when compaction fires and its warning can never fire.

## Target behavior

- **Inject, don't rewrite.** When the rule detects an uncompensated
  compaction on a bound session, lessmess calls the service's
  `POST /api/session/{sessionID}/synthetic` ("Add synthetic message") with the
  restoration text and `resume: false` (admitted to context, no model turn
  scheduled). The user's prompt or queued follow-up then goes through
  **verbatim** — no code path modifies user text anymore.
- **Self-terminating.** The injected synthetic lands in the transcript
  immediately, unlike queued prompts, so the very next rule evaluation sees it
  as compensation. The queue loop is structurally impossible: nothing depends
  on a user message landing.
- **Recognition.** The restoration synthetic carries a stable marker prefix
  ("Context restoration:"). `needsReprime` treats the newest user message *or*
  newest marker-bearing synthetic as compensation; the rule remains otherwise
  unchanged (strictly-newer completed compaction, ties count as continued,
  read failures fail open).
- **Slim presentation.** The transcript renders the marker-bearing synthetic
  as a one-line system row ("Context restored after compaction · bound to
  change X"); other synthetics stay skipped as today. Queue rows now show only
  the user's text with no changes. The full state ledger keeps riding inside
  the synthetic (invisible to the UI, ~1.3k tokens, useful to the agent).
- **Honest usage ceiling.** `chatUsage` bases `ContextLimit` on
  `model.Limit.Input` (falling back to `Limit.Context` when Input is absent),
  so the percent and the 80% warning track the real compaction trigger.
- **Fail-open to nothing, never to rewriting.** If a service build lacks the
  synthetic endpoint (capability-gated), lessmess logs and skips the
  restoration; it never falls back to modifying user text.

## Scope

- `internal/opencode/lifecycle.go` — `AddSynthetic` client method plus its
  `LifecycleCapabilities` entry (exact path+method detection from the OpenAPI
  document, per the package's capability pattern); strict request body
  (`{"text":…}` with `resume:false` omitted-or-false per the schema).
- `internal/server/reprime.go` — `maybeReprime` replaced by an
  `ensureReprime(ctx, sessionID)` pre-step; marker constant; recognition of
  the marker synthetic in the rule.
- `internal/server/chat.go`, `internal/server/sessionlifecycle.go` — call
  sites: run `ensureReprime` before the prompt/deliver, pass user text
  through untouched; transcript rendering of the marker synthetic.
- `internal/server/chat.go` (`chatUsage`) — input-limit ceiling with context
  fallback.
- `web/templates/partials.html`, `web/static/app.css` — minimal styling for
  the one-line restoration row (reuse system-message styling; no JS logic).
- Tests: `internal/opencode/lifecycle_test.go` (body contract),
  `internal/server/reprime_test.go` (rewrite), `internal/server/chat_test.go`
  (usage ceiling, synthetic rendering).

## Non-goals

- No change to when compactions fire (service behavior at the input limit) or
  to the manual Compact action.
- No service-side changes; lessmess only consumes the published synthetic
  endpoint.
- No revival of `compactionWatch` or any persisted re-prime state.
- No change to the restoration's informational content beyond the marker.

## Design decisions

- **Synthetic message, not prompt piggyback** (user decision): the service's
  own injection mechanism (same one it uses for AGENTS.md updates) is the
  correct vehicle; user messages must never be rewritten. `resume: false`
  lands the text without burning a model turn.
- **Detection stays transcript-based** (s4p21): the rule is sound for
  delivered messages; extending compensation to include the marker synthetic
  closes the queue gap without new state.
- **Injection at prompt/deliver submission time**: same trigger points as
  today — when the user next acts on the session — keeping the change
  minimal; proactive injection on snapshot observation is a possible
  follow-up, not this change.
- **Input-limit ceiling** for the usage panel: measured service behavior
  (240–248k on a 272k input limit) vs the 400k context figure the panel used,
  which no session can reach before compaction.

## Acceptance criteria

- After a completed compaction, the next send *or queued follow-up* goes
  through verbatim; a synthetic "Context restoration:" message precedes it in
  context; the transcript shows a one-line restoration row; the queue shows
  only the user's text.
- Multiple queued follow-ups after one compaction produce exactly one
  restoration synthetic (submissions after the first see it and skip).
- Cancelled queued items cannot cause repeated restoration.
- A service without the synthetic endpoint: prompts still go verbatim,
  restoration skipped with a logged warning, no user-visible failure.
- Usage panel: percent and warning computed against the input limit (fallback
  context when absent); the pre-compaction reading is ~88–91%, warning fires
  beforehand.
- `go vet ./...`, `go test ./...`, `CGO_ENABLED=0 go build`, and
  `lessmess validate` clean.

## Tasks

1. (task breakdown is maintained by the tool; see the board)
