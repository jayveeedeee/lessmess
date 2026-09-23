<!-- tasktracker:begin -->
# Structure: internal/server

<!-- tasktracker-meta: refreshed=2026-09-23 source=manual tree=b42321c17a15 -->

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
| `chat.go` | Chat session surface: snapshot, tool detail, diffs, prompt send with attachments and references, controls, interrupt, and permission and form replies. |
| `chat_test.go` | Tests for snapshot rendering, attachments and reference aliases, file limits, controls, mutations, and error mapping. |
| `closepipeline.go` | Gated close pipeline for worktree-backed changes: clean gate, push, PR create-or-reuse, unattended reviewer session, and PR comment, with typed failures that block close. |
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
| `instructions.go` | Prime-composition engine: loads and validates the embedded instruction-module manifest and renders each session's prime from modules selected by audience and change state. |
| `instructions.json` | Embedded manifest of versioned instruction modules (discussion, change.session, change.handoff, worktree, task.session, closeout) composed into session primes. |
| `instructions_test.go` | Tests for manifest validity, deterministic audience selection, placeholder and snapshot rendering, and the instructions endpoint. |
| `integrations.go` | OpenCode integration endpoints: provider connection listing and detail, key-based connect, OAuth attempt lifecycle, and credential connect and disconnect actions. |
| `integrations_test.go` | Tests for secret redaction, mutation guards and stale credentials, and the transient OAuth attempt lifecycle. |
| `lifecycle.go` | Close, reopen, commit, and commit-status handlers |
| `lifecycle_test.go` | Tests for close/reopen and commit endpoints |
| `mapping.go` | Opencode session-to-change mapping and endpoints |
| `mapping_test.go` | Tests for session mapping and its endpoints |
| `mcp_permissions.go` | MCP server overview with connect and disconnect plus permission request overview and saved-rule removal. |
| `mcp_permissions_test.go` | Tests for MCP overview mapping and reconnect failures and permission overview with strict management guards. |
| `onboarding.go` | First-run onboarding state in `.lessmess/onboarding.json`: completed/dismissed flags and per-step outcomes, read fail-open. |
| `onboarding_test.go` | Tests for onboarding roundtrip, fail-open parsing, and pending detection. |
| `opencodestatus.go` | OpenCode service status collection, management page and API, and service rediscovery. |
| `opencodestatus_test.go` | Tests for status redaction and unavailable states, rediscovery origin guards, and management security headers. |
| `opendefault.go` | Reports the repository `opencode.json` `default_agent` in the settings payload and serves the align endpoint that repoints it at the effective session agent via a byte-preserving plain-JSON patch. |
| `opendefault_test.go` | Tests for default-agent status reporting, the byte-preserving patch, align-endpoint validation, and the settings response and page hooks. |
| `prereqs.go` | Wizard prerequisite probes (opencode binary and service, git, writable repo) served at GET /api/setup/prereqs, with injectable fakes. |
| `prereqs_test.go` | Tests for the prerequisite checks using faked probes. |
| `projectnamechrome_test.go` | Tests for the project name in the tab title, header brand, terminal head, and setup page, with the directory fallback and HTML escaping. |
| `render.go` | HTML templates, markdown rendering, static assets |
| `render_nested_test.go` | Tests for nested board markup, task-detail doc context, and the container ledger endpoint. |
| `render_test.go` | Tests for rendering and HTML pages |
| `server.go` | Server struct, routes, core handlers, SSE stream |
| `server_test.go` | Tests for core routes and handlers |
| `sessionlifecycle.go` | Session lifecycle endpoints: navigation, fork, staged revert, compact, queued delivery and inbox, children, rename, export, and delete confirmation. |
| `sessionlifecycle_test.go` | Tests for fork, revert and compact guards, inbox ordering, delete confirmation and mapping cleanup, and export behavior. |
| `settings.go` | Layered project and personal settings schema with stateless load, per-field merge, and atomic single-layer writes. |
| `settings_test.go` | Tests for settings layering, merge precedence, patches, fail-open loads, and model ref splitting. |
| `settingsapi.go` | Settings page plus GET/PUT JSON API with live agent and model validation and an opencode options proxy. |
| `settingsapi_test.go` | Tests for settings API reads, writes, validation, and degraded options responses. |
| `settingschange.go` | Settings-page Change button: POST /api/settings/change starts or reuses a settings-primed discussion. |
| `settingschange_test.go` | Tests for the settings Change flow: fresh, reused, and rejected discussions. |
| `settingsgeneral_test.go` | Tests for the general settings section: directory-basename fallback, project over personal layering, patch rejection, and the field allowlist. |
| `settingswiring_test.go` | Tests for settings wiring into session spawn defaults, prompt addenda, branch recording, the gardener gate, and archived filtering. |
| `setup.go` | First-run setup wizard: serves the wizard page and /api/setup/* endpoints when the repo has no changes/ tree, then hot-swaps to the full server after bootstrap. |
| `setup_test.go` | Tests for the setup shell, its wizard page, and route guards. |
| `setupseed.go` | Opt-in wizard docs-seed job: POST /api/setup/docs-seed runs one docs.Seed per repo, polled via GET /api/setup/docs-seed-status. |
| `setupseed_test.go` | Tests for the seed job lifecycle and configured agent/model pass-through. |
| `spawnfallback.go` | Persists de-escalated session spawns (`.lessmess/spawn-fallback.json`) behind the index banner, with fail-open reads and best-effort writes. |
| `spawnfallback_test.go` | Tests for the spawn de-escalation ladder outcomes, the record lifecycle and fail-open reads, and the index fallback banner. |
| `taskstate.go` | Deterministic task-state endpoints (task status, update, reorder, decisions) with workflow rules enforced server-side and the user-only Done transition gated on the X-Lessmess-UI header. |
| `taskstate_test.go` | Tests for the task-state endpoints: status transitions with evidence, user-gated Done, update, reorder, and decisions. |
| `touched.go` | Derives covered docs dirs touched by a change |
| `validatedocs_test.go` | Tests for docs findings in the validate endpoint |
| `worktree.go` | Worktree-backed changes: docs-root resolution from worktree state plus git confirmation, scaffold-time branch and worktree setup with rollback and dirt warning, and typed git-mechanics errors. |
| `worktree_test.go` | Tests for worktree setup, docs-root resolution, and the close pipeline against a real git repository. |
<!-- tasktracker:end -->
