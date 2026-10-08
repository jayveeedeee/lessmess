# CHAT-02: Name chats once after the first real user message

## Why

The fixed "Codebase chat" title makes a Chats list hard to navigate. Naming must
reflect the actual user's subject, not the system prime, and must never fight
manual titles or the title assigned when a chat becomes a change.

## What

- Hook successful admission of the first real input through both the ordinary
  chat prompt endpoint and queue/steer delivery. Persist one-time naming state
  for eligible new general chats and placeholder planning discussions.
- Exclude setup prompts, synthetic messages, helper activity, bound sessions,
  and explicitly/custom-titled conversations; preserve historical meaningful
  titles instead of bulk-renaming existing entries.
- Add a capability-gated, stateless OpenCode title-generation adapter verified
  against the installed OpenAPI contract. Generate a bounded descriptive title
  from the first input without tools or new transcript messages.
- Run generation with a bounded background lifetime rather than delaying sends.
  Honor the session model when available; use a sanitized first-input fallback
  for unavailable generation, timeouts, empty/invalid output, or file-only input.
- Reuse compatible rename handling, persist fallback titles, and make manual
  renames explicitly own the title. Serialize/recheck completion so late results
  cannot overwrite manual names or scaffolded change titles, resurrect deleted
  entries, or repeatedly retrigger from polling.
- Keep cards, the open title, and breadcrumbs fresh through existing client
  snapshot/list refresh mechanisms.

## Files affected

- `internal/opencode/client.go`, `internal/opencode/lifecycle.go`, and their tests
  or a focused generation adapter/test file in that package
- `internal/server/chat.go` and `internal/server/sessionlifecycle.go`
- `internal/server/mapping.go` and focused naming tests under `internal/server/`
- `web/static/app.js` and browser-glue tests for title refresh

## Verification

- Assert exact advertised generation/rename routes and request bodies using fake
  OpenAPI/service responses; never probe mutations or assume version support.
- Prove primes do not name chats, the first accepted actual input does, rejected
  sends do not, and queue/steer/retry/reload paths do not repeatedly name them.
- Cover custom/manual titles, attachment-only input, malformed/oversized output,
  timeouts, missing capabilities, and degraded service-side rename behavior.
- Exercise races with manual rename, promotion, unlink/delete, and multiple
  sends. Naming failure must not fail a successful user send or mutate history.
- Run focused adapter/server/browser-glue tests and workflow validation.
