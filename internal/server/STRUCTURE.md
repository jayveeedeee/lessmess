<!-- tasktracker:begin -->
# Structure: internal/server

<!-- tasktracker-meta: refreshed=2026-09-17 source=2026-09-17-4esfh tree=a92805fc3c7a -->

HTTP server over the store: HTML pages, JSON endpoints, SSE updates, session mapping, PTY terminal, and the docs refresh queue

## Entries

| Entry | Purpose |
| --- | --- |
| `accent.go` | Accent color palette: entries, lookup, and resolution with roll-once random initialization persisted to the personal settings layer. |
| `accent_test.go` | Tests for the palette, roll-once resolution and persistence, fail-open ids, and accent validation, options, and allowlist wiring. |
| `autosession.go` | Once-only auto-spawn markers (`.lessmess/autosession.json`) for decomposed-task sessions. |
| `autosession_test.go` | Tests for auto-spawn-once semantics, retry after spawn failure, and the task session endpoints. |
| `board_nested_test.go` | Tests for nested task boards: drill-down, recursive counts, close gates, and subtask creation. |
| `bootstrap_test.go` | Tests the full bootstrap loop against a real store, asserting hot-open after bootstrap. |
| `brandassets.go` | Serves `/icon.svg`, `/favicon.ico`, and `/apple-touch-icon.png` rendered with the effective accent, with ETag and 304 revalidation. |
| `brandassets_test.go` | Tests for rendered brand bytes (SVG fill, ICO and PNG magic, tile size) and the accent-following brand routes. |
| `changesession.go` | Discussion session and scaffold trigger endpoints |
| `changesession_test.go` | Tests for discussion and scaffold flows |
| `docsqueue.go` | Serialized doc-gardener refresh queue and stale tracking |
| `docsqueue_test.go` | Tests for docs queue, touched dirs, refresh |
| `docsseed.go` | Normal-server docs seed endpoints (POST /docs/seed, GET /docs/seed-status) plus GET/POST /docs/exclusions for the settings editor. |
| `docsseed_test.go` | Tests for the seed endpoints (503s, 409, resume, force) and the exclusions round-trip. |
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
| `onboarding.go` | First-run onboarding state in `.lessmess/onboarding.json`: completed/dismissed flags and per-step outcomes, read fail-open. |
| `onboarding_test.go` | Tests for onboarding roundtrip, fail-open parsing, and pending detection. |
| `prereqs.go` | Wizard prerequisite probes (opencode binary and service, git, writable repo) served at GET /api/setup/prereqs, with injectable fakes. |
| `prereqs_test.go` | Tests for the prerequisite checks using faked probes. |
| `projectnamechrome_test.go` | — |
| `render.go` | HTML templates, markdown rendering, static assets |
| `render_nested_test.go` | Tests for nested board markup, task-detail doc context, and the container ledger endpoint. |
| `render_test.go` | Tests for rendering and HTML pages |
| `server.go` | Server struct, routes, core handlers, SSE stream |
| `server_test.go` | Tests for core routes and handlers |
| `settings.go` | Layered project and personal settings schema with stateless load, per-field merge, and atomic single-layer writes. |
| `settings_test.go` | Tests for settings layering, merge precedence, patches, fail-open loads, and model ref splitting. |
| `settingsapi.go` | Settings page plus GET/PUT JSON API with live agent and model validation and an opencode options proxy. |
| `settingsapi_test.go` | Tests for settings API reads, writes, validation, and degraded options responses. |
| `settingschange.go` | Settings-page Change button: POST /api/settings/change starts or reuses a settings-primed discussion. |
| `settingschange_test.go` | Tests for the settings Change flow: fresh, reused, and rejected discussions. |
| `settingsgeneral_test.go` | — |
| `settingswiring_test.go` | Tests for settings wiring into session spawn defaults, prompt addenda, branch recording, the gardener gate, and archived filtering. |
| `setup.go` | First-run setup wizard: serves the wizard page and /api/setup/* endpoints when the repo has no changes/ tree, then hot-swaps to the full server after bootstrap. |
| `setup_test.go` | Tests for the setup shell, its wizard page, and route guards. |
| `setupseed.go` | Opt-in wizard docs-seed job: POST /api/setup/docs-seed runs one docs.Seed per repo, polled via GET /api/setup/docs-seed-status. |
| `setupseed_test.go` | Tests for the seed job lifecycle and configured agent/model pass-through. |
| `terminal.go` | WebSocket-to-PTY bridge running the opencode TUI |
| `terminal_test.go` | Tests for the terminal WebSocket bridge |
| `touched.go` | Derives covered docs dirs touched by a change |
| `tuiconfig.go` | Merges the user's opencode CLI config with chrome-free overrides for embedded TUI sessions. |
| `tuiconfig_test.go` | Tests for TUI config merging, JSONC fallback, and XDG env replacement. |
| `validatedocs_test.go` | Tests for docs findings in the validate endpoint |
<!-- tasktracker:end -->
