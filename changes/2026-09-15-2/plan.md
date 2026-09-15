# 2026-09-15-2: Per-task subagent sessions

- Change ID: 2026-09-15-2
- Created: 2026-09-15
- Branch: —
- Status: see [ledger.md](ledger.md)

## Objective and context

A change session should be able to delegate a single task to an opencode subagent,
and the user should be able to find and talk to that subagent session directly from
the board. Today, subagent runs exist in the opencode service as real child sessions
(verified live: `Session.Info.parentID` is populated for task-tool spawns), but
lessmess neither maps them nor surfaces them — they are invisible unless the user
digs through the raw session list.

The user chose approach "B + scan fallback": an explicit bind endpoint driven by a
prompt convention, with an automatic `parentID`-based reconciler as the safety net,
covering the full stack (mapping, API, prompt, UI) in one change.

## Current behavior

- `.lessmess/sessions.json` (`internal/server/mapping.go`) maps session → change via
  `map[changeID][]SessionEntry{session, title, created}` plus the `_unassigned`
  bucket. No task or parent concept.
- Subagent children spawned by a change session (task tool) get `parentID` set in
  the service but are unknown to the mapping, so `changeOf(child)` returns nothing
  and the board never lists them.
- The embedded terminal spawns `opencode2 --session <id>` for any session ID, so
  "talk to a sub" needs no new attachment machinery — only discovery and UI.
- `POST /changes/scaffold` established the trust model: an agent-supplied session ID
  in the request body, guarded by mapping lookups (409 when already bound).
- `changePrompt` (`changesession.go`) primes change sessions; prompt edits affect
  only sessions created afterwards.

## Target behavior

- A change session that delegates task `TSK-NN` to a subagent titles the sub
  `TSK-NN: slug` and calls `POST /api/changes/{id}/task-sessions` to bind it.
- The server additionally reconciles: whenever a change's session list is served,
  one `ListSessions` call finds unmapped sessions whose `parentID` points at one of
  the change's bound sessions and maps them (task inferred from a `TSK-NN` title
  prefix; unmatched children map to the change without a task).
- The board shows each change session's sub sessions against their tasks, with a
  talk button that opens the existing terminal overlay on the sub's session ID.
- Mapping entries gain optional `task` and `parent` fields; all changes are additive
  and old mapping files load unchanged.

## Scope

- Mapping schema extension and task-scoped accessors (`mapping.go`).
- New bind endpoint with scaffold-style guards (`changesession.go` or `mapping.go`).
- `parentID` reconciler merged into the served session list (`mapping.go`).
- `changePrompt` delegation + bind instructions (`changesession.go`).
- Board UI: per-task sub list and talk button (`web/static/app.js`, templates).
- Unit tests for all of the above with the existing fake opencode client patterns.

## Non-goals

- No server-orchestrated per-task sessions (approach C rejected for now).
- No read-only transcript viewer; talking to a sub means the terminal overlay.
- No background watcher or polling loop for reconciliation — it rides the existing
  session-list fetch only.
- No `changes/` document ever records session IDs; the mapping stays tooling state.
- No retroactive priming of existing change sessions; they keep their old prompt.
- No enforcement of one-sub-per-task; the schema allows several.

## Design decisions

- **Approach B + scan fallback (user decision), amended by the PSB-00 spike.** The
  agent knows which task it delegated but, as the spike proved, can never supply
  the sub's session ID — so the reconciler is the primary mapper and the bind
  endpoint is corrective (user/UI-facing, caller session optional). The prompt
  instead teaches the title lever: child session title == task-tool description
  (verified), so prefixing descriptions with `TSK-NN: ` gives the reconciler the
  task association.
- **Storage stays in `.lessmess/sessions.json`.** A sub is still a session bound to
  the same change; additive `SessionEntry` fields (`task`, `parent`, both optional)
  preserve the one-file-per-feature convention and old-file compatibility.
- **Reconciler trigger is the served session list.** `GET /api/changes/{id}/sessions`
  (and the discussions list stays untouched) performs one extra `ListSessions` call,
  filters children by `parentID`, and merges. Bounded, no loops, fail-open: if the
  service is unreachable the stored list is served as today.
- **Guards mirror scaffold.** Bind requires the calling session to be mapped to the
  target change (409 otherwise, body names the conflicting change), the sub session
  to exist per live `GetSession` (422 otherwise), and the task ID to exist in the
  change's ledger rows (422 otherwise). Rebinding the same session to the same task
  is idempotent (200); rebinding to a different task is a 409.
- **Title convention `TSK-NN: slug`** is the reconciliation key for task inference;
  the prompt teaches it and the reconciler parses it. Unparseable children still map
  (change-wide, no task) so nothing is lost.
- **Prompt changes affect only new sessions**, per the standing rule; existing
  change sessions never learn the bind call, and the reconciler covers them.

## Detailed implementation approach

1. **PSB-00 (spike, go/no-go) — DONE.** Both questions answered live: children are
   directly promptable (yes), the parent model never sees the child session ID
   (no), and child title == task-tool description (confirmed). Findings in the task
   notes; consequences folded into the design decisions above.
2. **PSB-01 (mapping).** Add `Task`/`Parent` optional JSON fields to `SessionEntry`;
   add `listByTask(change, task)` and keep `changeOf` unchanged (a sub maps to the
   same change, which is exactly what the 409 guard needs). Tests: old-shaped files
   load; round-trip preserves new fields.
3. **PSB-02 (bind endpoint, corrective).** `POST /api/changes/{id}/task-sessions`
   with body `{task, sub, session?}`; validation order: change exists → mapping
   readable → task in ledger rows (422) → sub session live (422, captures its
   `parentID` and title) → when `session` is supplied it must be bound to the change
   (409, names its change) → idempotency (200 reused / 409 different task) → append
   entry with `Task`/`Parent` set. `slog.Info` on success, mirroring existing logs.
4. **PSB-03 (reconciler — primary mapper).** In the change-session list handler,
   after enrichment input is gathered: one `ListSessions`, filter sessions whose
   `parentID` is one of the change's bound sessions and which are not already
   mapped, parse `^([A-Z0-9]+-\d+):` from the title, map remaining children (task
   or blank), persist once. Purely additive merge; never removes entries. Requires
   exposing `parentID` on the `opencode.Session` struct (small client addition in
   PSB-01).
5. **PSB-04 (prompt, spike-amended).** Extend `changePrompt` with a delegation
   section: prefer doing the task inline for small work; for delegated work spawn
   the subagent via the task tool with the description prefixed `TSK-NN: slug`
   (child title == description, verified by PSB-00 — the reconciler reads it). No
   bind curl: the parent model cannot know the child session ID. Keep every
   existing pinned phrasing the tests assert.
6. **PSB-05 (UI).** Session list payload now carries `task`/`parent`; `app.js`
   groups sub entries under their task cards (unmatched subs under the change
   header), each with the existing talk/terminal affordance pointed at the sub's
   session ID; dead subs render in the existing `Live: false` style.

## File-level impact

- `internal/server/mapping.go` — SessionEntry fields, accessors, reconciler, list
  handler changes.
- `internal/server/changesession.go` — bind endpoint handler, `changePrompt` text.
- `internal/server/server.go` — route registration for the new endpoint.
- `internal/server/mapping_test.go`, `changesession_test.go` — new/updated tests.
- `web/static/app.js`, `web/templates/*.html` — task-grouped sub list + talk button.
- `README.md` — document the endpoint and the delegation workflow.

## Data, API, and schema changes

- `.lessmess/sessions.json`: entries gain optional `task` and `parent` strings.
- New endpoint `POST /api/changes/{id}/task-sessions` (JSON; guards above).
- `GET /api/changes/{id}/sessions` response entries gain `task` and `parent`;
  reconciled children may appear for the first time.

## Safety, security, and rollback

- Trust model identical to scaffold: the caller names its own session; the mapping
  guard keeps a session from binding into a change it does not belong to.
- The reconciler only ever adds mapping entries derived from live service data; it
  cannot delete or rewrite existing ones, and a service outage degrades to today's
  behavior.
- Rollback is config-free: stop shipping the prompt section (new sessions stop
  binding), and the extra fields are ignored by old code paths.

## Testing and verification strategy

- `go vet ./... && go test ./...` per repo standard; new unit tests use the fake
  opencode client (guard against regressions in old-file mapping compatibility).
- PSB-00 is a live smoke test against the running service, recorded as evidence.
- Manual verification: a scratch change session delegates one task, the sub appears
  on the board under the task, and the talk button opens a live terminal on it.

## Observability

- One `slog.Info` per bind and per reconciler batch (`bound task session`,
  `reconciled N subagent sessions`), matching existing session log style.

## Rollout sequence

Land PSB-01 → PSB-02/PSB-03 → PSB-04 → PSB-05; rebuild the binary (embedded web
assets) and start a fresh change session to pick up the new prompt.

## Risks and mitigations

- **Task-tool result may hide the child session ID** — the reconciler is the primary
  mapper in that world; the prompt then teaches the title convention only.
- **Direct prompting of a finished child might be rejected** — PSB-00 is the gate;
  if it fails, PSB-05 degrades to view-only affordances and the change is rescored.
- **Sub sessions culled by the service** — dead IDs render `Live: false` like
  today's stale sessions; no crash path.

## Acceptance criteria

- A change session can bind a subagent session to a task via the API, and the board
  shows it under that task with a working talk button.
- Unbound children of a change session appear on the board after a session-list
  refresh even when the bind call was never made.
- Old `.lessmess/sessions.json` files load unchanged; `lessmess validate` stays
  clean throughout; existing prompt-pin tests keep passing.

## Tasks

1. [PSB-00](tasks/00-spike-subagent-session-basics.md) — Spike: subagent child promptability and ID visibility (go/no-go).
2. [PSB-01](tasks/01-mapping-task-fields.md) — Mapping schema: task and parent fields.
3. [PSB-02](tasks/02-bind-endpoint.md) — Bind endpoint for task sessions.
4. [PSB-03](tasks/03-parentid-reconciler.md) — parentID reconciler on session list.
5. [PSB-04](tasks/04-change-prompt-delegation.md) — changePrompt delegation and bind instructions.
6. [PSB-05](tasks/05-board-sub-list.md) — Board UI: per-task sub list with talk button.
