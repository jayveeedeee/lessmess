# 2026-09-13-2: Settings page with session defaults and prompt addenda

- Change ID: 2026-09-13-2
- Created: 2026-09-13
- Branch: —
- Status: see [ledger.md](ledger.md)

## Objective and context

lessmess currently hard-codes every operational default: the six agent-facing
prompts (discussion, change, commit, repo-commit, gardener, explorer), the
opencode agent/model used for spawned sessions (always the service default),
and assorted behavior toggles (docs gardener on close, archived rows in the
index, terminal auto-open). Users want a **Settings page**, linked from the
right side of the top menu, to configure these defaults and to adjust the
core prompts.

Settings are **project configuration shared with everyone using the
repository**, so they must be committed and versioned like any other project
file. Some settings (agent/model above all) are also genuinely personal — a
teammate may lack the provider credentials — so a personal, gitignored
override layer sits on top of the committed file.

## Current behavior

- Prompts are pure functions in `internal/server` (`changesession.go`,
  `lifecycle.go`, `gitcommit.go`, `docssession.go`, `explorer.go`) with no
  customization hook.
- Sessions are created via `opencode.Client.CreateSession(title, directory)`
  with no agent/model selection; the opencode service default applies.
- `store.CreateChange` writes `—` into the root ledger's Branch column.
- `closeChange` always runs the best-effort docs hook; the index always lists
  every root-ledger row (including archived ones); the web client always
  auto-opens the terminal after spawning a session.
- No settings storage exists. `.lessmess/` (gitignored) holds tooling state
  only; `agentsdocs.json` is the precedent for a committed root config file.

## Target behavior

A committed **`lessmess.json`** at the repo root holds project settings; a
gitignored **`.lessmess/settings.json`** holds personal overrides. Effective
settings = built-in defaults ← project file ← personal file, merged per
field with source tracking. A `GET /settings` page (top menu, right side)
edits either layer; the server applies effective settings live, with no
restart:

| Section | Setting | Effect |
| --- | --- | --- |
| session | agent | Passed at opencode session creation (`POST /api/session` `agent` field) for every lessmess-spawned session |
| session | model | Same, as `Model.Ref` (`providerID` + `id`); empty = service default |
| session | autoOpenTerminal | Web client opens the terminal overlay after spawning a session (default true) |
| prompts | discussion, change, commit, repoCommit, gardener, explorer | Free text **appended** to the corresponding built-in prompt; base prompts never change |
| git | defaultBranch | Written into the root ledger Branch column for newly created changes |
| ui | showArchived | Index lists archived changes (default true = current behavior) |
| docs | autoGardenerOnClose | `closeChange` runs the docs-refresh hook (default true) |

Agent/model choices apply only to sessions created after the setting is
saved — never retroactively.

## Scope

- Layered settings storage: types, defaults, load, per-field merge with
  source tracking, atomic per-layer save, fail-open on malformed files.
- opencode client: `ModelRef`, creation-time agent/model
  (`CreateSessionWith`, with a plain-retry fallback on 400), `ListAgents` /
  `ListModels` for dropdown data.
- Server wiring: effective settings on `Server`; agent/model at all six
  session creation points (discussion, change, explorer chat, per-change
  commit, repo commit, gardener); prompt addenda in all six prompt builders;
  `git.defaultBranch` in `store.CreateChange` (signature gains `branch`);
  docs-gardener and show-archived gates.
- HTTP API: `GET /api/settings` (effective + both layers + sources),
  `PUT /api/settings?scope=project|personal` (partial write; empty clears
  the field from that layer), `GET /api/settings/options` (agents + models
  proxied from the service; 200 with `available:false` when offline).
- UI: `settings.html`, right-aligned Settings link in `layout.html`'s
  header, grouped left navigation (explorer selection idiom), scope toggle
  (Project / Personal), per-field source badges, agent/model datalists
  (primary agents only), prompt-addendum textareas, tri-state toggles
  (Inherit / On / Off), per-setting Change buttons that start or reuse a
  settings discussion (`POST /api/settings/change`),
  `autoOpenTerminal` honored in `app.js`.
- README settings section.

## Non-goals

- **No full prompt replacement.** Append-only addenda: the built-in prompts
  (and their scaffold/binding triggers) cannot be broken by an edit.
- **No retroactive application** of agent/model to existing sessions; no
  session-switch endpoints in v1.
- **No change-directory naming configuration.** The `changes/YYYY-MM-DD-N/`
  convention is pinned by the AGENTS.md validation contract and stays fixed.
- **No landing-page setting.** Redirecting `/` would make the change list
  unreachable from the top nav; revisit with a dedicated route if wanted.
- **No model variants** (`Model.Ref.variant`) and no per-role agent/model
  overrides (e.g. gardener on a cheaper model) — possible follow-ups.
- **No git branch creation** — `git.defaultBranch` is informational in the
  root ledger only.
- **No `lessmess init` / `lessmess validate` changes.** `lessmess.json` is
  created lazily on the first project-scope save; the settings page surfaces
  load errors itself.
- **No secrets in settings.** Both files hold names/toggles/prompt text only.

## Design decisions

- **Committed `lessmess.json` + gitignored `.lessmess/settings.json`.**
  Project policy is shared via git (the user's explicit requirement;
  `agentsdocs.json` precedent); personal prefs override per machine. Merge
  is per leaf field, personal wins, with a `sources` map (default / project
  / personal) exposed by the API so the UI can badge each field.
- **Creation-time agent/model, not post-create switches.** The opencode V2
  API (`POST /api/session`, verified against `/v2/openapi.json`) accepts
  `agent` (string) and `model` (`{id, providerID}`) in the create body — one
  call, no extra round trips. On a 400 (stale/unknown agent or model) the
  client logs a warning and retries a plain create, so a bad setting never
  blocks session creation.
- **Save-time validation (found during verification).** The live service
  *accepts* unknown agents/models at creation without an error and then
  never runs the session — no 400 means the retry fallback cannot fire.
  So `PUT /api/settings` validates submitted agent/model values against the
  live service (primary, non-hidden agents; available models; both scoped
  to the served repository via `?location[directory]=` so project-defined
  agents/providers count) and rejects unknown values with 422. When the
  service cannot be queried, values save unvalidated (offline stays
  possible; datalists guide the common path).
- **Agent/model pickers are datalist comboboxes** (live suggestions + free
  text), not `<select>` dropdowns — same UX online, graceful free text
  offline without control swapping.
- **Model stored as one `providerID/id` string** (opencode config
  convention), split on the *first* `/` only — model IDs themselves contain
  slashes (e.g. `fireworks-ai/accounts/fireworks/models/deepseek-v4p1-flash`).
- **Append-only prompt addenda.** Each builder's output gains
  `"\n\n" + addendum` when non-empty; builders stay pure functions, call
  sites pass the effective addendum. The gardener runner (no `Server`
  access) gets an addendum-provider closure from `SetOpencode`.
- **Tri-state booleans** (`*bool` in files: unset / true / false) so
  "inherit" is distinguishable from an explicit value; effective view
  materializes concrete defaults.
- **Fail open everywhere.** Malformed file → defaults + warning log +
  `loadError` surfaced in `GET /api/settings` and on the page; saves are
  atomic via `model.WriteFileAtomic`.
- **Additive store change.** `store.CreateChange` gains a `branch`
  parameter (empty = `—`); both server call sites and store tests updated.
- **Dropdowns filter to `mode == "primary"` agents** — subagent-mode agents
  are not valid session drivers.

## File-level impact

- `internal/server/settings.go`, `internal/server/settings_test.go` (new)
- `internal/opencode/client.go`, `internal/opencode/client_test.go`
- `internal/server/server.go` (settings store, routes, index filter)
- `internal/server/changesession.go`, `lifecycle.go`, `gitcommit.go`,
  `explorer.go`, `docssession.go` (addenda + `CreateSessionWith` + gates)
- `internal/store/store.go`, `internal/store/store_test.go`
  (`CreateChange` branch parameter)
- `web/templates/settings.html` (new), `web/templates/layout.html` (nav),
  `web/static/app.js` (settings page + auto-open), `web/static/app.css`
- `README.md`
- Runtime files: `lessmess.json` (committed, created on first project save),
  `.lessmess/settings.json` (gitignored tooling state)

## Data, API, and configuration changes

- New committed root config `lessmess.json`; new gitignored
  `.lessmess/settings.json`. Same JSON schema, all fields optional.
- New endpoints: `GET /settings`, `GET /api/settings`,
  `PUT /api/settings?scope=`, `GET /api/settings/options`.
- `store.CreateChange(title, prefix, date)` →
  `CreateChange(title, prefix, branch, date)`.
- opencode session create bodies may now include `agent` and `model`.
- No `changes/` workflow format changes.

## Safety, security, and rollback

- Settings writes are confined to `lessmess.json` and `.lessmess/` (atomic).
- No credentials in settings; the service password stays server-side; the
  options endpoint proxies agent/model lists without exposing auth.
- Prompt addenda cannot remove workflow safeguards from base prompts.
- Rollback: revert the code; both settings files are inert without it.

## Testing and verification strategy

- Unit: load/merge/sources/fail-open/atomic save; model-string split;
  prompt builders with addenda; opencode client new methods (httptest);
  CreateChange branch; index archived filter; gardener gate.
- Handler tests: settings GET/PUT (scope, clear-on-empty, 422 malformed),
  options endpoint offline degradation.
- `go vet ./... && go test ./...`, `lessmess validate`.
- Live smoke: save at both scopes, spawn a session with non-default
  agent/model, verify an addendum reaches a prompt, toggle gardener gate.

## Observability requirements

- Warning logs on settings load failure, session-create fallback retry, and
  save failure; `loadError` string in the settings API/page.

## Rollout sequence

Storage + client first (independent), then server wiring, then API, then
UI, then end-to-end verification.

## Risks and mitigations

- **Stale agent/model in a committed file breaks on another machine** →
  plain-retry fallback on 400; personal override layer exists exactly for
  this.
- **Layered config confuses users** → per-field source badges and an
  explicit scope toggle; PUT only touches the selected layer.
- **Addendum injection weakens prompts** → append-only; base text immutable.

## Acceptance criteria

1. `GET /settings` renders from the top-menu Settings link; every listed
   setting is viewable with its source (default/project/personal).
2. Saving with scope=project writes `lessmess.json` (shows in git status);
   scope=personal writes `.lessmess/settings.json` (gitignored); personal
   wins in the effective view.
3. A newly spawned session (any of the six flows) uses the configured
   agent/model; unknown values are rejected with 422 at save time when the
   service is reachable (the service accepts them silently at creation and
   the session never runs), and the 400-retry fallback covers services
   that do reject at creation.
4. A non-empty addendum appears verbatim at the end of its prompt; empty
   addenda leave prompts byte-identical to today.
5. New changes record `git.defaultBranch` in the root ledger Branch column.
6. Toggles work: gardener on close, archived rows in index, terminal
   auto-open.
7. Malformed `lessmess.json` → defaults plus visible warning, no crash.
8. `go vet ./... && go test ./...` and `lessmess validate` pass; README
   documents the page and both files.

## Tasks

1. [SET-00](tasks/00-settings-storage-and-merge.md) — Settings storage, layering, and merge
2. [SET-01](tasks/01-opencode-session-defaults.md) — opencode client session defaults support
3. [SET-02](tasks/02-server-settings-wiring.md) — Wire settings into server behavior
4. [SET-03](tasks/03-settings-http-api.md) — Settings HTTP API
5. [SET-04](tasks/04-settings-page-ui.md) — Settings page UI and client wiring
6. [SET-05](tasks/05-end-to-end-verification.md) — End-to-end verification and docs
7. [SET-06](tasks/06-settings-page-grouped-nav.md) — Settings page grouped left navigation
8. [SET-07](tasks/07-settings-nav-explorer-style.md) — Settings nav in explorer selection style
9. [SET-08](tasks/08-per-setting-change-button.md) — Per-setting Change button with session reuse
