# 2026-09-12-7: Repo docs management

- Change ID: 2026-09-12-7
- Created: 2026-09-12
- Branch: —
- Status: see [ledger.md](ledger.md)

## Objective and context

Extend tasktracker from a `changes/` kanban into a **repo docs management** layer that
bootstraps and maintains agent-facing documentation across a repository:

- `tasktracker init` turns an uninitialized directory into a workflow-ready repo:
  root `AGENTS.md` carrying the canonical change-management instructions, `changes/`
  skeleton, `.gitignore` / `.tasktracker/` handling, starter `opencode.json`, and the
  committed docs config.
- Every configured ("significant") folder gets a doc pair:
  - **`STRUCTURE.md`** — a machine-owned navigation map of the files/folders below it,
    their purposes, and rollups of child folders. Regenerated wholesale; cheap and
    deterministic.
  - **`AGENTS.md`** — curated learnings, gotchas, and instructions for that area.
    Refined/append-only; never regenerated, so accumulated knowledge is safe.
- When a change is closed, a serialized **doc gardener** job refreshes the doc pairs
  for the folders that change touched, via an unattended opencode session. If the
  opencode service is unavailable, folders are flagged stale and reconciled later.
- `tasktracker docs seed` performs the one-time initial run-through for an existing
  codebase: bottom-up, incremental/resumable, budget-capped.

This systematizes the obligation already stated in the root `AGENTS.md` ("If you
modified any files/styles/structures/configurations/workflows mentioned in `AGENTS.md`
files, you MUST update the corresponding `AGENTS.md` files") instead of relying on
agent discipline alone.

## Current behavior

- tasktracker serves the kanban over `changes/` only; all file writes are constrained
  to the `changes/` tree. There is no repo-wide documentation tooling.
- `POST /changes/{id}/close` (`internal/server/lifecycle.go`) only flips overall
  status to `Done`; nothing downstream reacts to close-out.
- The opencode integration (`internal/opencode`, `internal/server/changesession.go`,
  commit flow in `lifecycle.go`) already creates, primes, renames, and awaits
  unattended sessions server-side — the exact machinery a doc gardener needs.
- `tasktracker validate` enforces the `changes/` contract only.
- Tooling state lives in gitignored `.tasktracker/` (sessions mapping today).

## Target behavior

1. **Init**: `tasktracker init [--dir .]` in any folder (git not assumed) writes the
   root `AGENTS.md` with the canonical workflow text embedded from the binary,
   `changes/ledger.md` skeleton, `.gitignore` containing `.tasktracker/`, starter
   `opencode.json`, and a default `agentsdocs.json`. Existing files are merged, never
   clobbered; a second run is a no-op.
2. **Per-folder doc pair**: covered folders hold `STRUCTURE.md` (fully inside
   `<!-- tasktracker:begin -->` / `<!-- tasktracker:end -->` markers) and `AGENTS.md`
   (curated, with a marker-guarded auto section; content outside markers is treated
   as human-authored and preserved byte-for-byte). Each auto section carries freshness
   metadata: last-refresh date, source change ID, and a tree hash.
3. **Close-out refresh**: closing a change computes the touched-folder set from its
   tasks' "Files affected" sections, maps it onto covered dirs, and enqueues one
   serialized gardener job. The job spawns an unattended opencode session primed with
   the change context and the current docs; the agent updates `STRUCTURE.md` blurbs
   and appends/refines `AGENTS.md` learnings, each learning citing the change ID.
   After the session finishes, the server verifies that only marker sections changed
   (reverting otherwise) and stamps freshness metadata.
4. **Stale fallback**: if the service is unavailable or a job fails, affected folders
   are flagged stale in `.tasktracker/`; the next successful run (or a manual
   `tasktracker docs refresh`) reconciles them. `validate` and the board surface
   staleness.
5. **Seed**: `tasktracker docs seed [--dry-run] [--budget N]` walks bottom-up (leaves
   first, so parents summarize children), generates `STRUCTURE.md` deterministically,
   and uses LLM sessions for purpose blurbs and first-pass `AGENTS.md` learnings.
   Progress is resumable from `.tasktracker/docs-seed.json`; runs are idempotent.

## Scope

- New `internal/docs` package: coverage config, tree walk, deterministic
  `STRUCTURE.md` generation, seed engine (LLM access via the existing
  `internal/opencode` client, behind a summarizer interface for tests).
- `internal/model`: doc-file format (markers, freshness metadata, merge semantics).
- `internal/server`: close-out hook, serialized refresh queue, gardener session
  management, marker-violation guard.
- `cmd/tasktracker`: `init` and `docs seed` / `docs refresh` subcommands.
- `internal/store`: staleness/queue state in `.tasktracker/`, extended validation.
- Committed `agentsdocs.json` for this repository.
- Root `AGENTS.md` and `README.md` updates documenting the new subsystem.

## Non-goals

- No human-approval step for gardener amendments (decided: fully automatic, with
  marker-guard and change-ID provenance as the safety net).
- No continuous whole-repo fsnotify triggering content updates (stale fallback
  covers out-of-band edits; existing `changes/` watch is untouched).
- No git operations (the commit flow stays exactly as-is; git is not assumed in
  target repos).
- No multi-repo / remote operation; one process still serves one repository.
- No deletion of doc files; when a folder disappears its docs disappear with it, and
  the parent's `STRUCTURE.md` simply drops the entry on next refresh.
- No changes to the `changes/` workflow contract itself (schemas stay pinned).

## Design decisions

1. **Two files, two update semantics** — structure is regenerated wholesale (safe:
   machine-owned), learnings are append/refine-only (safe: never clobbered). A single
   combined file would force one semantics onto both content types.
2. **Marker-guarded auto sections** — `<!-- tasktracker:begin/end -->` delimit
   machine-maintained regions. `STRUCTURE.md` is ~fully machine-owned; `AGENTS.md` is
   mostly curated with a small auto section. Everything outside markers is preserved
   byte-for-byte, matching the server's existing "external edits are never clobbered"
   posture.
3. **Close-out hook + stale fallback** — close-out is the authoritative, correctly
   batched trigger (the change record already names affected files). The stale queue
   is the safety net for service downtime; no whole-repo watcher.
4. **Server-spawned gardener, unattended** — reuses the proven
   `CreateSession`/`Prompt`/`WaitDone` pattern from the commit flow. One serialized
   queue = single writer per file, mirroring the ledger single-writer rule.
5. **Deterministic first, LLM second** — the `STRUCTURE.md` skeleton (tree, rollups,
   hashes) is pure Go; the LLM only writes purpose blurbs for new/changed entries and
   `AGENTS.md` learnings. Existing blurbs are merged forward, never regenerated —
   this keeps refresh cheap and stable, and makes most behavior unit-testable.
6. **Committed config, gitignored state** — `agentsdocs.json` (coverage policy) is
   committed because it governs canonical files; queue/stale/seed-cursor live in
   `.tasktracker/` per the `AGENTS.md` tooling-state rule.
7. **Opt-in by config presence** — without `agentsdocs.json` the close-out hook is a
   no-op, so existing repositories (including this one until configured) see zero
   behavior change.
8. **Recursion exemption** — gardener writes are tagged and never re-enqueue; doc
   updates do not create changes.
9. **Provenance** — every auto-added learning cites its source change ID; freshness
   metadata records the last change and tree hash, enabling mechanical staleness
   validation.
10. **Canonical workflow text embedded** — `init` writes the change-management
    instructions from a `go:embed`ded asset; a test asserts the repo root `AGENTS.md`
    contains it verbatim so the two never drift.

## Compatibility

- Default-off: no `agentsdocs.json` → no hook, no new writes anywhere. Existing
  `changes/` behavior, validation, and tests are untouched.
- Write scope expands beyond `changes/` for the first time — limited strictly to
  marker sections of `STRUCTURE.md`/`AGENTS.md` inside covered dirs, using the same
  atomic temp+rename writes (`internal/model/atomic.go`) and validation-refusal
  discipline.
- The root `AGENTS.md` of an existing repo is never overwritten by `init` or `seed`;
  marker sections are merged in only.

## Implementation approach

Ordered bottom-up so each layer is testable before the next builds on it:

1. Doc-file format model (markers, metadata, byte-preserving merge) in
   `internal/model`.
2. Coverage config (`agentsdocs.json`) loader + defaults in `internal/docs`.
3. Deterministic bottom-up `STRUCTURE.md` generator with blurb merge + tree hashing.
4. `tasktracker init` (embedded workflow asset + skeleton files, merge-safe).
5. `tasktracker docs seed` (summarizer interface over `internal/opencode`;
   resumable cursor; `--dry-run`, `--budget`).
6. Close-out hook + serialized persisted queue + stale flags in `internal/server`.
7. Gardener session: prompt builder, spawn/await, marker-violation verify-and-revert.
8. `validate` extension (coverage, marker integrity, freshness) + board staleness
   indicator.
9. Dogfood on this repository; tune prompts; update `README.md` and root `AGENTS.md`.

## File-level impact

- `internal/model/docfile.go`, `docfile_test.go`, `testdata/` — new.
- `internal/model/templates.go` — doc templates; possibly the embedded workflow asset.
- `internal/docs/` — new package: `config.go`, `walk.go`, `structure.go`, `seed.go`,
  `summarize.go`, tests + fixtures.
- `internal/server/docsqueue.go`, `docssession.go` — new; `lifecycle.go` — close hook.
- `internal/store/validate.go`, `store.go` — docs state + validation extension.
- `cmd/tasktracker/main.go` — `init`, `docs seed`, `docs refresh` subcommands.
- `web/` — staleness indicator (small template/static change).
- `agentsdocs.json` — new committed config.
- `README.md`, `AGENTS.md` — documentation updates.
- `.tasktracker/docs-*.json` — new gitignored runtime state (queue, stale, seed cursor).

## Data, API, message, configuration, and schema changes

- **Committed config** `agentsdocs.json`: include/exclude globs, per-dir overrides,
  default exclusions (`node_modules`, `.git`, `dist`, build outputs, hidden dirs).
- **Doc files**: marker format `<!-- tasktracker:begin -->` / `<!-- tasktracker:end -->`;
  freshness metadata line inside the auto section (date, change ID, tree hash).
- **Tooling state** (gitignored, `.tasktracker/`): `docs-queue.json` (pending jobs),
  `docs-stale.json` (stale folder set), `docs-seed.json` (seed cursor + budget).
- **HTTP**: no breaking changes. New endpoints only as needed (e.g.
  `POST /docs/refresh` for manual reconciliation). The close endpoint gains a side
  effect, gated by config presence.
- **Agent prompts**: new gardener prompt (change context + target folders + current
  docs + write constraints).

## Safety, security, rate-limit, migration, and rollback

- **Write confinement**: gardener output is accepted only if a server-side diff shows
  changes confined to marker sections of the intended files; violations are reverted
  and logged. The repo-root `AGENTS.md` gets extra protection (workflow text must
  remain intact).
- **Permissions**: gardener sessions run under the same `opencode.json` permission
  envelope as existing change sessions (project-scoped, `.env` denied, no push).
- **Rate limits / cost**: serialized queue (one gardener at a time); deterministic
  generation needs zero tokens; seed has an explicit budget cap and resume cursor.
- **Migration**: none — feature is additive and opt-in.
- **Rollback**: delete generated doc files / `agentsdocs.json`, or revert the change;
  `.tasktracker/` state is disposable. Since docs are plain markdown, git history
  (where present) also serves as undo.

## Testing and verification strategy

- `internal/model`: round-trip, human-content preservation, missing-file creation,
  metadata parse — table-driven tests with `testdata/`.
- `internal/docs`: fixture trees; generator idempotence (second run byte-identical);
  config glob edge cases; seed with a stub summarizer; resume-after-interruption.
- `internal/server`: httptest close → enqueue; service-down → stale; queue ordering;
  marker-violation revert (fake opencode client, matching existing test patterns).
- `cmd`: `init` in a temp dir passes `validate` and is idempotent; `seed --dry-run`
  output stable.
- Existing suites (`go vet ./...`, `go test ./...`) must stay green — proves the
  `changes/` contract is untouched.
- Dogfood: seed this repo, close a real change, record observed gardener behavior.

## Observability

- `slog` events: enqueue, job start/finish, session IDs, stale-flag set/cleared,
  marker-violation reverts, seed progress.
- Board: staleness indicator; validation banner covers new rules.
- Freshness metadata inside each doc gives humans/auditors local provenance.

## Rollout sequence

1. Land model + config + generator (inert without config).
2. Land `init` + `seed` CLI (manual invocation only).
3. Land close-out hook + queue + gardener (active only with `agentsdocs.json`).
4. Extend validation + UI.
5. Dogfood here; only then recommend for other repos.

## Risks and mitigations

- **Hallucinated or low-value learnings** — mitigated by mandatory change-ID
  citations, scoped prompts (one folder set, current docs provided), and easy human
  editing outside markers.
- **Doc churn** — batched per change close, not per task; blurbs merge forward so
  stable trees produce byte-identical docs.
- **Gardener overreach** — server-side marker-confinement check with revert; root
  `AGENTS.md` integrity assertion.
- **Token cost on large repos** — seed budget cap, resumable cursor, deterministic
  skeleton; per-folder sessions keep contexts small.
- **Queue/state corruption** — same atomic-write + validation discipline as today;
  worst case is deleting disposable `.tasktracker/docs-*.json` and re-flagging stale.
- **Drift between embedded workflow asset and root `AGENTS.md`** — verbatim
  containment test.

## Acceptance criteria

1. `tasktracker init` in an empty directory produces a repo that passes
   `tasktracker validate`; second run is a no-op; existing files never clobbered.
2. `tasktracker docs seed --dry-run` on this repo prints a stable plan; a real seed
   creates marker-delimited doc pairs in covered dirs, is resumable after
   interruption, and a second seed is byte-identical given an unchanged tree.
3. Closing a change enqueues a gardener job; the gardener updates only marker
   sections of affected docs; added learnings cite the change ID; content outside
   markers is byte-identical afterward.
4. With the opencode service down at close time, folders become stale and reconcile
   on the next available run; `validate` and the board show staleness meanwhile.
5. Gardener writes never retrigger the hook; `validate` passes after a gardener run.
6. A gardener session that edits outside markers is reverted and logged.
7. Without `agentsdocs.json`, closing a change has no docs side effects.
8. `go vet ./...` and `go test ./...` pass; all pre-existing tests unmodified and green.
9. `README.md` and root `AGENTS.md` document the subsystem accurately.

## Tasks

1. [DOC-00](tasks/00-doc-file-format-model.md) — Doc file format model (markers, metadata, merge)
2. [DOC-01](tasks/01-coverage-config.md) — Coverage configuration (`agentsdocs.json`)
3. [DOC-02](tasks/02-structure-generator.md) — Deterministic STRUCTURE.md generator
4. [DOC-03](tasks/03-init-command.md) — `tasktracker init` command
5. [DOC-04](tasks/04-seed-command.md) — `tasktracker docs seed` command
6. [DOC-05](tasks/05-closeout-hook-queue.md) — Close-out hook + serialized refresh queue
7. [DOC-06](tasks/06-gardener-session.md) — Doc gardener session + confinement guard
8. [DOC-07](tasks/07-validation-board-surfacing.md) — Validation + board staleness surfacing
9. [DOC-08](tasks/08-dogfood-documentation.md) — Dogfood on this repo + documentation
