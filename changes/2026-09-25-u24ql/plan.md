# 2026-09-25-u24ql: Fix manual compaction empty message id

- Change ID: 2026-09-25-u24ql
- Created: 2026-09-25
- Branch: —
- Status: tracked in the tool-owned JSON state (.lessmess/workflow/)

## Objective and context

Manual compaction from the Chat UI has never worked: every click of the
Compact button ends in "Compaction failed: OpenCode rejected the request;
refresh the available options and retry." Root cause is in lessmess's own
OpenCode client, not the service: `Client.CompactSession`
(`internal/opencode/lifecycle.go`) always marshals both body fields, and the
Chat surface never supplies a message ID, so the service receives
`"id":""`.

The service contract (identical in the published and installed OpenAPI
document, `opencode2 v0.0.0-beta-17519`) requires `id` to be a
`msg_`-prefixed string or null. Verified live against the running service on
a disposable session:

- `{"id":"","delivery":"queue"}` → HTTP 400, `InvalidRequestError: Expected
  a string starting with "msg_", got "" at ["id"]` (the exact body lessmess
  sends today)
- `{"id":null,"delivery":"queue"}` → HTTP 200, compaction inbox item queued

## Current behavior

- Chat Compact → `POST /api/sessions/{id}/compact` with no `messageID` →
  `CompactSession` sends `{"id":"","delivery":"queue"}` → service answers
  400 → `writeChatUpstreamError` maps it to "OpenCode rejected the request…"
  → the UI reports "Compaction failed: …" every time.
- `CompactSession` is never exercised by `internal/opencode/lifecycle_test.go`,
  and the httptest fakes accept any body, so nothing pins the request shape.

## Target behavior

- Compact with no message anchor sends a spec-valid body without `id`
  (`{"delivery":"queue"}`); compaction is queued and the existing
  202 → transcript → pending-clear flow completes: "Compacting conversation
  context..." then "Conversation context compacted."
- A supplied `messageID` anchor still produces `"id":"msg_…"` in the body.

## Scope

- `internal/opencode/lifecycle.go` — `CompactSession`: build the request
  body conditionally, omitting `id` when the message ID is empty (same
  optional-field style as `DeliverPrompt`).
- `internal/opencode/lifecycle_test.go` — pin the contract: empty anchor
  sends exactly `{"delivery":"queue"}`; supplied anchor sends
  `"id":"msg_…"`.

## Non-goals

- No server (`internal/server/sessionlifecycle.go`) or frontend changes:
  the 400 originates between the lessmess client and the service; the
  server-side `msg_` validation of a supplied `messageID` and the UI gating
  are already correct.
- No auto-compaction (usage ≥ 80%) behavior changes — that path is
  service-driven and unaffected.
- No retry/fallback logic for other 400s.

## Design decisions

- **Omit `id` rather than send null.** Both are spec-valid and null was
  verified live, but the schema carries `additionalProperties: false` and no
  required list, so omission is the safest shape and matches how
  `DeliverPrompt` treats optional fields (`omitempty`). The implementation
  step re-verifies the exact omission body live on a disposable session
  before the task moves to Test.
- **Fix at the single body-building boundary.** `CompactSession` is the only
  caller (`internal/server/sessionlifecycle.go:377`), so one conditional map
  covers every path.

## Acceptance criteria

- `go vet ./... && go test ./...` clean.
- New lifecycle tests fail against the old code (body contains `"id":""`)
  and pass with the fix.
- Live probe of the exact new body on a disposable OpenCode session returns
  HTTP 200 with a compaction inbox item (session deleted afterwards).
- In Chat, the Compact button on an idle session queues compaction and the
  transcript shows the compaction lifecycle without a failure notice.

## Tasks

1. (task breakdown is maintained by the tool; see the board)
