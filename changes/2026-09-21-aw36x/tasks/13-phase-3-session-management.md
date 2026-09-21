# MAC-13: Phase 3 session management

## Objective

Complete non-destructive and destructive session management from Chat.

## Dependencies

- MAC-12

## Scope

- Rename, export, delete, unlink, and clearly differentiated mapping behavior.
- Mobile-safe display of non-interactive shell operation output where exposed by a session action.

## Implementation steps

1. Add rename/export controls and download-safe export responses.
2. Separate unlink from OpenCode deletion in wording, routes, and confirmations.
3. Prevent deletion from leaving stale change/task mappings.
4. Render bounded shell-operation output as transcript content, not an interactive terminal.
5. Test parent deletion, mapped sessions, export errors, and repeated operations.

## Verification

- Server/mapping/UI tests and live rename, export, unlink, and delete checks.

## Completion criteria

Session ownership and destructive outcomes are explicit, mappings remain valid, and management works on mobile.

## Files affected

- `internal/opencode/`
- `internal/server/mapping.go`
- Chat/session handlers and UI

## Notes

Interactive shell/PTY remains Terminal-only.
