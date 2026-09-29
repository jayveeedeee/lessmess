# HST-01: Fix insertBefore crash capping history at one page

## Why

After the first "Load older messages" click succeeds, every further click
fails with "Could not load older messages. Failed to execute 'insertBefore'
on 'Node': The node before which the new node is to be inserted is not a
child of this node", so reachable history is capped at the initial
snapshot plus one page. The insertion point selector
`root.querySelector("[data-chat-block], .chat-interaction")` matches in
document order: once a `.chat-history-page` wrapper exists, the first
`[data-chat-block]` it returns is a grandchild of `.chat-snapshot`, and
`insertBefore` with a non-child reference throws `NotFoundError`.

## What

In `web/static/app.js`:

- `loadOlderChat`: scope the insertion point to direct children and
  prefer the oldest existing history wrapper —
  `:scope > .chat-history-page, :scope > [data-chat-block], :scope > .chat-interaction`
  — so a new (older) page inserts before the first wrapper, keeping
  chronological order, and the reference is always a child of `root`.
- `pollChat`: apply the same scoped selector to its history-page
  re-insertion for robustness (its root is freshly swapped, so the old
  selector worked, but the scoped form cannot regress if that ordering
  ever changes).

## Files affected

- `web/static/app.js`

## Verification

- `go vet ./...` and `go test ./...` pass; `lessmess validate` clean.
- Rebuild + restart, hard-reload the board, open a >100-message session,
  click "Load older messages" three or more times: pages continue loading
  in chronological order with no error, until the control disappears at
  the true start of the conversation.
