# MAC-40: Visible context usage and interactive detail modals

## Objective

Show accurate active-context usage in Chat and restore interaction with task and
plan detail modals opened above Chat.

## Dependencies

- MAC-39.

## Scope

- Read active context from OpenCode's `GET /api/session/{sessionID}/context`
  rather than treating cumulative session tokens as current context.
- Show compact context usage in the Chat header and detailed usage in Controls.
- Refresh usage without adding high-frequency upstream fanout.
- Fix modal isolation so a nested `#detail` remains interactive while its page
  siblings and Chat stay inert.
- Restore close, Escape, backdrop, and compact Contents disclosure behavior.

## Implementation steps

1. Add a typed OpenCode active-context client method and token usage shape.
2. Derive current usage from the latest assistant request in active context.
3. Add a lightweight Chat usage endpoint and periodic header refresh.
4. Correct nested detail inert ownership and cleanup.
5. Add API, render, JavaScript, and regression contracts.

## Verification

- Focused/full Go tests, vet, JavaScript syntax, build, validation, live usage
  response, detail interaction contracts, and diff check.

## Completion criteria

Chat displays current context usage, and task/plan details can be closed and
their compact Contents menu can be expanded while Chat is open.

## Files affected

- `internal/opencode/client.go`
- `internal/opencode/client_test.go`
- `internal/server/chat.go`
- `internal/server/chat_test.go`
- `internal/server/server.go`
- `web/templates/layout.html`
- `web/static/app.js`
- `web/static/app.css`

## Notes

Session token totals remain useful accounting data but are not a context-window
measurement because they accumulate across turns and compactions.
