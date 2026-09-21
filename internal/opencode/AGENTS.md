# AGENTS.md

<!-- tasktracker:begin -->
## Working in internal/opencode

- Purpose: a thin HTTP client for the opencode background service, covering only the endpoints lessmess needs.
- Key types: `Client` (Basic-auth HTTP wrapper), `APIError` (non-2xx responses), and `Session`; build clients with `New` or `DiscoverClient`, and call methods like `CreateSession`, `ListSessions`, `Prompt`, and `WaitDone`.
- All requests go through `Client.do`, which sets Basic auth with user `opencode` and unwraps the service's `{"data": ...}` response envelope.
- `Discover` shells out to `opencode2 service status`; the password is read from `~/.config/opencode/service.json` (override the path via `PasswordFromFile`).
- `WaitDone` blocks on the server-side wait endpoint and retries transport timeouts until the caller's context expires; a context error means the session is still busy.
- `TestLiveSmoke` only runs with `TT_LIVE_OPENCODE=1`; all other tests use `httptest` fakes.
- `CreateSessionWith` adds optional `agent` and `model` defaults to the V2 create body; plain `CreateSession` just delegates with empty values. A 400 for an unknown agent/model is returned to the caller — the warn-and-retry-plain fallback lives in `internal/server`'s `spawnSession`, not here.
- `ModelRef` mirrors the V2 `Model.Ref` schema (`providerID` + `id`, plus an optional `variant` lessmess never sets); model IDs may contain slashes, so a stored `providerID/id` string must be split on the first `/` only (`splitModelRef` in `internal/server`).
- `ListAgentsFor`/`ListModelsFor` scope requests with the deep-object `?location[directory]=<dir>` query so agents/providers defined by the served repository's own opencode config are included; they return raw lists — filtering to primary, non-hidden session drivers happens in `internal/server`'s `settingsapi.go`.
- `Session.ParentID` carries the service's `parentID`, populated on task-tool-spawned subagent children and parsed by both `ListSessions` and `GetSession`; it is the only child-relationship signal lessmess needs — `internal/server`'s task-session reconciler binds on it, and the parent model can never supply the child's session ID, so no method here needs to expose child IDs any other way.
- Mobile chat uses the narrow V2 adapters `ListMessages`, `ListActiveSessions`, `Interrupt`, `ListPermissions`/`ReplyPermission`, and `ListForms`/`ReplyForm`; assistant `text`, `reasoning`, and `tool` content decode to typed parts while `UnknownPart` retains the discriminator and raw JSON so API additions cannot break a transcript.
<!-- tasktracker:end -->
