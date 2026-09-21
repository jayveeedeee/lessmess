# MAC-10: Phase 3 fork revert and compaction

## Objective

Add mobile controls for branching conversation history, reverting work, and compacting context.

## Dependencies

- MAC-09

## Scope

- Fork at a selected message and map/open the resulting session correctly.
- Stage, preview, commit, and cancel revert.
- Manual compaction with progress and failure recovery.

## Implementation steps

1. Add message-level fork actions and bind the child session without losing parent context.
2. Build a staged revert preview with explicit affected range/files and destructive confirmation.
3. Add compaction control gated by session state and context information.
4. Test cancel paths, double submissions, stale previews, reconnect, and mapping consistency.

## Verification

- Handler/UI tests plus real-service fork, revert-cancel, revert-commit, and compaction checks.

## Completion criteria

All three operations are understandable and recoverable on mobile, with no ambiguous destructive action.

## Files affected

- Session lifecycle handlers and tests
- Chat templates, JS, and CSS
- Session mapping code where required

## Notes

Revert must remain a staged operation; never collapse preview and commit into one tap.
