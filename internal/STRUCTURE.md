<!-- tasktracker:begin -->
# Structure: internal

<!-- tasktracker-meta: refreshed=2026-09-21 source=2026-09-21-8235b tree=6e216e246791 -->

Core Go packages behind the tasktracker binary: workflow model, store, HTTP server, docs management, opencode client, and terminal sessions.

## Entries

| Entry | Purpose |
| --- | --- |
| `docs/` | Implements agentsdocs management: coverage config, per-folder STRUCTURE.md/AGENTS.md generation, seeding, summarization, validation, and refresh. |
| `gitops/` | Executes the mutating git and GitHub CLI mechanics behind the worktree-per-change pipeline: branches, worktrees, push, and PRs. |
| `model/` | Parses and serializes the markdown files of the `changes/` workflow defined in AGENTS.md. |
| `opencode/` | Minimal Go client for the opencode background service HTTP API. |
| `server/` | HTTP server over the store: HTML pages, JSON endpoints, SSE updates, session mapping, PTY terminal, and the docs refresh queue |
| `store/` | In-memory model of the changes/ tree with watching, validation, and safe atomic writes. |
| `terminal/` | Manages in-process PTYs running interactive TUI sessions bridged to browser WebSockets. |
<!-- tasktracker:end -->
