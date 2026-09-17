# 2026-09-17-rny74: Spawn fallback hardening and default_agent alignment

- Change ID: 2026-09-17-rny74
- Created: 2026-09-17
- Branch: —
- Status: see [ledger.md](ledger.md)

## Objective and context

lessmess applies the configured `session.agent`/`session.model` when it spawns
opencode sessions. Two gaps make the configuration silently lose to opencode's
own defaults, observed live in the tevr-core repository where the user selects
the `build` agent but change sessions kept landing on `kg-orchestrator`:

1. **Fallback drops the agent.** When the service rejects the configured
   agent/model combination with 400, `spawnSessionWithModel` retries with
   *both* dropped. opencode then applies the repository's own
   `default_agent` (tevr-core: `kg-orchestrator`). A stale model name
   therefore costs the user their agent choice. This produced the bound
   change session of tevr-core change `2026-09-16-esl9l` (agent
   `kg-orchestrator` while discussions seconds earlier got `build`).
2. **The fallback is invisible.** The only signal is a server-log warning;
   the user sees the wrong agent in the UI with no explanation.
3. **Divergence from opencode's own default is invisible.** Sessions created
   outside lessmess (user-run opencode TUI/CLI) follow `opencode.json`'s
   `default_agent`. Nothing tells the user when it differs from the agent
   they selected in lessmess.

## Current behavior

- `internal/server/settings.go` (`spawnSessionWithModel`, lines 467–492):
  one create attempt with `eff.Session.Agent` + model ref; on a 400
  `opencode.APIError` it logs a warning and retries via plain
  `oc.CreateSession` (no agent, no model). All server spawn sites funnel
  through this function (`s.spawnSession`), plus the docs runner via
  `server.go` line 147.
- No persisted record of a fallback exists; no UI surface reports it.
- The Settings page (`settingsapi.go`, `web/templates/settings.html`) is
  unaware of the served repository's `opencode.json` `default_agent`.
- The opencode service honors an explicit `agent` on `POST /api/session`
  (verified live against tevr-core's service: create with `agent=build` +
  model `zai-coding-plan/glm-5.3-flash` → 200, agent `build`).

## Target behavior

1. **Agent-preserving fallback ladder.** On a 400 rejection, de-escalate in
   this order, stopping at the first success: agent+model → agent only →
   model only → plain. Each de-escalation logs a warning naming the step and
   the service error.
2. **Persisted fallback record + board banner.** A fallback writes
   `.lessmess/spawn-fallback.json` (atomic): timestamp, attempted agent and
   model, the outcome step that succeeded (`agent-only` | `model-only` |
   `plain`), and the service error message. The board index renders a
   warning banner from this record. The next fully clean spawn (agent+model
   accepted, or nothing configured) clears the record.
3. **default_agent divergence notice + align button.** The Settings page
   reads the repository's `opencode.json`; when it declares a `default_agent`
   different from the effective `session.agent`, it renders an advisory
   ("sessions created outside lessmess will use X") plus an explicit
   **Align** button. The button patches only the `default_agent` key of
   `opencode.json` in place, preserving all other content and key order,
   after validating the agent against the live service.

## Scope

- `spawnSessionWithModel` fallback ladder and its warnings
  (`internal/server/settings.go`).
- New spawn-fallback state file write/clear (`internal/server`, following the
  one-file-per-feature `.lessmess/` convention, atomic writes via
  `model.WriteFileAtomic`).
- Board index banner rendering (`internal/server/render.go` or `indexView`
  inputs, `web/templates/index.html`).
- Settings response/page additions for `default_agent` divergence
  (`internal/server/settingsapi.go`, `web/templates/settings.html`).
- New align endpoint patching `default_agent` in the repository's
  `opencode.json` (read-modify-write, order-preserving, JSONC-refusing).

## Non-goals

- No silent/automatic sync of `default_agent` — only the explicit button.
- No management of global opencode config (`~/.config/opencode/...`).
- No change to settings layering (project `lessmess.json` / personal
  `.lessmess/settings.json`).
- No edits to other repositories' configs from this change; tevr-core's
  `opencode.json` is adjusted by the user (or via the shipped align button
  running against tevr-core), not by this repository's implementation work.
- No change to which agent the embedded terminal uses (it attaches to an
  existing session via `opencode2 --session <id>`).

## Design decisions

- **Ladder order agent-first.** The agent is identity and behavior; the model
  is cost/quality. A bad model must not cost the agent, and vice versa;
  plain create stays the last resort so a bad setting never blocks spawning
  (preserving the existing guarantee).
- **One state file per feature.** `.lessmess/spawn-fallback.json`, written
  atomically; absent file = no banner. Cleared (file removed or emptied) on
  the next clean spawn. No dismissal state — the record is server truth and
  self-heals.
- **Banner is server-rendered** from the record in the index view, consistent
  with `GitDirty`-style render-time inputs; no new read endpoint.
- **Align is explicit and validated.** The button reuses the settings-save
  validation rules (agent must be a known primary, non-hidden agent for the
  served directory; 422 otherwise; 503 when the opencode service is
  unreachable and validation cannot run).
- **Preserve the file, not just the JSON.** `opencode.json` is committed in
  projects; the patcher keeps original key order and only replaces or appends
  the `default_agent` value. Files with JSONC features (comments, trailing
  commas) are refused with a clear message rather than rewritten, since Go's
  stdlib JSON cannot round-trip them.
- **Last-writer-wins is acceptable** for the align write (same concurrency
  stance as `applySettingsPatch`); documented, no locking introduced.

## Data / API changes

- New state file: `.lessmess/spawn-fallback.json` (gitignored via the
  existing `.lessmess/` ignore).
- New endpoint: `POST /api/settings/opencode-default-agent` — aligns
  `default_agent` to the effective `session.agent`; responses report old/new
  values; 422 on invalid agent or unpatchable file, 503 without opencode.
- `GET /api/settings` response gains `opencodeDefaultAgent` info (declared
  value, file path, parse status) so the page can render the notice; the
  settings HTML template renders the advisory and button server-side.

## Safety, security, and rollback

- The align endpoint writes only inside the served repository root and only
  the `default_agent` key; refusals leave the file untouched (write via
  `model.WriteFileAtomic`).
- No migration; the state file is inert to older binaries and safely absent.
- Rollback: rebuild the previous binary; leftover state file is harmless and
  can be deleted.

## Testing and verification strategy

- Unit tests with a fake opencode client that 400s configurable combinations:
  ladder reaches agent-only, model-only, and plain outcomes; warnings logged;
  record written/cleared at the right steps.
- Index HTML renders the banner when the record exists and not when absent.
- Settings response/page: advisory present on divergence, absent on match or
  missing file.
- Align endpoint: golden test that only `default_agent` changes and key order
  is preserved; JSONC refusal; invalid agent 422; 503 without opencode.
- Repo checks: `go vet ./... && go test ./...`; `lessmess validate`
  unaffected.

## Observability

- `slog` warnings on each de-escalation step (agent, model, plain) with the
  service error, replacing today's single warn.
- The persisted record is the user-visible trace; the banner shows what was
  attempted, what won, and why.

## Rollout sequence

1. SPF-00 fallback ladder (behavior fix, no UI dependency).
2. SPF-01 record + banner (visibility).
3. SPF-02 divergence notice (read-only).
4. SPF-03 align endpoint + button (write path).
Templates are embedded — rebuild and restart the server to see UI changes.

## Risks and mitigations

- **Extra create attempts on persistent 400s** (up to 4 per spawn): only on
  the rejection path; creation is cheap and the ladder stops at first
  success.
- **`opencode.json` semantics drift** (key name changes in opencode): the
  notice simply disappears when the key is absent; align writes the same key
  tevr-core already uses today.
- **JSONC false positives** (a `//` inside a string value): the scanner must
  be string-aware; worst case is a false refusal with a clear message, never
  corruption.

## Acceptance criteria

1. With a service that rejects agent+model but accepts agent-only, a spawned
   session has the configured agent and no model; a warning is logged and the
   board shows the fallback banner; the next clean spawn clears it.
2. Ladder never leaves the agent behind while the model alone was the
   problem, and never blocks spawning (plain retry preserved).
3. Settings page shows the divergence advisory exactly when `opencode.json`
   declares a `default_agent` different from the effective `session.agent`.
4. Align patches only `default_agent`, preserves all other content and key
   order, refuses JSONC safely, and validates the agent first.
5. `go vet ./...` and `go test ./...` pass.

## Tasks

1. [SPF-00](tasks/00-spawn-fallback-ladder.md) — Agent-preserving spawn
   fallback ladder in `spawnSessionWithModel`.
2. [SPF-01](tasks/01-fallback-record-banner.md) — Persisted fallback record
   and board warning banner.
3. [SPF-02](tasks/02-default-agent-notice.md) — `default_agent` divergence
   notice on the Settings page.
4. [SPF-03](tasks/03-align-endpoint.md) — Align endpoint and button patching
   `opencode.json`'s `default_agent`.
