
# HOF-00: spawn-change endpoint, changePrompt session ID + handoff step, SpawnedFrom field

Status: see [../ledger.md](../ledger.md).

## Objective

Server-side core of the handoff: a bound change session (or the board) can create a new change with a fresh primed session bound to it, seeded from a handoff artifact written inside the source change.

## Dependencies

None — first task of the change.

## Scope

- `internal/server/changesession.go`: `spawnChange` handler, `spawnChangeRequest` type, artifact path validation, handoff addendum builder; `changePrompt` gains a session-ID parameter and a handoff step (with the user's explicit approval, write `changes/<id>/handoff-<topic>.md`, then POST spawn-change); caller-check error messages are agent-facing so a misled agent self-corrects.
- `internal/server/server.go`: route `POST /changes/{id}/spawn-change`.
- `internal/server/mapping.go`: `SpawnFrom`… `SpawnedFrom string \`json:"spawnedFrom,omitempty"\`` on `SessionEntry` (and the test used by `spawnChange` sets it).
- Update existing pinned tests that assert on `changePrompt(id)` (e.g. `settingswiring_test.go`).

Out of scope: any board UI (HOF-01), README (HOF-02).

## Implementation steps

1. Add `spawnChangeRequest{Title, Prefix, Artifact, Session}` with scaffold-identical validation for title/prefix (reuse `prefixRe`; no `|` in title).
2. Validate `Artifact`: non-empty, `.md` extension, `filepath.Base` equals the artifact (no separators/`..`), and `os.Stat` confirms it exists inside `changes/<id>/`. 422 otherwise.
3. Guard order mirroring `bindTaskSession`: 503 (no opencode / unreadable mapping) → 404 unknown change → 400 bad JSON → 422 shape validation → 409 supplied session bound elsewhere.
4. Create change B via `s.st.CreateChange(title, prefix, s.effectiveSettings().Git.DefaultBranch, today)`.
5. Spawn session titled `<B> — <title>` via `s.spawnSession`; on spawn/prime failure delete the session and return 502 with B's ID in the body (canonical data is never rolled back).
6. Prime = `s.promptWith(changePromptWithHandoff(B, A, artifact), "change")` — `changePrompt` text plus a short addendum naming the source change and artifact as the authoritative starting context, to be distilled into plan.md + tasks before implementation.
7. Bind: `s.sessions.add(B, SessionEntry{Session, Title, Created, SpawnedFrom: A})`; log `change handoff spawned`.
8. Update `changePrompt` call sites and pinned tests for the new signature.

## Verification

- `go vet ./... && go test ./...` from the repo root.
- Handler tests (fake client, fixture store, patterns from `changesession_test.go`): happy path 201 creates B + binds session with `SpawnedFrom` and the addendum in the prime; 409 cross-change caller; 422 missing artifact / traversal (`../x.md`, `a/b.md`) / bad prefix; 502 path deletes the session and names B; scaffold's bound-session 409 still pinned.

## Completion criteria

All verification passes; endpoint behaves exactly as the plan's API surface section specifies; no existing behavior regressed.

## Files affected

- `internal/server/changesession.go`, `internal/server/server.go`, `internal/server/mapping.go`
- `internal/server/changesession_test.go`, `internal/server/settingswiring_test.go` (pinned-prompt updates)

## Notes

- The agent-facing flow relies on `changePrompt` carrying the session's own ID — without it the session cannot fill `session` (same gap PSB-00 documented for task-sessions).
- `SpawnedFrom` stores the source change ID; the UI badge is HOF-01's job.
- Implementation tightened artifact validation beyond the original plan text: the endpoint requires the full `handoff*.md` bare-filename convention (not just `.md`), so fixed-name workflow files can never be mistaken for context. Plan synced.
- `createChangeSession` now builds the prime after the spawn (change primes carry the session's own ID); task primes unchanged. Pinned prompts updated in `mapping_test.go` and `settingswiring_test.go`.
- Amendment 2026-09-18 (second user-reported finding, from the first real handoff in another repo): the spawned session wrote six task files without governing-ledger rows (validation rule-3 violations). The addendum now requires task files and ledger rows in the same pass plus a validator run before implementation; `changePrompt` rule 2 states the same pairing invariant. Pinned substrings in `TestSpawnChangeHandoff` unaffected; targeted server tests pass.
- Validation evidence 2026-09-17: `go vet ./...` clean; `go test ./...` all 6 packages ok; new handler tests `TestSpawnChangeHandoff` (201 + provenance + prompt contents + clean validate), `TestSpawnChangeValidation` (400/404/409/422 matrix incl. traversal and non-handoff names), `TestSpawnChangeNoService` (503), `TestSpawnChangePrimeFailure` (502 names change, change survives, session deleted); scaffold 409 still pinned by `TestScaffoldBoundSessionRefused`.
- Amendment 2026-09-17 (user-reported from another instance): an old-primed bound session that tries scaffold now gets a 409 that teaches the sanctioned path — write `changes/<bound>/handoff-<topic>.md`, then POST `/changes/<bound>/spawn-change` with `{title,prefix,artifact,session}` and explicit user approval. `TestScaffoldBoundSessionRefused` asserts the pointer. Note: a repo-wide `go test ./...` currently fails only in `internal/docs` (`TestWorkflowAssetDrift`) due to a concurrent change's in-flight AGENTS.md/asset sync — unrelated to this change; all other packages pass.
