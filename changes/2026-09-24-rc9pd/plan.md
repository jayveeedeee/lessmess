# 2026-09-24-rc9pd: Experimental settings-gated docs and worktrees

- Change ID: 2026-09-24-rc9pd
- Created: 2026-09-24
- Branch: —
- Status: tracked in the tool-owned JSON state (.lessmess/workflow/)

## Objective and context

The docs subsystem and the worktree pipeline are both power features that most
repositories never need, yet docs is pushed during onboarding (wizard bootstrap
checkbox, dedicated seed step, `lessmess init`) and nothing tells the user that
either feature is experimental. This change makes docs a settings-gated,
off-by-default feature that is turned on entirely from Settings (never the
wizard), and marks both docs and worktrees as experimental in the Settings UI.

## Previous behavior

- Docs enablement is purely file-presence: a root `agentsdocs.json` switches the
  whole subsystem on (`internal/server/server.go` builds the docs queue/watcher
  when `docs.LoadConfig` returns a config); without it every docs surface
  no-ops or 503s with "docs coverage disabled (no agentsdocs.json)".
- The setup wizard pushes docs: the bootstrap step carries a pre-checked
  "Enable agent-facing docs coverage" checkbox plus an exclusion picker, a
  dedicated docs-seed step follows (`POST /api/setup/docs-seed`,
  `GET /api/setup/docs-seed-status`), and prereqs include an informational
  `docs-coverage` check. `lessmess init` also writes a default
  `agentsdocs.json` unconditionally.
- Existing docs settings keys (`docs.autoGardenerOnClose`,
  `docs.gardenerModel`) tune the feature from inside; none gates it.
- Worktrees: `git.worktrees` is a tri-state Inherit/On/Off setting, default
  Off, with a single runtime check (`worktreesEnabled()`). Already opt-in —
  only the experimental labeling is missing.
- No "experimental" visual pattern exists anywhere in `web/`.

## Implemented behavior

- New tri-state setting **`docs.enabled`** (Inherit/On/Off, **default Off**) at
  the top of the Settings Docs section, badged Experimental. At process start,
  effective-on requires both the setting and a readable `agentsdocs.json`; the
  server builds the docs queue/watcher only then. Toggling runtime behavior
  takes effect on restart (help text says so), while Settings reads the saved
  value immediately.
- **Initialize coverage from Settings**: with docs enabled but no
  `agentsdocs.json`, the Docs section offers an "Initialize coverage" action
  that writes only the default coverage config through a narrow docs-package
  initializer; it must not re-run the broader workflow bootstrap. The
  exclusions editor shows a hint until config exists. After saving On, the user
  can initialize coverage immediately, then restart once to activate docs.
- **Wizard is docs-free**: the bootstrap step loses the coverage checkbox and
  exclusion picker; the docs step, its two endpoints, and the `docs-coverage`
  prereq are removed → 5-step wizard (`prereqs, name, bootstrap, agent,
  finish`).
- **`lessmess init` stops writing `agentsdocs.json`** — docs is opt-in
  everywhere; only the settings toggle plus coverage config enables it.
- Inactive surfaces distinguish setting-off, missing/unreadable config, and
  restart-required states and point at Settings → Docs (Explorer disabled box,
  handler responses).
- A small docs status API reports the saved setting, config presence/readability,
  current runtime activation, and whether a restart is needed. Settings uses it
  instead of conflating a missing config with an existing malformed file.
- `/api/validate` and CLI `validate` omit docs findings and missing-doc counts
  while docs is disabled, so the docs bell stays hidden even when a dormant
  `agentsdocs.json` remains in the repository. Explicit seed commands also
  respect the setting.
- **Worktrees**: behavior unchanged (still off by default); the
  `git.worktrees` field gains the Experimental badge and a help-text note.
- New reusable `.exp-badge` chip in `app.css`, modeled on `.src-badge`.
- Dogfood: this repo's committed `lessmess.json` sets `"docs": {"enabled":
  true}` so the local gardener keeps running after the default flips.

## Scope

- `internal/server/settings.go` (+ `settingsapi.go`, `settingschange.go`):
  schema, merge, exported read helper, and per-field allowlist for
  `docs.enabled`.
- `internal/server/server.go`: gate docs queue/watcher construction on the
  effective setting and expose coherent inactive-state wording.
- `internal/server/server.go`, `docsqueue.go`, `docsseed.go`: gate docs
  validation/pending counts, refresh, and seed on the active runtime.
- `internal/server/explorer.go` + `web/templates/explorer.html`: disabled
  wording → Settings → Docs.
- `internal/server/setup.go` (+ `prereqs.go`, `setupseed.go`): drop docs
  bootstrap payload fields, docs step endpoints, `docs-coverage` prereq.
- `internal/docs/init.go`: add a narrow, idempotent coverage-config initializer;
  the broader workflow `Init` no longer writes `agentsdocs.json`, and obsolete
  wizard-only init options are removed once their callers are gone.
- `cmd/lessmess/main.go`: `validate` and `docs seed` honor `docs.enabled`;
  `init` no longer writes `agentsdocs.json`.
- New normal-server status/initialize endpoints for docs coverage, without
  touching any other bootstrap artifact.
- `web/templates/settings.html`, `web/templates/setup.html`,
  `web/static/app.js` (`BOOL_DEFAULTS`, `initSettings`, `initSetup` wiring),
  `web/static/app.css` (`.exp-badge`).
- `lessmess.json` (this repo): `"docs": {"enabled": true}`.
- `README.md`: settings list, docs gating, init behavior, experimental notes.
- Tests: render tests (wizard steps, settings sections), settings API tests,
  unit tests for the new gating.

## Non-goals

- No hot-reload of the docs toggle (restart-applied, like today's config-file
  behavior).
- The Explorer link remains visible as the discoverable route into Settings;
  with docs inactive it renders guidance rather than a docs tree.
- No change to worktree lifecycle, close pipeline, or defaults.
- No change to `docs.autoGardenerOnClose` / `docs.gardenerModel` semantics.
- No migration path that silently enables docs for repos with an existing
  `agentsdocs.json` — upgrading repos must set `docs.enabled` (dogfooded here
  via the committed project layer).

## Design decisions

- `docs.enabled` default **Off** (user intent: "something you turn on in the
  settings"); tri-state Inherit/On/Off like `git.worktrees`, project policy in
  committed `lessmess.json`, personal override in `.lessmess/settings.json`.
- Turning the setting on does not fabricate `agentsdocs.json` behind the
  user's back: the explicit "Initialize coverage" action writes only that
  committed file through a dedicated idempotent primitive.
- `lessmess init` stops writing the coverage file: docs is fully opt-in, and
  onboarding artifacts should not exist for an off-by-default feature.
- The wizard loses docs entirely rather than hiding it conditionally — the
  feature is settings-only now.
- The experimental badge is one shared CSS class used by both features rather
  than per-feature styling.

## Acceptance criteria

- Fresh repo, default settings: no docs processes or docs findings, the bell
  stays hidden, Explorer shows inactive guidance pointing at Settings, the
  wizard shows 5 steps with no docs content, and `lessmess init` writes no
  `agentsdocs.json`.
- Settings → Docs: `docs.enabled` tri-state field with Experimental badge;
  with existing config, turning it On and restarting enables docs; with no
  config, save On → Initialize coverage → restart enables it. Initialization
  touches only `agentsdocs.json` and is idempotent.
- `git.worktrees` field carries the Experimental badge; behavior unchanged.
- `go vet ./... && go test ./...` green, including render tests for the
  wizard/settings shapes; `lessmess validate` has no workflow violations or
  new docs findings relative to the recorded baseline (the current checkout
  already has queue-stale warnings from a prior gardener 404 plus one unrelated
  stale-reference warning in `internal/registry/AGENTS.md`).

## Tasks

1. (task breakdown is maintained by the tool; see the board)
