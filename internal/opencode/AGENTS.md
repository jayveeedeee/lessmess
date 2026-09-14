# AGENTS.md

<!-- tasktracker:begin -->
## Working in internal/opencode

- Purpose: a thin HTTP client for the opencode background service, covering only the endpoints tasktracker needs.
- Key types: `Client` (Basic-auth HTTP wrapper), `APIError` (non-2xx responses), and `Session`; build clients with `New` or `DiscoverClient`, and call methods like `CreateSession`, `ListSessions`, `Prompt`, and `WaitDone`.
- All requests go through `Client.do`, which sets Basic auth with user `opencode` and unwraps the service's `{"data": ...}` response envelope.
- `Discover` shells out to `opencode2 service status`; the password is read from `~/.config/opencode/service.json` (override the path via `PasswordFromFile`).
- `WaitDone` blocks on the server-side wait endpoint and retries transport timeouts until the caller's context expires; a context error means the session is still busy.
- `TestLiveSmoke` only runs with `TT_LIVE_OPENCODE=1`; all other tests use `httptest` fakes.
- (2026-09-13-2) `CreateSessionWith` adds optional `agent` and `model` defaults to the V2 create body; plain `CreateSession` just delegates with empty values. A 400 for an unknown agent/model is returned to the caller — the warn-and-retry-plain fallback lives in `internal/server`'s `spawnSession`, not here.
- (2026-09-13-2) `ModelRef` mirrors the V2 `Model.Ref` schema (`providerID` + `id`, plus an optional `variant` lessmess never sets); model IDs may contain slashes, so a stored `providerID/id` string must be split on the first `/` only (`splitModelRef` in `internal/server`).
- (2026-09-13-2) `ListAgentsFor`/`ListModelsFor` scope requests with the deep-object `?location[directory]=<dir>` query so agents/providers defined by the served repository's own opencode config are included; they return raw lists — filtering to primary, non-hidden session drivers happens in `internal/server`'s `settingsapi.go`.
<!-- tasktracker:end -->
