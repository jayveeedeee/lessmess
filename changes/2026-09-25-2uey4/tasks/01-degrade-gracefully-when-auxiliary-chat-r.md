# CHAT-01: Degrade gracefully when auxiliary chat reads fail

## Why

Today any single failing read aborts the whole poll
(`writeChatUpstreamError`): a slow or erroring `ListPermissions` throws
away an already-fetched transcript and the UI updates nothing — which
reads exactly as "the API is blocked". Under a saturated service the aux
reads are the flaky ones; the transcript is the payload the user actually
needs.

## What

- In `chatSnapshot`, make only the transcript read (`ListMessagesPage`)
  fatal. A transcript failure still fails the poll as today.
- `ListActiveSessions` failure ⇒ render with `Busy=false`;
  `ListPermissions`/`ListForms` failure ⇒ render with those sections
  empty — HTTP 200, transcript intact.
- Mark degradation on the snapshot root (`data-degraded="true"`) when any
  aux read failed, if it threads through the template cheaply; skip the
  marker rather than reshaping the template contract.
- depends: CHAT-00 (fan-out lands first so degradation composes with
  concurrent reads).

## Files affected

- `internal/server/chat.go`
- `internal/server/chat_test.go`
- `web/templates/chat.html` (only if the degradation marker is added)

## Verification

- New tests: fake service where `ListPermissions` (then separately
  `ListActiveSessions`, then `ListForms`) returns 500 — snapshot returns
  200 with transcript blocks present and the section empty; transcript
  failure still returns the upstream error shape.
- Existing error-shape tests updated only where the degradation policy
  intentionally changed them.
- `go vet ./... && go test ./...` clean.
