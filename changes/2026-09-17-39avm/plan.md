# 2026-09-17-39avm: Change handoff

- Change ID: 2026-09-17-39avm
- Created: 2026-09-17
- Branch: —
- Status: see [ledger.md](ledger.md)

## Objective and context

Let a session bound to an existing change A hand genuinely out-of-scope work to a brand-new change B with full context, in one flow. Today this is impossible by design: `POST /changes/scaffold` refuses any session already bound to a change (409), and `changePrompt` forbids bound sessions from creating changes — so the user's only path is to open a fresh discussion and re-explain the context by hand. The workflow's own rules point at the fix: supporting artifacts are legal inside a change directory, and the board can already spawn fresh primed sessions for an existing change. This change wires those pieces into a first-class handoff.

## Current behavior

- `POST /changes/scaffold` (`scaffoldChange`, `internal/server/changesession.go`) creates a change, renames and binds the caller session; a bound caller gets 409.
- `POST /changes/{id}/sessions` (`createChangeSession`, `internal/server/mapping.go`) spawns a fresh session primed with `changePrompt(id)` (or `taskPrompt` for a task) and binds it.
- `changePrompt` injects only the change ID — a bound session does not know its own session ID, so it cannot identify itself to session-taking endpoints.
- Sessions map one-to-one to a change (`s.sessions.changeOf`); `SessionEntry` carries optional `task`/`parent` only.
- The board's sessions panel (`web/templates/board.html`, `web/static/app.js`) lists sessions and can create one for the change; there is no way to create a change from within another change.

## Target behavior

- New endpoint `POST /changes/{id}/spawn-change`: creates a new change B from within change A, spawns a fresh session bound to B, and primes it with `changePrompt(B)` plus a handoff addendum pointing at a context artifact authored inside A.
- The handoff context lives in `changes/<A>/handoff-<topic>.md` — written by A's session (agent-driven flow) or by the user (manual flow). Supporting artifacts inside a change directory are workflow-legal per AGENTS.md.
- `changePrompt` gains the session's own ID (as `discussionPrompt` already does) and a handoff step: with the user's explicit approval, write the artifact and call the endpoint. Scaffold's 409 guard and prompt prohibition remain untouched.
- The board's sessions panel gets a "Spawn new change" action (title, prefix, artifact picker) for human-driven use, listing `handoff*.md` files via a small GET endpoint.
- The new session's mapping entry records `spawnedFrom: <A>` (omitempty) so the board can badge provenance.

## Scope

- `internal/server/changesession.go`: new `spawnChange` handler + request type + validation; `changePrompt` signature change (inject session ID) and new handoff step; handoff addendum builder.
- `internal/server/server.go`: routes `POST /changes/{id}/spawn-change` and `GET /changes/{id}/handoffs`.
- `internal/server/mapping.go`: `SpawnedFrom` field (omitempty) on `SessionEntry`.
- `web/templates/board.html` + `web/static/app.js`: "Spawn new change" form in the sessions panel; `spawnedFrom` badge on session rows.
- `web/static/app.js` (or server render): artifact picker populated from the handoffs endpoint.
- `README.md`: user-visible endpoint and workflow.
- Tests: handler tests in `changesession_test.go` / `mapping` tests following the existing fake-client patterns.

## Non-goals

- No change to `POST /changes/scaffold` behavior or its guard.
- No changes to discussion sessions or their prompt.
- No auto-detection of "out of scope" work — the handoff is always user-approved.
- No cross-repository or multi-hop orchestration (chained handoffs B→C are allowed but unmanaged).
- No board UI beyond the sessions-panel action (no index-page entry point).

## Design decisions

1. **Artifact file, not inline text.** The request references an existing `changes/<A>/handoff-*.md` file rather than carrying the context in the POST body. Context authored by the source session stays in the source change's directory (workflow-legal supporting artifact), survives retries, and is readable before the request fires.
2. **Bind immediately, plan first.** B exists and the fresh session is bound at spawn; its prime says to distill the handoff artifact into `plan.md` + tasks before implementing. The user's approval gate is satisfied by the user driving/approving the handoff itself, so no separate discussion phase is needed.
3. **CreateChange before spawn.** Validation → `CreateChange` → spawn → prime → bind. The change directory is canonical data and is never rolled back (mirrors `scaffoldChange`); if spawn or prime fails, the session is deleted (no unbound leaks, mirrors `createChangeSession`) and the 502 body names B so the user can continue it from the board.
4. **Session ID in `changePrompt`.** Enables the agent-driven flow (the session can call the endpoint with its own ID, like discussions do with scaffold). The endpoint's caller check — a supplied `session` must be bound to the path change — prevents cross-change abuse; an unsupplied session is allowed for UI-driven calls (same rationale as `task-sessions`, PSB-00).
5. **Artifact convention `handoff*.md` at the change-directory root.** Avoids collision with `plan.md`, `ledger.md`, and `tasks/`; the handoffs endpoint lists only files matching the convention.
6. **Provenance is additive.** `SpawnedFrom` is omitempty on `SessionEntry`; existing mappings and tests are unaffected. The addendum text also names the source change, so B's plan carries provenance in canonical data too.

## API surface

- `POST /changes/{id}/spawn-change` — body `{"title":"...", "prefix":"HOF"?, "artifact":"handoff-parser.md", "session":"ses_..."?}`. 201 `{change, session, title}`; 400 bad JSON; 404 unknown change; 422 invalid title/prefix, artifact missing/outside the change dir/not a `handoff*.md` bare filename; 409 supplied session bound elsewhere; 503 without opencode or with unreadable mapping; 502 spawn/prime failure (body names the created change).
- `GET /changes/{id}/handoffs` — 200 `{"handoffs":["handoff-a.md", ...]}` (sorted), 404 unknown change.

## Acceptance criteria

1. From a session bound to change A (or the board), a user-approved handoff produces change B, a session bound to B primed with `changePrompt(B)` plus the artifact addendum, and A's session untouched on A.
2. `spawn-change` with a bound caller from a different change returns 409; with a missing/outside artifact returns 422; scaffold's existing 409 behavior is unchanged (tests pin both).
3. The board's sessions panel offers "Spawn new change", lists `handoff*.md` artifacts, and badges sessions with `spawnedFrom`.
4. `go vet ./...` and `go test ./...` pass; `lessmess validate` clean on a repo after a handoff.
5. README documents the endpoint and the handoff workflow.

## Tasks

1. [HOF-00](tasks/00-spawn-endpoint.md) — spawn-change endpoint, changePrompt session ID + handoff step, SpawnedFrom field, handler tests.
2. [HOF-01](tasks/01-board-ui.md) — handoffs listing endpoint + "Spawn new change" board action + spawnedFrom badge.
3. [HOF-02](tasks/02-docs-validation.md) — README, workflow doc touch-ups, full validation pass.
