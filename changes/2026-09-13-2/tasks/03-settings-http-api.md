
# SET-03: Settings HTTP API

Status: see [../ledger.md](../ledger.md).

## Objective

Expose settings over HTTP for the settings page and other clients:
read effective + layered values, write one layer at a time, and proxy
agent/model options from the opencode service.

## Dependencies

- SET-00 (settings store)
- SET-01 (`ListAgents` / `ListModels`)

## Scope

- Routes (registered in `server.New`):
  - `GET /api/settings` → `{effective, project, personal, sources,
    loadError?}`; `effective` is the concrete view; `project`/`personal`
    are the raw layer files (null when absent); `sources` maps dotted
    field paths to `default|project|personal`.
  - `PUT /api/settings?scope=project|personal` → partial JSON body; only
    submitted fields change in that layer; empty string / null clears a
    field from the layer (inheritance resumes); 400 on unknown scope, 422
    on malformed body; responds with the same payload as GET.
  - `GET /api/settings/options` → `{available, agents, models,
    defaultModel}`; agents filtered to `mode == "primary"`; models carry
    `id`, `providerID`, `name`, plus a pre-joined `providerID/id` value;
    when `s.oc` is nil or a list call fails, 200 with `available:false`
    and empty lists (the page then shows a hint, values still saveable).
- Handler tests with fixture store + fake opencode client.

## Implementation steps

1. Implement the three handlers in `settings.go` (or a new
   `settingsapi.go`), register routes.
2. Save path reuses SET-00's scoped save and re-reads effective settings.
3. Tests: GET shape with defaults and with layers; PUT writes the correct
   layer only; clearing a field restores the inherited value; malformed
   JSON → 422; bad scope → 400; options endpoint full and degraded modes.

## Verification

- `go test ./internal/server/ -run SettingsAPI -v` (and full suite) pass.

## Completion criteria

- API contract per plan, all handler tests green; offline options endpoint
  never errors to the client.

## Files affected

- `internal/server/settings.go` or `internal/server/settingsapi.go` (new)
- `internal/server/server.go` (routes)
- `internal/server/settings_test.go` or `settingsapi_test.go`

## Notes

- JSON only; the HTML page (SET-04) consumes these endpoints client-side.
- No authentication concerns beyond the existing local-only posture.
- Scope addition (found in SET-05 verification, folded into this task):
  save-time validation of submitted `session.agent`/`session.model`
  (`validateSessionSettings`). The live service accepts unknown values at
  creation without an error and then never runs the session, so the
  spawn-time 400 fallback cannot fire for typos; PUT now rejects unknown
  agent/model with 422 when the service is reachable, and skips validation
  when it is not. List calls are location-scoped (`ListAgentsFor`/
  `ListModelsFor` with `?location[directory]=`) so project-defined agents
  and providers count; the options endpoint uses the same scoping.
- Verified 2026-09-13: handler tests (GET shape, scoped PUT,
  clear-restores-inheritance, 400/422 rejects, options live + degraded,
  validation accept/reject/skip) plus live re-test on a throwaway repo:
  unknown agent → 422, unknown model → 422, valid pair → 200.
