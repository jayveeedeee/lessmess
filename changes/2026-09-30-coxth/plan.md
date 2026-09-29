# 2026-09-30-coxth: Fix chat history dropping assistant messages

- Change ID: 2026-09-30-coxth
- Created: 2026-09-30
- Branch: —
- Status: tracked in the tool-owned JSON state (.lessmess/workflow/)

## Objective and context

Clicking "Load older messages" at the top of a chat shows the user's own
older messages but none of the assistant's responses. The history loader's
duplicate-suppression logic is wrong for messages that render as more than
one transcript block, which is the normal shape of every assistant turn in
a coding session.

## Current behavior

- `makeChatTranscriptBlocks` (`internal/server/chat.go`) splits one
  assistant message into sibling transcript blocks that all share the same
  message ID in `data-chat-source` and in hidden `data-message` marker
  spans (`web/templates/partials.html`): reasoning/tool parts become a
  collapsed activity block, text parts become message blocks with the
  visible prose.
- `loadOlderChat` (`web/static/app.js`) builds an `existing` map from every
  `data-message` marker in the DOM, skips any fetched block whose source
  IDs are all in `existing`, and mutates `existing` as blocks are kept.
- For an assistant turn that starts with reasoning or a tool call, the
  activity block is kept first and marks the message ID as existing; the
  sibling block holding the model's text is then treated as a duplicate
  and dropped. Text-first assistant messages lose their second and later
  text parts the same way. User messages always render as exactly one
  block, so they always survive — hence "only my own messages".
- A second defect capped reachable history at one page past the initial
  snapshot: `loadOlderChat` picked its insertion point with
  `root.querySelector("[data-chat-block], .chat-interaction")`, which
  matches descendants in document order — after the first click the
  match lives inside the `.chat-history-page` wrapper, and `insertBefore`
  with a non-child reference throws `NotFoundError` on every subsequent
  click.

## Target behavior

- After loading older messages, every block of an assistant message
  (activity rows and text prose alike) renders.
- Whole-block duplicates — a block whose message markers were already in
  the DOM before the fetch — are still skipped, preserving the overlap
  defense the check was written for.
- Duplicate hidden marker spans inside partially overlapping blocks are
  still stripped so message IDs stay unique in the DOM.

## Scope

- `web/static/app.js`, `loadOlderChat` only: the block-skip check consults
  a frozen snapshot of the markers that existed before the fetch; markers
  added while placing this page's blocks go into a separate `seen` set
  used solely to strip duplicate hidden markers.

## Non-goals

- No server-side block identity keys (`data-chat-block-key`) — rejected as
  unnecessary for this bug; revisit only if genuine page overlap ever
  needs exact block-level dedupe.
- No changes to the snapshot template, Go code, polling, or the history
  cursor flow.
- No JS test harness.

## Design decisions

- Client-only fix: the bug is fully explained by the in-loop mutation of
  the dedupe set, so freezing the pre-existing set is the minimal correct
  change. Server-side keys would touch Go, template, and JS for no
  additional user-visible benefit.
- The frozen set (not the growing set) drives the block-level skip so
  sibling blocks of one message are never mistaken for duplicates of each
  other.

## Acceptance criteria

- In a session with more than 50 messages, clicking "Load older messages"
  shows assistant prose (text blocks) interleaved with user messages, not
  only user messages plus bare collapsed activity rows.
- Repeated clicks keep paging older until the control disappears at the
  end of history.
- Snapshot polls while history is loaded neither duplicate nor drop
  history rows.
- `go vet ./...` and `go test ./...` still pass (no Go changes expected),
  and `lessmess validate` reports no new violations.

## Tasks

1. (task breakdown is maintained by the tool; see the board)
