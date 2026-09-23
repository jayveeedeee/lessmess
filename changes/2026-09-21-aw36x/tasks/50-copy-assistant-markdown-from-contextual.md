# MAC-50: Copy assistant Markdown from contextual action

## Objective

Allow an assistant response to be selected and copied as Markdown source.

## Dependencies

- MAC-49.

## Scope

- Open a copy-only contextual menu when an assistant message is selected.
- Preserve original Markdown alongside safely rendered response HTML.
- Copy source Markdown rather than rendered plain text.
- Keep Fork and Revert restricted to user messages.

## Implementation steps

1. Carry raw text through Chat message and text-part views.
2. Render escaped hidden Markdown sources for actionable messages.
3. Prefer those sources in the clipboard handler and add assistant Copy markup.
4. Assert assistant lifecycle actions remain absent.

## Verification

- Focused/full tests, vet, JavaScript syntax, build, validation, and diff check.

## Completion criteria

Selecting assistant output offers Copy and places its original Markdown on the
clipboard without exposing Fork or Revert.

## Files affected

- `internal/server/chat.go`
- `internal/server/chat_test.go`
- `web/templates/partials.html`
- `web/static/app.js`

## Notes
