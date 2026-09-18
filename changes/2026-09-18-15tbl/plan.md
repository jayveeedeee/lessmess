# 2026-09-18-15tbl: JSON workflow state and instruction injection

- Change ID: 2026-09-18-15tbl
- Created: 2026-09-18
- Branch: —
- Status: see [ledger.md](ledger.md)

## Objective and context

Make the tool the deterministic owner of workflow state and agent instructions:

1. All change **tracking data** (change index, task tree, statuses, dependencies, dates, notes, priority order, decision logs) moves from markdown tables in `changes/` into JSON files under a committed `.lessmess/workflow/` subtree. Markdown stops being the state machine; the store/API becomes the only sanctioned mutation path.
2. The ~18 KB of workflow **instructions** currently baked into root `AGENTS.md` (and drift-pinned into `internal/docs/assets/workflow_agents.md`, and stale-copied into every `init`-bootstrapped repo) becomes versioned **instruction modules embedded in the binary**, injected deterministically into session primes at spawn — selected by session type and change/task state so irrelevant rules never pollute context.

Motivation: strict markdown parsing is the dominant fragility in `internal/model`; agents alter state by hand-editing prose tables with only advisory rules; instruction refinements require rewriting markdown in three decoupled places; and every opencode session in the repo pays for all 18 KB of workflow text regardless of relevance.

Baseline confirmed with the user: **committed** `.lessmess/workflow/` subtree (rest of `.lessmess/` stays gitignored and personal); **prose stays markdown** under `changes/<id>/` referenced from JSON; **docs system (STRUCTURE.md/AGENTS.md learnings pairs) untouched**.

## Current behavior

- `changes/ledger.md` (root), `changes/<id>/ledger.md`, and container `tasks/<n>-x/ledger.md` hold all tracking state as hand-editable markdown tables with pinned schemas.
- `internal/model` (~1.6 kLOC with tests) parses and byte-preservingly re-splices those tables; strict parsing turns stray characters into `ErrInvalid`; most `lessmess validate` rules exist to police hand-edited markdown.
- Agents mutate state by editing prose per the AGENTS.md contract; workflow rules (user-gated `Done`, dependency order, sequence allocation, decomposition gating) are advisory only.
- Root `AGENTS.md` carries the full workflow contract; it is byte-pinned to the embedded `internal/docs/assets/workflow_agents.md` (refreshed by hand with an `awk` one-liner, enforced by a test) and copied verbatim into every repo `lessmess init` touches — nothing ever updates downstream copies.
- Session primes (`discussionPrompt`, `changePrompt`, `taskPrompt`, `gardenerPrompt`, `explorerPrompt`, `commitPrompt`, `repoCommitPrompt`) are hardcoded Go strings in `internal/server`; per-repo addenda already come from layered JSON settings.
- `.lessmess/` is entirely gitignored tooling state; worktree-backed changes resolve their `changes/<id>/` directory inside the registered worktree.

## Target behavior

### Storage

```
.lessmess/workflow/                 # committed (gitignore negation)
├── index.json                      # today's root ledger: one entry per change
│                                   #   (id, title, prefix, branch, status, dates, archived)
└── changes/<id>.json               # one change: overall status (+derived flag),
                                    #   decision log, task tree:
                                    #   id, seq, parent, title, file, status,
                                    #   dependsOn[], updated, notes

changes/<id>/                       # prose only, still git-committed
├── plan.md                         # unchanged role
└── tasks/<nn>-name.md              # prose bodies; no frontmatter, no tables;
    └── <nn>-name>/                 #   containers keep prose dirs for subtasks,
        └── tasks/…                 #   no container ledger.md
```

- All markdown ledger files (root, change, container) disappear; humans get generated views instead.
- Statuses store the exact current display strings (`Not started`, … `Done`) to minimize mapping.
- Task identity, order (array order = priority), nesting (`parent`), and per-task metadata live only in JSON; prose files are referenced by path and may keep the heading skeleton by convention.

### Mutations

- Every state change goes through `store` methods / HTTP endpoints writing JSON atomically (re-read → apply → `WriteFileAtomic`, same conflict-safety pattern as today).
- Workflow rules become code-enforced invariants: `Done` transitions are user-gated (agent-side attempts rejected), task sequence allocation, dependency presence, decomposition preconditions, derived overall status (`DeriveOverall` logic kept, written by the tool only).
- Agent instruction never includes table schemas — the contract becomes "call the API".

### Instruction injection

- Instruction modules ship as embedded JSON in the binary (`id`, `version`, `audience`, optional `conditions`, `text` with placeholders such as `{{changeId}}`, `{{apiBase}}`).
- A deterministic selection function `f(session kind, change state, task state)` picks the module set at spawn; the prime is rendered as binding header + state snapshot from JSON + selected modules + settings addendum (addenda mechanism unchanged).
- Module map (v1): `discussion`, `change.session`, `task.session`, `task.planning`, `decomposition` (task sessions), `worktree` (worktree-backed changes only), `handoff` (change sessions), `closeout` (task sessions), plus the existing `gardener`, `explorer`, `commit`, `repoCommit` bodies migrated to modules.
- Static selection at spawn in v1 (modules chosen slightly wider where cheap); no event-driven re-injection yet.
- The injected module set is logged and recorded with the session mapping for auditability; `GET /workflow/instructions` serves the current module manifest for debugging/escape-hatch fetching.

### Contract surface

- Root `AGENTS.md` shrinks to a short pointer (≤ ~30 lines: "this repo uses lessmess; state is tool-owned JSON — never edit `.lessmess/` by hand; change work happens in board-spawned sessions") while keeping the marker-guarded learnings sections untouched.
- The drift-pin between root `AGENTS.md` and `internal/docs/assets/workflow_agents.md` is deleted; the embedded asset becomes the pointer text that `init` writes.

## Scope

- New JSON state model, store cutover, md→JSON migration, task-state API, instruction module subsystem with injection, board/detail view rework, AGENTS.md/init/README updates, `.gitignore` negation.
- Migration of this repository's 40 existing change directories (including archived and legacy numeric IDs).
- This repo is the dogfood target and the reference consumer.

## Non-goals

- Moving plan/task **prose** into JSON (stays markdown).
- Touching the docs system's STRUCTURE.md/AGENTS.md learnings machinery beyond the workflow-text asset swap and natural learning refreshes at close.
- Event-driven mid-session re-injection of instructions.
- Per-repo instruction overrides (built-in only in v1 — overrides would reintroduce the drift this change removes).
- Multi-user concurrency beyond the existing single-server-single-writer model.
- Changes to the settings layering, docs queue, sessions mapping, or accent/UI chrome beyond what the store cutover forces.

## Design decisions

1. **Committed `.lessmess/workflow/`** via gitignore negation (`!.lessmess/` / `.lessmess/*` / `!.lessmess/workflow/`): state keeps git history, review, and multi-machine sync; personal state remains ignored. User-confirmed.
2. **Prose/reference split**: JSON owns identity and state; md owns narrative. Avoids escaped-multiline-string ugliness and keeps agent file-editing ergonomics for prose.
3. **No frontmatter in task prose files**: identity is JSON-only; the old `id`+`title` frontmatter would be a drift source. Migration strips it.
4. **Instruction modules embedded in the binary, JSON-formatted**: instructions version with the tool; a session gets the rules of the binary that spawned it; refinement needs no repo-file updates.
5. **Modules must be self-contained** (no cross-module "see rule N" references); the selection table is exhaustive and test-pinned so no session type hits a missing-rule situation.
6. **Container directories keep prose subdirs but lose `ledger.md`**: nesting is fully expressed by `parent` + dotted IDs in JSON; `DecomposeTask`/`CreateTask` numbering rules are preserved in the JSON model.
7. **Worktree interplay simplifies**: state is always main-tree; the worktree resolver keeps applying to prose directories only.
8. **Derived overall status stays derived** and is written by the tool on every task change (replacing the "both ledgers" dual-write).
9. **Migration is one-way and verified**: md tables are round-trip-checked (render md from JSON, compare with source tables) before the md ledgers are deleted; git history is the rollback for deleted files.

## Detailed implementation approach

Ordered task breakdown; each task lands with tests green:

1. **JSI-00 — JSON state schema and model types**: Go structs + load/save for `index.json` and `changes/<id>.json`, status vocabularies reused, stable field order, atomic writes, round-trip tests. No wiring.
2. **JSI-01 — md→JSON migration**: read the existing tree via current parsers (root/change/container ledgers, task frontmatter), emit `.lessmess/workflow/` JSON, strip task frontmatter, verify round-trip, then delete md ledgers; exposed as `lessmess migrate` and auto-run by `serve`/`validate` when md state is detected and JSON is absent.
3. **JSI-02 — store cutover**: `Open`/`Reload`/`Watch`/`Validate`/tree building over JSON + prose files; mutations rewrite JSON; `ErrNoChanges` semantics move to `.lessmess/workflow/` absence; `.gitignore` negation applied; worktree resolver scoped to prose.
4. **JSI-03 — task-state API**: `POST` endpoints for task status/update (title/notes/deps)/reorder and decision-log append, with server-side enforcement of user-gated `Done`, dependency, and sequence rules; change-level status endpoint adapted.
5. **JSI-04 — instruction modules and injection**: embedded module JSON, selection engine, prime renderer replacing hardcoded prompt bodies, audit recording (log + session mapping), `GET /workflow/instructions`.
6. **JSI-05 — board and detail views from JSON**: ledger detail pages become generated views; task detail composes JSON identity with prose body; index/board keep their shape.
7. **JSI-06 — AGENTS.md shrink, init rework, docs**: pointer `AGENTS.md`, drift-pin deletion, `init` bootstrap of the new skeleton, README, and stale root learnings left to the normal close-time gardening.

## File-level impact

- `internal/model`: new JSON state types and view renderer; ledger/task-file parsers demoted to migration-only; `docfile.go` untouched (docs system); `templates.go` rewritten for generated views.
- `internal/store`: `store.go`/`tree.go`/`validate.go`/`overall.go` rewritten over JSON; `watch.go` gains `.lessmess/workflow/`; `statedir.go` unchanged.
- `internal/server`: new task-state + instructions endpoints; `changesession.go` primes become renders; `mapping.go` records injected modules; templates and `render.go` adjustments; worktree resolver scoped down.
- `internal/docs`: `init.go` embeds pointer text; `assets/workflow_agents.md` replaced; drift-pin test updated.
- `cmd/lessmess`: `migrate` subcommand; `init` rework.
- `web/templates`: ledger detail → generated view; task detail composition.
- Repo root: `AGENTS.md` (pointer + learnings), `.gitignore` (negation), `README.md`.
- All 40 `changes/<id>/ledger.md` files + container ledgers + root `changes/ledger.md` deleted; task files lose frontmatter.

## Data, API, configuration, and schema changes

- New: `.lessmess/workflow/index.json`, `.lessmess/workflow/changes/<id>.json` (schema above).
- New endpoints: task status/update/reorder, decision-log append, `GET /workflow/instructions`.
- Changed: prime composition (state snapshot + modules); `ErrNoChanges` trigger; `lessmess init` output; `.gitignore` content.
- Removed: markdown ledger read/write paths after migration.

## Safety, security, migration, and rollback considerations

- Migration runs before the store opens; it is idempotent, refuses to run when JSON already exists, and round-trip-verifies before deleting any md ledger. Failure leaves the md tree untouched.
- Deleted md files remain in git history — rollback is `git revert` of the migration commit plus removing `.lessmess/workflow/` and pinning the previous binary.
- `Done` gating and sequence/dependency rules become server-enforced; agents lose the ability to bypass them by editing files (state file edits are out-of-contract and detectable via `git diff` on the committed JSON).
- Instruction JSON is embedded (read-only) — no injection of untrusted content; placeholders are server-controlled values.

## Testing and verification strategy

- Unit: JSON round-trip (model), migration fixtures including this repo's tree snapshot, legacy numeric IDs, archived changes, containers; store open/mutate/watch over JSON; endpoint enforcement tests (agent `Done` attempt → 4xx; sequence allocation; dependency checks); injection selection determinism pinned by table tests; view rendering snapshots.
- `go vet ./... && go test ./...` per task.
- `lessmess validate` clean on the migrated repo; manual smoke: spawn discussion/change/task sessions and inspect primes for module selection and state snapshot; board, drill-down, worktree change, docs refresh all exercised.

## Observability requirements

- Log line per state write (existing pattern) naming change/task/transition.
- Log line + mapping record of injected module IDs at session spawn.
- Migration summary (changes, tasks, deletions, verification result) logged and printed.

## Rollout sequence

1. Land JSI-00…JSI-06 sequentially in this repo (dogfood).
2. First `serve` after JSI-02 migrates this repo's tree automatically.
3. External bootstrapped repos auto-migrate the same way on their next `lessmess` upgrade; breaking change documented in README.

## Risks and mitigations

- **Transition window (JSI-02→04)** where old primes reference md ledgers that no longer exist — mitigated by tight sequencing inside one landing and updating primes in JSI-04 immediately after the cutover.
- **Rogue hand-edits to `.lessmess/workflow/*.json`** — pointer forbids it; JSON is committed so diffs are reviewable; endpoints validate invariants on load (`validate` reports drift).
- **Module authoring drift (cross-references, gaps)** — self-containment rule plus exhaustive test-pinned selection table.
- **Migration edge cases** (legacy IDs, empty containers, archived trees) — fixtures from the real repo tree; round-trip verification gate.
- **Worktree prose resolver regressions** — existing worktree tests ported to JSON store.

## Acceptance criteria

1. No markdown ledger tables exist in workflow state; `.lessmess/workflow/` JSON is the sole state source and is committed.
2. All state mutations flow through store/API; user-gated rules (notably `Done`) are enforced server-side and covered by tests.
3. Primes contain only the selected instruction modules; selection is deterministic and test-pinned; injected sets are auditable.
4. Root `AGENTS.md` workflow text is a ≤ ~30-line pointer; the drift-pin asset and test are gone; `init` produces the new skeleton.
5. This repo's 40 changes migrate with verified round-trip; board, nested drill-down, sessions, worktrees, docs refresh, and settings all still work; `go vet ./...` and `go test ./...` green; `lessmess validate` clean.

## Tasks

1. [JSI-00](tasks/00-json-state-schema.md) — JSON state schema and model types
2. [JSI-01](tasks/01-md-to-json-migration.md) — md→JSON migration
3. [JSI-02](tasks/02-store-cutover.md) — store cutover to JSON state
4. [JSI-03](tasks/03-task-state-api.md) — deterministic task-state API
5. [JSI-04](tasks/04-instruction-injection.md) — instruction modules and injection engine
6. [JSI-05](tasks/05-board-views-from-json.md) — board and detail views from JSON
7. [JSI-06](tasks/06-agents-shrink-init-docs.md) — AGENTS.md shrink, init rework, docs
