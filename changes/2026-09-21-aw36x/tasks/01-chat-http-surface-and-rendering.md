# MAC-01: Chat HTTP surface and rendering

## Objective

Expose a small same-origin chat surface that turns OpenCode session state into
safe lessmess UI fragments and accepts the essential chat mutations.

## Dependencies

- MAC-00

## Scope

- Explicit transcript, prompt, interrupt, permission-reply, and form-reply
  routes under one session-scoped lessmess API.
- A normalized server-side chat view model and HTML fragment for user,
  assistant, reasoning, tool, error, permission, and form blocks.
- Shared safe markdown rendering for textual content and escaped fallback output
  for unknown parts.
- Route validation, nil-OpenCode behavior, upstream error mapping, and handler
  tests. No generic OpenCode proxy.

## Implementation steps

1. Add a focused chat handler file and register explicit routes for fetching a
   snapshot, posting a prompt, interrupting, and replying to pending
   interactions.
2. Validate `ses_` IDs, request shapes, reply values, and required fields before
   calling OpenCode; return 503 when the integration is unavailable.
3. Build presentation models from the API response. Render known parts with
   semantic classes and concise tool summaries; render unknown parts as escaped
   labelled fallbacks without dumping secrets or executable markup.
4. Render transcript and pending interactions as one fragment so each poll is
   authoritative and reconnects need no replay logic.
5. Test successful and failing mutations, empty and mixed transcripts,
   permissions/forms, unknown parts, HTML injection attempts, and upstream
   service failures.

## Verification

- `go test ./internal/server ./internal/opencode`
- Handler tests assert that OpenCode credentials never appear in responses.
- Re-fetching the same snapshot produces stable message identities and markup,
  allowing the browser to avoid duplicate visible entries.

## Completion criteria

A same-origin client can completely perform the core mobile conversation flow
through narrow lessmess routes, and every rendered payload is safe when supplied
malformed, unknown, or adversarial OpenCode content.

## Files affected

- `internal/server/server.go`
- `internal/server/chat.go`
- `internal/server/chat_test.go`
- `internal/server/render.go`
- `web/templates/partials.html` or a focused chat partial
- `internal/server/AGENTS.md` and `web/templates/AGENTS.md` as needed

## Notes

Keep polling stateless: lessmess does not cache transcripts or create chat state
files. OpenCode remains authoritative.
