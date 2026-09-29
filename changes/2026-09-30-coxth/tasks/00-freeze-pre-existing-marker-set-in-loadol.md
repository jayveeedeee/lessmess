# HST-00: Freeze pre-existing marker set in loadOlderChat

## Why

Loading older chat history drops the assistant's responses: one assistant
message renders as several transcript blocks that share a single message
ID, and `loadOlderChat` skips any block whose source IDs are all in its
`existing` map — a map it grows as it keeps blocks. The activity block
(reasoning/tools) is kept first and poisons the set; the sibling block
holding the model's text is then discarded as a "duplicate". See
`../plan.md` for the full trace.

## What

In `web/static/app.js` (`loadOlderChat`):

- Keep the `existing` map as a frozen snapshot of the `data-message`
  markers present in the DOM before the fetch; the block-level skip
  (`source.every(id => existing[id])`) consults only this frozen set.
- Add a separate `seen` set that grows as this page's blocks are placed;
  use it (with `existing`) only to strip duplicate hidden marker spans
  inside partially overlapping blocks, keeping message IDs unique in the
  DOM.

No template, Go, cursor, or polling changes.

## Files affected

- `web/static/app.js`

## Verification

- `go vet ./...` and `go test ./...` pass (no Go change expected).
- `lessmess validate` clean.
- Rebuild (`CGO_ENABLED=0 go build -o lessmess ./cmd/lessmess`) and
  restart the server — assets are embedded — then in a session with more
  than 50 messages: click "Load older messages" and confirm assistant
  prose appears between user messages, repeated clicks page to the end of
  history, and polls neither duplicate nor drop loaded history rows.
