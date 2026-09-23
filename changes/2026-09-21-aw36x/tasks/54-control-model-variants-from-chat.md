# MAC-54: Control model variants from Chat

## Objective

Allow Chat users to select the OpenCode model variant used by the active
session.

## Dependencies

- MAC-53.

## Scope

- Decode the variants advertised by each OpenCode model.
- Expose the current variant and selected model's variants in Chat Controls.
- Validate variant changes server-side and switch through the V2 model route.
- Reset to the model default when the selected model changes.

## Implementation steps

1. Extend the OpenCode model projection with variant metadata.
2. Add variant data and validation to Chat Controls.
3. Add a model-dependent Variant selector and mutation wiring.
4. Cover adapter, handler, and UI contracts and run full verification.

## Verification

- Adapter tests for model variant decoding and switching.
- Chat control tests for current, valid, default, and invalid variants.
- Full tests, vet, JavaScript syntax, build, validation, and diff check.

## Completion criteria

The active session's advertised model variants can be selected or reset to the
model default from Chat Controls.

## Files affected

- `internal/opencode/client.go`
- `internal/opencode/client_test.go`
- `internal/server/chat.go`
- `internal/server/chat_test.go`
- `internal/server/render_test.go`
- `web/templates/layout.html`
- `web/static/app.js`

## Notes

- OpenCode V2 represents reasoning profiles as model variants on `Model.Ref`,
  not as agent metadata; the selector therefore follows the active model.
- Chat Controls now lists the selected model's advertised variants and a
  Default option. Models without variants disable the selector.
- Variant changes reuse the session model switch endpoint with provider, model,
  and variant; changing models resets the variant to Default.
- The server re-reads model metadata and rejects variants that are unavailable
  for the selected model.
- Verified against the running service (`zai-coding-plan/glm-5.3-flash`
  advertises `low`, `high`, and `max`) plus adapter/handler/UI tests,
  `go vet ./...`, `go test ./...`, JavaScript syntax, static build, workflow
  validation, diff check, and served HTML/API checks.
