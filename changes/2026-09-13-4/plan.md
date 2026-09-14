# 2026-09-13-4: Onboarding wizard

- Change ID: 2026-09-13-4
- Created: 2026-09-13
- Branch: —
- Status: see [ledger.md](ledger.md)

## Objective and context

Give lessmess a first-run onboarding wizard so a user can point `lessmess serve` at a blank or existing repo that has no workflow setup yet and be walked through: system prerequisite checks, repo bootstrap, choosing a default opencode agent/model, and an explicitly opt-in initial docs generation. Completion is persisted so the wizard never nags again.

Today none of this exists: the server refuses to start without `changes/`, prerequisites are discovered only via log warnings or runtime failures, and one docs path (`docs seed`) ignores the configured agent/model entirely.

Decided with the user on 2026-09-13 (discussion session `ses_f65a9621dffewgb7rCMTWI231t`):

1. The wizard lives in the **web UI**; the server gains a setup mode (CLI `init` stays as-is).
2. It appears **once per repo**, persisted in `.lessmess/onboarding.json`, re-openable from Settings.
3. Initial docs generation is an **explicit opt-in** wizard step (default: skip).
4. A missing/unreachable opencode service is handled by **detect + instructions + re-check** — the wizard never tries to start the service itself.

## Current behavior

- `runServe` (`cmd/lessmess/main.go:82`) calls `store.Open`, which fails with `changes/ directory not found under <dir>` (`internal/store/store.go:75`) and the process exits 1. There is no UI at all on an uninitialized repo; the only path is CLI `lessmess init` first.
- `docs.Init` (`internal/docs/init.go`) bootstraps merge-safely and idempotently: workflow AGENTS.md, `changes/ledger.md` skeleton, `.gitignore` (`.lessmess/`), starter `opencode.json`, default `agentsdocs.json`. Always writes the coverage config; no option to skip it.
- Prerequisites are real but unchecked in one place:
  - `opencode2` binary on PATH — used by `opencode.Discover` (`opencode2 service status`) and the embedded terminal PTY spawn (`opencode2 --session <id>`).
  - opencode background service reachable + password in `~/.config/opencode/service.json` — otherwise every session endpoint 503s.
  - `git` binary + work tree — `gitcommit.go` already degrades to `repo:false`.
- Docs auto-run triggers: `docs seed` CLI (manual only, but `OpenCodeSummarizer` uses plain `CreateSession` and **ignores `session.agent`/`session.model`**, unlike the server's `spawnSession`); gardener on change close (gated by `docs.autoGardenerOnClose`, default true — cannot fire on a fresh repo); `POST /docs/refresh` (on fresh coverage every dir is stale, so one click runs gardener sessions for the whole tree).
- Agent/model defaults already exist as layered settings (`session.agent`, `session.model`), validated at save time against the live repo-scoped service lists, with `GET /api/settings/options` providing the choices and a 400-retry fallback at spawn.
- No onboarding concept exists anywhere in the codebase.

## Target behavior

- `lessmess serve` on a repo without `changes/` starts in **setup mode** instead of exiting: it serves the wizard page, static assets, the settings API (needed for the agent step), and `/api/setup/*`; every other route returns 503 (JSON) or redirects to `/` (HTML).
- Wizard flow:
  1. **Prerequisites** — per-check status (ok/warn/fail) with detail and remediation hints; a Re-check button re-runs; checks that gate later steps (opencode2 binary, service health, repo writable) must pass to continue.
  2. **Bootstrap** — runs `docs.Init` server-side with a separate **docs-coverage choice** (default: enabled) controlling whether `agentsdocs.json` is written, plus an **exclusion picker**: a lazily loaded directory tree (top-level picks become base-name patterns matching same-named dirs at any depth; nested picks become exact path patterns) whose selection is written into the config — and updated on later submissions. Reports per-artifact created/merged/skipped.
  3. **Default agent + model** — dropdowns from the live service (`/api/settings/options`), saved via `PUT /api/settings` to the **personal** layer by default (agent availability is machine-specific), project layer selectable; skippable (service defaults).
  4. **Docs seeding — explicit opt-in** — only shown when coverage is enabled; "Generate docs now" runs a budgeted, asynchronous server-side seed honoring the chosen agent/model, with a cost warning and live progress; skip means nothing runs, ever.
  5. **Finish** — onboarding marked complete; redirect to the normal UI.
- On bootstrap the server **hot-opens** the store in-process (no restart): watchers, docs queue/watcher, and the opencode client attach exactly as a normal start would.
- On initialized repos with incomplete onboarding, the index shows a dismissible banner linking to `/setup`; the Settings page gains a "Re-run onboarding" link.
- The wizard renders as a card centered vertically and horizontally on the page.
- Docs seed (server-side and CLI) honors `session.agent`/`session.model`.

## Scope

- `store.Open` missing-`changes/` error becomes a detectable sentinel.
- New setup-mode server shell + hot-open in `internal/server` and `cmd/lessmess`.
- New `/api/setup/*` endpoints (prereqs, bootstrap, docs-seed + status, complete, dismiss) and a `GET /setup` page, registered in both setup mode and the normal server.
- `docs.Init` gains an options variant (skip coverage config).
- Docs seed honoring agent/model (server-side seed + CLI parity).
- Wizard UI: new template, client logic in `app.js`, styles in `app.css`; index banner; Settings re-entry link.
- `.lessmess/onboarding.json` tooling-state file.
- Tests for all of the above; README update.

## Non-goals

- Interactive CLI wizard (CLI `init`/`docs seed` stay non-interactive).
- Auto-starting the opencode service (detect + instructions only).
- Changes to workflow file formats, doc markers, or the changes/ contract.
- Forced redirects or blocking modals on already-initialized repos (banner is dismissible).
- Repairing a malformed existing `changes/` tree (stays a hard startup error).
- TUI/chrome config, gardener behavior, and explorer changes.
- Moving the settings subsystem to its own package (only a thin exported helper is added).

## Design decisions

1. **Setup mode is a separate small mux**, not `Server` with a nil store — `s.st` is dereferenced throughout; a dedicated `setupServer` keeps the change minimal and safe. Only the new `store.ErrNoChanges` sentinel triggers it; a malformed root ledger stays fatal.
2. **Hot-open via a swappable root handler.** `main.go` defines a `boot` closure that runs the exact existing serve wiring (open store → `Watch` → `server.New` → `SetOpencode` → `PublicBase`) and returns the full handler; the setup server calls it once on successful bootstrap and atomically swaps. Swap-once guarded; shutdown closes whichever child is active.
3. **One registrar for setup routes on both muxes.** `registerSetupRoutes(mux, env)` is used by the setup mux and by `Server.New`, so `/setup` and `/api/setup/*` work identically in setup mode, after hot-open, and on initialized repos (Settings re-entry). `env` supplies the repo dir and an opencode-client accessor.
4. **Onboarding state** lives in `.lessmess/onboarding.json` (one-file-per-feature tooling state): `{version, completedAt, dismissed, steps:{prereqs,bootstrap,agent,docs}}`. Missing/malformed file reads fail-open as incomplete; writes are atomic via `model.WriteFileAtomic`. Pending = no `completedAt` and not dismissed.
5. **Prereq checks use injectable function vars** (same test pattern as `SpawnCommand`): binary lookup, service discovery, git probe. Response: `{checks:[{id,name,status,detail,remedy}], ready}`; `ready` requires no `fail` checks.
6. **Bootstrap options:** `docs.InitWithOptions(root, InitOptions{Config bool})`; existing `docs.Init(root)` delegates with all steps enabled, so the CLI and current tests are untouched.
7. **Seed honors agent/model:** the seed `SessionClient` interface gains `CreateSessionWith` (already implemented by `opencode.Client`); a `NewOpenCodeSummarizerWith(c, wait, agent, model)` constructor variant carries the defaults while the old constructor delegates with empty values. The CLI reads the effective values via a new exported `server.SessionDefaults(dir)` helper — no settings-package refactor. No 400-retry in the seed path: values were validated at save time and a rejection surfaces as a clear per-dir failure.
8. **Async server-side seed** with one in-flight job: `POST` starts it (409 if running, 503 without oc/coverage), progress lines from the Seed `io.Writer` collect into an in-memory buffer, and a `GET` status poller (`{running, lines, error}`) follows the existing commitStatus UI pattern. The resumable `.lessmess/docs-seed.json` cursor is unchanged.
9. **Personal layer is the default** for the agent/model choice (machine-specific availability), with a project-layer radio for teams that want it committed.
10. **Docs coverage vs docs generation are separate choices**: enabling coverage at bootstrap only writes `agentsdocs.json`; generation happens solely via the opt-in step (or later manual action).

## Implementation approach

Ordered per the task breakdown (ONB-00 … ONB-07); dependencies in [ledger.md](ledger.md). Foundations first (state file, setup shell, sentinel), then endpoints (prereqs, bootstrap, seed), then UI (wizard, banner), then docs/verification.

## File-level impact

- `internal/store/store.go` — sentinel error for missing `changes/` (message preserved).
- `cmd/lessmess/main.go` — setup-mode branch in `runServe` with boot closure; `runDocsSeed` passes agent/model.
- `internal/server/onboarding.go` (+test) — state file helpers. **(new)**
- `internal/server/setup.go` (+test) — setup server shell, route registrar, handler swap. **(new)**
- `internal/server/prereqs.go` (+test) — prereq checks + endpoint. **(new)**
- `internal/server/setupseed.go` (+test) — async docs-seed job + status. **(new)**
- `internal/server/server.go` — register setup routes; index view gains onboarding-pending flag.
- `internal/server/settings.go` — export `SessionDefaults`; minor refactor so settings handlers can serve the setup mux (repo dir without a store).
- `internal/server/render.go` — `setup` template set.
- `internal/docs/init.go` — `InitWithOptions`/`InitOptions`.
- `internal/docs/summarize.go` — `CreateSessionWith` on `SessionClient`; summarizer variant.
- `web/templates/setup.html` — wizard page. **(new)**
- `web/templates/index.html` — onboarding banner; `web/templates/settings.html` — re-entry link.
- `web/static/app.js`, `web/static/app.css` — wizard flow, banner, styles.
- `README.md` — first-run onboarding documentation.

## Data, API, message, configuration, or schema changes

- New tooling-state file `.lessmess/onboarding.json` (gitignored, schema-versioned).
- New endpoints: `GET /setup`; `GET /api/setup/prereqs`; `GET /api/setup/dirs`; `POST /api/setup/bootstrap` `{docsCoverage:bool, excludeDirs:[string]}`; `POST /api/setup/docs-seed` `{budget:int}`; `GET /api/setup/docs-seed-status`; `POST /api/setup/complete`; `POST /api/setup/dismiss`.
- New exported API: `store.ErrNoChanges`, `docs.InitWithOptions`/`InitOptions`, `docs.NewOpenCodeSummarizerWith`, `server.SessionDefaults`.
- `docs.SessionClient` interface extended (internal, fakes updated).
- Index view model gains an onboarding-pending flag.

## Safety, security, rate-limit, migration, and rollback considerations

- Server binds localhost only (unchanged). Bootstrap writes only the merge-safe init artifacts, atomically.
- The seed endpoint is the only LLM-budget consumer: explicit opt-in, budget field, one in-flight job, resumable cursor; failures roll back per dir (existing `summarizeOne` confinement).
- Setup mode denies all non-setup routes; no changes/ data is touched until bootstrap creates it.
- No migration needed; `onboarding.json` is created lazily and absent means incomplete.
- Rollback: rebuild the previous binary. Init artifacts are ordinary files; no format changes.

## Testing and verification strategy

- Unit: sentinel detection; `InitWithOptions` skip-config; onboarding roundtrip + fail-open; prereqs across faked ok/warn/fail paths; bootstrap endpoint on a temp dir including hot-open callback fired once; seed endpoint with a fake `SessionClient` capturing agent/model + status transitions; setup page and banner render tests.
- Repo gates: `go vet ./... && go test ./...`; rebuild with `CGO_ENABLED=0 go build -o lessmess ./cmd/lessmess`.
- Manual E2E: (a) empty temp dir → wizard → bootstrap → agent → skip docs → finish → board usable, restart → straight to normal UI; (b) this repo → banner appears → dismiss → gone; (c) docs opt-in on a scratch repo with a small budget.

## Observability requirements

`slog` lines in existing style: setup-mode entry (dir), prereq failures (check id + detail), bootstrap actions, hot-open success/failure, seed start/finish with dir counts, onboarding complete/dismiss.

## Rollout sequence

Single binary rebuild; no coordination. Existing repos see the banner once; dismissal persists.

## Risks and mitigations

- **Double-started watchers/docs queue on hot-open** → single boot closure reused from `runServe`, swap-once guard, `Close` covers the active child.
- **Banner surprises existing users** → one-time dismiss persisted in `onboarding.json`.
- **Setup endpoints on the normal server re-running init** → init is idempotent/merge-safe by design; seed still requires coverage + service (503 otherwise).
- **`SessionClient` interface break** → `opencode.Client` already implements `CreateSessionWith`; test fakes updated in the same change.
- **Wizard usable but service appears midway** → Re-check re-discovers and swaps the client in setup mode.

## Acceptance criteria

1. `lessmess serve` on an empty dir serves the wizard instead of exiting 1; all normal routes 503/redirect until bootstrap.
2. `/api/setup/prereqs` reports accurate per-check status with remediation; Re-check works.
3. Bootstrap creates the init artifacts merge-safely (coverage optional) and the store hot-opens without a process restart.
4. The agent step saves a validated agent/model to the chosen layer; skip keeps service defaults.
5. The docs step skipped = zero LLM sessions; opted-in = budgeted seed runs honoring the configured agent/model with visible progress.
6. Completion persists; the banner never reappears; Settings re-entry reopens the wizard.
7. `docs seed` CLI honors the configured agent/model.
8. `go vet ./... && go test ./...` pass; binary rebuilt; manual E2E scenarios verified.

## Tasks

1. [ONB-00](tasks/00-onboarding-state-file.md) — Onboarding state file helpers (`.lessmess/onboarding.json`)
2. [ONB-01](tasks/01-setup-mode-server.md) — Setup-mode server shell with hot-open
3. [ONB-02](tasks/02-prereq-checks-endpoint.md) — Prerequisite checks endpoint
4. [ONB-03](tasks/03-bootstrap-endpoint.md) — Bootstrap endpoint with docs-coverage option
5. [ONB-04](tasks/04-docs-seed-endpoint.md) — Server-side docs seed honoring agent/model (+ CLI parity)
6. [ONB-05](tasks/05-wizard-ui.md) — Wizard UI (template, client flow, styles)
7. [ONB-06](tasks/06-onboarding-banner.md) — Index banner + Settings re-entry
8. [ONB-07](tasks/07-readme-and-verification.md) — README + full verification
