---
id: OCI-00
title: Verify opencode API contracts
---

# OCI-00: Verify opencode API contracts

Status: see [../ledger.md](../ledger.md).

## Objective

Pin down the exact request/response contracts this integration depends on, against the live opencode service and the OpenAPI spec, so implementation tasks build on verified shapes rather than assumptions.

## Dependencies

None.

## Scope

In scope: `POST /api/session` (payload + response), `POST /api/session/{id}/prompt` (payload shape), `POST /api/session/{id}/rename`, PTY flow (`POST /api/pty` payload for running `opencode2 --session <id>` in the repo dir, `connect-token`, WS `connect` handshake details), `GET /api/event` event types relevant to sessions/PTYs, and the published OpenAPI spec cross-check.
Out of scope: writing any production code.

## Implementation steps

1. Fetch the OpenAPI spec (`/openapi.json` from the live service, or https://opencode.ai/v2/openapi.json) and extract the schemas for the endpoints above.
2. Probe the live service (read-only where possible): create a throwaway session, rename it, fetch it, post a trivial prompt, inspect the message list shape.
3. Probe the PTY flow end-to-end enough to confirm: create → connect-token → WS connect handshake parameters (headers, query token, subprotocols).
4. Confirm `opencode2 --session <id>` attaches to an API-created session.
5. Record exact contracts (endpoints, payloads, example responses, quirks) in this task's notes. Delete the throwaway session afterward.

## Verification

- Notes contain the verified contract for every endpoint the client package will wrap.

## Completion criteria

- All contracts recorded; no open "unknowns" remain for OCI-01/OCI-02.

## Files affected

- None (research only); notes in this file.

## Notes

Verified 2026-09-12 against the live background service (auth: Basic `opencode` + `~/.config/opencode/service.json` password; discovery: `opencode2 service status` prints base URL):

- **OpenAPI**: full spec at `<svc>/openapi.json` (saved to /tmp/oc-openapi.json during research).
- **Create session**: `POST /api/session` `{"title", "location":{"directory":<abs path>}}` → `{"data":{"id":"ses_…","projectID":<repo commit hash>,"title","location":{"directory"},"tokens","time",…}}`. Optional fields: id, agent, model.
- **Get/List**: `GET /api/session/{id}` → `{"data":{…}}`; `GET /api/session` → `{"data":[…]}` (includes title, location.directory, tokens, time, model, agent).
- **Rename**: `POST /api/session/{id}/rename` `{"title"}` → 200 empty body; verified title updates.
- **Delete**: `DELETE /api/session/{id}` → 204.
- **Prompt**: `POST /api/session/{id}/prompt` body `{"text" (required), files?, agents?, skills?, metadata?, delivery?, resume?}` (from spec; not sent live).
- **Resume**: `opencode2 run --session <id> "msg"` works on API-created sessions (returned `resume-ok` in ~5s). **Crucial: without `--auto` the turn hangs indefinitely on a permission prompt** — confirms OCI-06 (pre-approved config) is required for unattended sessions.
- **PTY routes (pivot)**: `POST /api/pty {"command","args":[],"cwd","title"}` works (returns id, pid, status) — but `POST /api/pty/{id}/connect-token` returns **403 ForbiddenError "Invalid PTY connect token request" for every variant tried** (no params, location query, Origin header, JSON body) *including via the official `opencode2 api` CLI*. Conclusion: the token route is gated (likely for the hosted OpenCode Console), unusable for local integration. The `connect` WS endpoint takes a `ticket` query param (browser WS can't send headers; ticket-as-auth design) — moot after pivot.
- **Event stream**: `GET /api/event` yields `server.connected` + heartbeats (SSE). Optional for v1.
- **Decision (recorded in ledger)**: terminal layer pivots to an **in-process PTY** (tasktracker spawns `opencode2 --session <id>` via creack/pty and bridges it to its own WebSocket), dropping the service PTY routes entirely. The service API is used only for plain REST (session create/list/rename/prompt/delete), all verified working.
- Cleanup: all throwaway sessions/PTYs deleted (204s).

