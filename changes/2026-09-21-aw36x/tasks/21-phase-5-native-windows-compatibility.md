# MAC-21: Phase 5 native Windows compatibility

## Objective

Make the API-driven lessmess workflow natively supported on Windows without requiring WSL.

## Dependencies

- MAC-20

## Scope

- Platform-aware OpenCode discovery/credentials, paths, atomic writes, watchers, Git worktrees, signals/process shutdown, and browser Chat.
- Terminal is disabled or labelled unsupported unless ConPTY is separately implemented.

## Implementation steps

1. Replace Unix-specific service credential assumptions with tested platform discovery.
2. Add Windows-native tests for state writes/replacement, fsnotify behavior, Git/worktree paths, and process lifecycle.
3. Make terminal capability explicit so unsupported PTY startup is never presented as working.
4. Run the complete Chat workflow against a native Windows OpenCode service and repository.
5. Document supported Windows versions, prerequisites, and filesystem recommendations.

## Verification

- Native Windows CI/tests plus recorded real-machine onboarding and Chat session evidence.

## Completion criteria

Windows users can initialize, serve, manage changes, and use Chat natively; unsupported Terminal behavior is clear and harmless.

## Files affected

- `internal/opencode/`
- `internal/terminal/` and server capability wiring
- Platform-specific tests
- README

## Notes

A cross-compiled `.exe` alone is not evidence of Windows support.
