
# ONB-05: Wizard UI (template, client flow, styles)

Status: see [../ledger.md](../ledger.md).

## Objective

The actual wizard experience: a `/setup` page walking through prerequisites → bootstrap → default agent/model → docs opt-in → finish, driven client-side against the ONB-02/03/04 endpoints and the existing settings API.

## Dependencies

- ONB-02 (prereqs endpoint)
- ONB-03 (bootstrap endpoint)
- ONB-04 (docs-seed endpoint)

## Scope

- `web/templates/setup.html` (new): `content` template with five step sections; parsed with `layout.html` into a new `renderer.setup` set (same pattern as `settings.html`).
- `web/static/app.js`: `initSetup` — step state machine; prereq fetch/render + Re-check + continue gating (`ready` required); bootstrap POST with docs-coverage checkbox (default checked) and per-artifact result list; agent/model dropdowns from `GET /api/settings/options` with scope radio (personal default) and save via `PUT /api/settings` plus a "Use service defaults" skip; docs step (rendered only when coverage was enabled) with a budget input, cost warning text, "Generate docs now" (POST + poll `docs-seed-status`, render progress lines) and "Skip for now"; finish → `POST /api/setup/complete` → redirect to `/`.
- `web/static/app.css`: wizard layout, per-check status pills reusing existing status colors, progress log styling.
- Steps already satisfied resume sensibly on reload (e.g. bootstrap skipped ahead when `changes-present` is true); the page works identically in setup mode and on the normal server.
- All new DOM ids documented; htmx is available but plain fetch (as in `initSettings`/`initCommitAll`) is the established pattern for this kind of flow.

## Implementation steps

1. Add the template + renderer set + route render (replace ONB-01's placeholder).
2. Implement `initSetup` step by step, mirroring `initSettings`' structure (load → render → bind → save).
3. Style with existing tokens (`--st-*`, `.btn-accent`, `.btn-ghost`, `.pill`).
4. Add a render test for the page (assert step sections and stable ids exist).
5. Manual E2E on a temp dir: full flow both with docs opt-in (small budget) and skip; reload mid-wizard resumes; finish lands on a working index.

## Verification

- `go test ./internal/server -run 'SetupPage|Render' -v` passes; `go vet ./...` clean.
- Manual E2E scenarios above verified with the rebuilt binary; screenshots checked.

## Completion criteria

- A user can complete onboarding end-to-end in the browser on a blank repo without touching the CLI, and every gate/skip behaves per the plan.

## Files affected

- `web/templates/setup.html` (new)
- `web/static/app.js`, `web/static/app.css`
- `internal/server/render.go`, `internal/server/render_test.go`
- `internal/server/setup.go` (page render replaces placeholder)

## Notes

- Template-name uniqueness across the global set applies (see `web/templates/AGENTS.md`); define only `content` in `setup.html`.
- Rebuild the binary for UI verification — embedded assets are invisible to a running server (repo learning).
- Copy for the docs step: one opencode session per covered directory, bounded by the budget; already-summarized dirs are skipped on re-run.
