# AGENTS.md

<!-- tasktracker:begin -->
## Working in internal/terminal

- Purpose: manages in-process PTYs (via `github.com/creack/pty`) that run interactive TUIs such as `opencode2 --session <id>` for bridging to browser WebSockets.
- `PTY` wraps one running pseudo-terminal process; `Manager` tracks the live set for lookup and shutdown cleanup.
- `Manager.Spawn` starts a command in a new PTY at a given cwd and window size, returning a `PTY` with an auto-generated `pty-N` ID; use `Resize` to update dimensions.
- Always call `CloseAll` on server shutdown, and `PTY.Close` per instance, to terminate child processes and release file descriptors.
- `Manager` is mutex-protected for the map, but individual `PTY` reads/writes are not synchronized; coordinate concurrent access to a single PTY.
<!-- tasktracker:end -->
