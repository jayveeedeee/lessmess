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

## Current behavior

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

## Target behavior

- New tri-state setting **`docs.enabled`** (Inherit/On/Off, **default Off**) at
  the top of the Settings Docs section, badged Experimental. Effective-on
  requires both the setting and a readable `agentsdocs.json`; the server builds
  the docs queue/watcher only then, and every existing nil-guard keeps its
  behavior. Toggling takes effect on restart (help text says so).
- **Initialize coverage from Settings**: with docs enabled but no
  `agentsdocs.json`, the Docs section offers an "Initialize coverage" action
  that writes the default config via the existing `docs.InitWithOptions`
  machinery; the exclusions editor shows a hint until config exists. The UI
  alone can turn docs on.
- **Wizard is docs-free**: the bootstrap step loses the coverage checkbox and
  exclusion picker; the docs step, its two endpoints, and the `docs-coverage`
  prereq are removed → 5-step wizard (`prereqs, name, bootstrap, agent,
  finish`).
- **`lessmess init` stops writing `agentsdocs.json`** — docs is opt-in
  everywhere; only the settings toggle plus coverage config enables it.
- Disabled-by-setting surfaces are worded distinctly from missing-config ones
  and point at Settings → Docs (Explorer disabled box, handler 503 messages).
- **Worktrees**: behavior unchanged (still off by default); the
  `git.worktrees` field gains the Experimental badge and a help-text note.
- New reusable `.exp-badge` chip in `app.css`, modeled on `.src-badge`.
- Dogfood: this repo's committed `lessmess.json` sets `"docs": {"enabled":
  true}` so the local gardener keeps running after the default flips.

## Scope

- `internal/server/settings.go` (+ `settingsapi.go`, `settingschange.go`):
  schema, merge, validation, per-field allowlist for `docs.enabled`.
- `internal/server/server.go`: gate docs queue/watcher construction on the
  effective setting; reword disabled messages.
- `internal/server/explorer.go` + `web/templates/explorer.html`: disabled
  wording → Settings → Docs.
- `internal/server/setup.go` (+ `prereqs.go`, `setupseed.go`): drop docs
  bootstrap payload fields, docs step endpoints, `docs-coverage` prereq.
- `internal/docs/init.go` + `cmd/lessmess/main.go`: `init` no longer writes
  `agentsdocs.json`; `docs seed` message updated to mention Settings.
- New settings endpoint to initialize docs coverage (thin wrapper over
  `docs.InitWithOptions`).
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
  user's back: the explicit "Initialize coverage" action does that, reusing
  `docs.InitWithOptions` (same machinery the wizard bootstrap used).
- `lessmess init` stops writing the coverage file: docs is fully opt-in, and
  onboarding artifacts should not exist for an off-by-default feature.
- The wizard loses docs entirely rather than hiding it conditionally — the
  feature is settings-only now.
- The experimental badge is one shared CSS class used by both features rather
  than per-feature styling.

## Acceptance criteria

- Fresh repo, default settings: no docs processes, no docs UI affordances
  (bell stays hidden, Explorer shows the disabled box pointing at Settings),
  wizard shows 5 steps with no docs content, `lessmess init` writes no
  `agentsdocs.json`.
- Settings → Docs: `docs.enabled` tri-state field with Experimental badge;
  turning it On (and restarting) enables docs when config exists; with no
  config, "Initialize coverage" writes a default `agentsdocs.json` and the
  subsystem comes up on restart.
- `git.worktrees` field carries the Experimental badge; behavior unchanged.
- `go vet ./... && go test ./...` green, including render tests for the
  wizard/settings shapes; `lessmess validate` clean.

## Tasks

1. (task breakdown is maintained by the tool; see the board)
