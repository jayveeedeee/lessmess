# MAC-05: Phase 2 attachments and references

## Objective

Let mobile users send and inspect the attachments and project references used in normal coding prompts.

## Dependencies

- MAC-04

## Scope

- File/image attachment selection, upload encoding, preview, removal, and submission.
- Existing-message attachment and reference rendering with safe open/download behavior.
- Mobile size/type validation and explicit failures.

## Implementation steps

1. Add explicit server request types and limits for supported OpenCode attachments.
2. Add an accessible mobile picker and draft attachment tray.
3. Render attachment/reference parts with safe filenames, media handling, and fallbacks.
4. Test traversal names, unsupported types, oversize input, retry, and transcript reload.

## Verification

- Focused Go/render tests and a real-service image plus text-file prompt from a phone viewport.

## Completion criteria

A user can compose, review, send, reopen, and download supported attachments without Terminal.

## Files affected

- `internal/opencode/`
- `internal/server/chat.go`
- `web/templates/`
- `web/static/app.js`
- `web/static/app.css`

## Notes

Keep temporary data bounded and avoid persisting attachment contents in lessmess state.
