# CHAT-03: Safely promote the same chat through existing scaffolding

## Why

The existing scaffold endpoint already preserves the session while binding it
to a change. Promotion needs a safe explicit entry into that flow and immediate
new workflow instructions, rather than leaving the session acting as a free
agent or accidentally creating duplicate changes.

## What

- Add an explicit server-side promotion entry for eligible project-owned,
  standalone user sessions. Validate confirmed title/prefix, mapping ownership,
  idle state, and stale confirmations before admission.
- Deliver a controlled user-approved scaffold-and-plan instruction and the
  `lessmess-scaffold` skill to the same session. Confirmation does not authorize
  implementation. Preserve manually typed requests to scaffold in chat too.
- Keep `POST /changes/scaffold` and its shared change-record creator as the sole
  scaffold flow; do not introduce a parallel implementation or copy history to
  a fresh session.
- Make duplicate promotion admission and mapping transition safe, including
  persistence failures and late naming. A retry cannot create a second change;
  already-bound sessions cannot be moved to an unrelated change.
- On success, immediately supply binding facts, authoritative state, change
  workflow rules, and worktree guidance using existing instruction mechanisms.
  Do not wait for compaction to replace obsolete unbound behavior.
- Retain actual directory/worktree semantics: the original session may remain
  rooted in the main tree and must use the scaffold response's worktree path
  correctly. Later change sessions follow the existing worktree behavior.
- Return actionable errors and preserve discoverability until binding succeeds;
  a change already created by the scaffold must not be silently rolled back.

## Files affected

- `internal/server/server.go` and a focused promotion handler/test file
- `internal/server/changesession.go` and `internal/server/changesession_test.go`
- `internal/server/mapping.go` and `internal/server/mapping_test.go`
- `internal/server/instructions.json` and `internal/server/instructions_test.go`
- `internal/server/reprime.go` and `internal/server/reprime_test.go` if sharing
  restoration machinery
- `internal/server/skills.json` and its tests only if the transition procedure
  needs clarification
- `internal/server/worktree_test.go` for retained-session guidance

## Verification

- Cover valid promotion with the original session ID/history; verify approved
  title/prefix, scaffold skill delivery, mapping move, renamed fallback title,
  and new instructions/current state.
- Cover unknown/unowned/helper/bound sessions, busy and stale confirmation,
  malformed fields, duplicate clicks/retries, upstream failure, and save failure.
- Confirm late automatic naming cannot overwrite the change title and failed
  promotion leaves the standalone chat available.
- Verify promotion authorizes scaffolding/planning only, typed scaffold requests
  still work, and retained worktree guidance is accurate with the setting on/off.
- Run focused server tests and workflow validation.
