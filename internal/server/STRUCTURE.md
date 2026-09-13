<!-- tasktracker:begin -->
# Structure: internal/server

<!-- tasktracker-meta: refreshed=2026-09-13 source=manual tree=e735f3b8cf4a -->

HTTP server over the store: HTML pages, JSON endpoints, SSE updates, session mapping, PTY terminal, and the docs refresh queue

## Entries

| Entry | Purpose |
| --- | --- |
| `changesession.go` | Discussion session and scaffold trigger endpoints |
| `changesession_test.go` | Tests for discussion and scaffold flows |
| `docsqueue.go` | Serialized doc-gardener refresh queue and stale tracking |
| `docsqueue_test.go` | Tests for docs queue, touched dirs, refresh |
| `docssession.go` | Doc gardener runner with confinement verify and restore |
| `docssession_test.go` | Tests for gardener confinement and prompts |
| `docswatch.go` | Fsnotify watcher emitting debounced events when covered docs change. |
| `docswatch_test.go` | Tests for watcher events, temp and hidden ignores, and resync. |
| `explorer.go` | Explorer page, tree fragment, and directory-scoped chat endpoints. |
| `explorer_test.go` | Tests for the explorer view, tree rendering, and chat endpoint. |
| `gitcommit.go` | Read-only git status helper plus repo-wide commit-all endpoints and prompt. |
| `gitcommit_test.go` | Tests for git status parsing and the repo-wide commit-all flow. |
| `lifecycle.go` | Close, reopen, commit, and commit-status handlers |
| `lifecycle_test.go` | Tests for close/reopen and commit endpoints |
| `mapping.go` | Opencode session-to-change mapping and endpoints |
| `mapping_test.go` | Tests for session mapping and its endpoints |
| `render.go` | HTML templates, markdown rendering, static assets |
| `render_test.go` | Tests for rendering and HTML pages |
| `server.go` | Server struct, routes, core handlers, SSE stream |
| `server_test.go` | Tests for core routes and handlers |
| `settings.go` | Layered project and personal settings schema with stateless load, per-field merge, and atomic single-layer writes. |
| `settings_test.go` | Tests for settings layering, merge precedence, patches, fail-open loads, and model ref splitting. |
| `settingsapi.go` | Settings page plus GET/PUT JSON API with live agent and model validation and an opencode options proxy. |
| `settingsapi_test.go` | Tests for settings API reads, writes, validation, and degraded options responses. |
| `settingschange.go` | Settings-page Change button: POST /api/settings/change starts or reuses a settings-primed discussion. |
| `settingschange_test.go` | Tests for the settings Change flow: fresh, reused, and rejected discussions. |
| `settingswiring_test.go` | Tests for settings wiring into session spawn defaults, prompt addenda, branch recording, the gardener gate, and archived filtering. |
| `terminal.go` | WebSocket-to-PTY bridge running the opencode TUI |
| `terminal_test.go` | Tests for the terminal WebSocket bridge |
| `touched.go` | Derives covered docs dirs touched by a change |
| `tuiconfig.go` | Merges the user's opencode CLI config with chrome-free overrides for embedded TUI sessions. |
| `tuiconfig_test.go` | Tests for TUI config merging, JSONC fallback, and XDG env replacement. |
| `validatedocs_test.go` | Tests for docs findings in the validate endpoint |
<!-- tasktracker:end -->
