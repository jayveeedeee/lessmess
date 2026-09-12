<!-- tasktracker:begin -->
# Structure: internal/terminal

<!-- tasktracker-meta: refreshed=2026-09-13 source=manual tree=2fb00d4ead90 -->

Manages in-process PTYs running interactive TUI sessions bridged to browser WebSockets.

## Entries

| Entry | Purpose |
| --- | --- |
| `terminal.go` | PTY wrapper and lifecycle manager |
| `terminal_test.go` | Tests for PTY spawning with explicit and inherited environments. |
<!-- tasktracker:end -->
