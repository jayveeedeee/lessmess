
# SET-01: opencode client session defaults support

Status: see [../ledger.md](../ledger.md).

## Objective

Extend `internal/opencode` so sessions can be created with an explicit
agent and/or model, and so the server can list agents and models for the
settings dropdowns.

## Dependencies

None (developable in parallel with SET-00).

## Scope

- In `internal/opencode/client.go`:
  - `ModelRef` type (`id`, `providerID`, optional `variant` — v1 never sets
    variant) matching the V2 `Model.Ref` schema.
  - `CreateSessionWith(ctx, title, directory string, agent string,
    model *ModelRef)`: like `CreateSession` but adds `agent` and `model`
    to the create body when set. `CreateSession` delegates with empty
    values so existing callers compile unchanged.
  - Fallback: when a create with agent/model fails with a 400, log and
    retry once as a plain create (a stale setting must never block session
    creation).
  - `AgentInfo` / `ModelInfo` types and `ListAgents(ctx)` /
    `ListModels(ctx)` wrapping `GET /api/agent` and `GET /api/model`
    (response envelope `{location, data}` — note `Client.do` already
    unwraps `{"data": ...}`; confirm whether these two endpoints nest
    `data` twice and handle accordingly).
- Tests in `internal/opencode/client_test.go` (httptest fakes).

## Implementation steps

1. Verify response envelopes against the running service
   (`opencode2 api get /api/agent`, `/api/model`) — agent entries carry
   `id`, `name`, `description`, `mode`; model entries carry `id`,
   `providerID`, `name`.
2. Implement `ModelRef`, `CreateSessionWith` + fallback retry,
   `ListAgents`, `ListModels`.
3. Tests: create body includes agent/model when set and omits them when
   empty; 400 triggers exactly one plain retry; non-400 errors do not
   retry; list methods decode entries.

## Verification

- `go test ./internal/opencode/ -v` passes.
- Optional live check: `TT_LIVE_OPENCODE=1 go test ./internal/opencode/ -run Live`.

## Completion criteria

- New methods behave per plan against fakes; existing tests untouched and
  passing.

## Files affected

- `internal/opencode/client.go`
- `internal/opencode/client_test.go`

## Notes

- API shapes verified against `https://opencode.ai/v2/openapi.json`:
  create accepts `agent` (string) and `model` (`{id, providerID}`);
  `POST /api/session/{id}/agent|model` exist but are NOT needed (v1 sets
  defaults at creation; non-goal: switching existing sessions).
- Read `internal/opencode/AGENTS.md` before editing.
- Deviation from the original scope: the plain-retry fallback on 400 lives
  in one server-side helper (SET-02's `spawnSession`) shared by all six
  call sites, not in the client — the client stays a pure API wrapper and
  warning logs stay in the server.
- Envelope finding: `GET /api/agent` and `/api/model` return top-level
  `{location, data}` where `data` IS the payload, so `Client.do`'s existing
  unwrap needs no changes.
- Verified 2026-09-13: `go vet ./internal/opencode/` and
  `go test ./internal/opencode/ -v` pass (new tests: create-with-defaults
  body shape, empty-defaults omission, list/default decode).
