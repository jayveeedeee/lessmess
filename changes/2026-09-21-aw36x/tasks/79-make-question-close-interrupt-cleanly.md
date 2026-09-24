# MAC-79: Make question Close interrupt cleanly

## Objective

Make closing a pending question interrupt the active response cleanly, without
misreporting an intentional stop as a provider failure.

## Dependencies

- MAC-78

## Scope

- Route the question Close action through the existing session interrupt API.
- Keep Close dismissive: do not submit form answers or alter the change.
- Show interruption-specific transient status while the request is sent.
- Do not infer a provider failure from a bare assistant `finish: error` value.
- Continue surfacing sanitized structured provider errors.

## Implementation steps

1. Invoke the interrupt mutation when dismissing a composer-hosted form.
2. Give interrupt mutations accurate status text.
3. Remove the unsupported provider diagnosis for an unstructured error finish.
4. Add controller and transcript regression contracts.

## Verification

- Run focused chat controller and transcript tests.
- Run full tests, vet, JavaScript syntax, validation, build, and served checks.

## Completion criteria

Question Close stops active processing like Stop, and intentional interruption
does not produce the generic provider-connection failure message.

## Files affected

- `web/static/app.js`
- `internal/server/chat.go`
- `internal/server/chat_test.go`
- `internal/server/render_test.go`

## Notes
