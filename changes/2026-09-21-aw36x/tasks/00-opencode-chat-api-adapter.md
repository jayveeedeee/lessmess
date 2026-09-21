# MAC-00: OpenCode chat API adapter

## Objective

Extend the thin OpenCode client with the minimum typed operations needed for a
structured chat, without changing existing session creation or terminal paths.

## Dependencies

None.

## Scope

- Session transcript messages and active/busy state.
- Prompt submission and interruption.
- Pending permission requests and permission replies.
- Pending forms and form replies.
- Common message parts needed by the UI, plus a safe representation for unknown
  part types.
- Focused `httptest` contract coverage for request paths, payloads, response
  envelopes, malformed responses, and OpenCode errors.

## Implementation steps

1. Add only the message, part, active-state, permission, and form types consumed
   by lessmess; use `json.RawMessage` at evolving union boundaries rather than
   reproducing the entire OpenCode schema.
2. Add client methods for transcript retrieval, active state, interrupt,
   permission list/reply, and form list/reply. Keep all requests behind the
   existing authenticated `Client.do` path.
3. Make unknown message or field variants decodable so a newer OpenCode service
   cannot make the complete transcript unavailable.
4. Add table-driven fake-service tests for success, empty state, unknown parts,
   malformed data, and non-2xx errors. Preserve existing client behavior.

## Verification

- `go test ./internal/opencode`
- Existing create/list/prompt/wait client tests remain green.
- A fixture containing an unrecognized message part decodes without error and
  retains its type for downstream fallback rendering.

## Completion criteria

The server layer can retrieve and control a session conversation, including
blocking permissions and forms, without direct HTTP calls outside the OpenCode
client or exposing service credentials.

## Files affected

- `internal/opencode/client.go`
- `internal/opencode/client_test.go`
- `internal/opencode/AGENTS.md` if the package contract gains lasting behavior

## Notes

Do not add generated OpenAPI code or a second HTTP client abstraction. The V2
API is still evolving, so model the stable fields lessmess renders and fail open
at message-part union boundaries.
