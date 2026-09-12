# AGENTS.md

<!-- tasktracker:begin -->
## Working in internal/opencode

- Purpose: a thin HTTP client for the opencode background service, covering only the endpoints tasktracker needs.
- Key types: `Client` (Basic-auth HTTP wrapper), `APIError` (non-2xx responses), and `Session`; build clients with `New` or `DiscoverClient`, and call methods like `CreateSession`, `ListSessions`, `Prompt`, and `WaitDone`.
- All requests go through `Client.do`, which sets Basic auth with user `opencode` and unwraps the service's `{"data": ...}` response envelope.
- `Discover` shells out to `opencode2 service status`; the password is read from `~/.config/opencode/service.json` (override the path via `PasswordFromFile`).
- `WaitDone` blocks on the server-side wait endpoint and retries transport timeouts until the caller's context expires; a context error means the session is still busy.
- `TestLiveSmoke` only runs with `TT_LIVE_OPENCODE=1`; all other tests use `httptest` fakes.
<!-- tasktracker:end -->
