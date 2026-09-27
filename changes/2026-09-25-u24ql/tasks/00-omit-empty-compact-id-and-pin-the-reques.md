# CMP-00: Omit empty compact id and pin the request contract

Fix `Client.CompactSession` in `internal/opencode/lifecycle.go` so manual
compaction stops sending the invalid `"id":""` body, and pin the request
contract in `internal/opencode/lifecycle_test.go`.

Context and evidence live in [plan.md](../plan.md): the service rejects
`{"id":"","delivery":"queue"}` with 400 (`Expected a string starting with
"msg_", got ""`), while `{"id":null,…}` returns 200 — and the schema makes
omission the safest shape (`additionalProperties: false`, `id` not
required).

## Files affected

- `internal/opencode/lifecycle.go` — `CompactSession` body construction
- `internal/opencode/lifecycle_test.go` — contract-pinning test additions

## Steps

1. In `CompactSession`, build the body conditionally: when `messageID` is
   empty send `map[string]any{"delivery": delivery}` only; when set, include
   `"id": messageID` as today. Keep the capability gate unchanged.
2. Extend the fake in `TestLifecyclePublishedContracts` (its OpenAPI
   document already advertises the compact path) to answer
   `POST /api/session/ses_1/compact` with a compaction inbox item, then:
   - call `CompactSession` with an empty message ID and assert the recorded
     body is exactly `{"delivery":"queue"}` — this is the regression pin;
     it fails against the old code;
   - call `CompactSession` with an anchor ID and assert the body carries
     `"id":"msg_…"` next to the delivery field.
3. Verify: `go vet ./... && go test ./...` from the repo root.

## Verification (before Test)

- New tests fail on the old body shape and pass with the fix.
- Live probe, mirroring the diagnosis: create a disposable OpenCode session
  in a temp directory via the service API, POST the exact new body
  (`{"delivery":"queue"}`) to its compact endpoint, confirm HTTP 200 with a
  compaction inbox item, then DELETE the probe session.
- Optional end-to-end confirmation in Chat: the Compact button on an idle
  session reports "Compaction queued" and the transcript later shows
  "Conversation context compacted." (User acceptance covers this; the task
  stops at Test with the evidence above.)
