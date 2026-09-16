# 2026-09-16-le45q: Nested task decomposition

- Change ID: 2026-09-16-le45q
- Created: 2026-09-16
- Branch: —
- Status: see [ledger.md](ledger.md)

## Objective and context

A task being worked on may turn out to need detailed, multi-step work that contributes to the parent task. Today the workflow has exactly two levels — a change with a flat `tasks/` directory — so that complexity either bloats one task file or gets hidden inside unrelated tasks. This change introduces recursive task decomposition: any task can be expanded into a sub plan with its own tasks, which can themselves be expanded, with no fixed depth limit.

The user-facing model: *a task becomes a plan with its own tasks.* Structure changes only by deliberate action; status stays manually governed; progress is displayed everywhere but written nowhere except the row's own ledger.

## Current behavior

- Tasks are strictly flat: `changes/<id>/tasks/NN-slug.md` files only; `store.scan` skips directories inside `tasks/` (`internal/store/store.go`).
- One task table per change in `changes/<id>/ledger.md`, pinned six-column schema (`model.TaskColumns`); validation Rule 3 pins task file ↔ ledger row ↔ filename sequence.
- Task IDs are `PREFIX-NN` (or bare `NN`); opencode sessions bind to tasks via `taskTitleRe = ^([A-Z][A-Z0-9]{1,3}-\d+):` in `internal/server/mapping.go`, and `POST /changes/{id}/task-sessions` binds subagent sessions manually.
- The board is one six-column kanban per change; the task detail route `GET /changes/{id}/tasks/{file}` handles a single filename segment; the terminal task panel mirrors the board DOM client-side.
- The workflow text in AGENTS.md is embedded in the binary at `internal/docs/assets/workflow_agents.md` and pinned by a drift test.

## Target behavior

### On-disk structure

A decomposed task keeps its file and gains a sibling container directory mirroring the change structure:

```text
changes/<id>/
  ledger.md                 ← top-level task rows only (schema unchanged)
  tasks/
    00-engine.md            ← plain task, exactly as today
    00-engine/              ← container, created on decomposition
      ledger.md             ← child rows only, same pinned task table
      tasks/
        01-urls.md          ← subtask
        01-urls/tasks/…     ← recursion, unlimited depth
```

Rules:

- A directory under `tasks/` is valid only as the container of the sibling `NN-slug.md` task file (exact name match, minus `.md`); a task file without a container is a normal task.
- A container must contain `ledger.md` and `tasks/` (recursive extension of Rule 2). Loose supporting artifacts are allowed inside containers, as in change directories.
- No `plan.md` per container: the task file's own body remains the task's detail document.

### IDs and hrefs

- Dotted IDs: `EXC-00` → `EXC-00.00` → `EXC-00.00.01`. Per-level numbering starts at `00`, zero-padded two digits per segment, matching the existing naming rule.
- Hrefs are nested paths (`tasks/00-engine/tasks/01-urls.md`); `Store.TaskFile`'s path guard already rejects traversal and escapes.
- Rule 3 generalizes: the last dotted segment of the ID equals the filename sequence; frontmatter `id` equals the ledger row ID; task frontmatter stays exactly `id` and `title` (structure is positional, so no `parent` field).

### Container ledgers

- Same pinned task table (columns, statuses, row-order-as-priority) as change ledgers.
- Minimal headers instead of the change ledger's: `- Task: <task-id> (change <change-id>)` and `- Last updated:`. No overall status, branch, or plan link — those belong to the change.
- A task's status lives in exactly one place: the row in its governing ledger (the change ledger for top-level tasks, the parent container's ledger below that).

### Status semantics and rollup

- Rollup is **display-only**. Parent cards show a computed badge (e.g. `2/3 ✓`); every ancestor aggregates its whole subtree; the board header shows a change-level indicator; the index Tasks column counts recursively (shown as `x/y`). Nothing is ever written to any ledger by rollup.
- Counting rule: `Test` and `Done` count as complete; `Cancelled` drops out of the denominator — mirroring close-out semantics.
- Parent statuses stay manual and user-gated (`Done` only on explicit user instruction), per the existing vocabulary and workflow.
- **Close-out readiness is recursive**: closing a change requires every non-cancelled task in the entire tree to be `Test` or `Done`.

### Decomposition governance (user-instructed only)

- The tooling never decomposes anything on its own; no heuristic runs.
- Decomposition happens only when: (a) the user clicks **Expand** on a task card / detail, (b) the user explicitly instructs an agent in a session, or (c) it is part of a plan the user approved during planning.
- Agents *propose* decompositions when work reveals complexity; they never write containers unprompted. This rule lives in the workflow text and in the change and task-scoped prompts.
- Collapsing a bad decomposition = deleting the container directory (children go with it); plain file/git operations, no special tooling.

### Sessions

- `SessionEntry.Task` holds dotted IDs; `taskTitleRe` extends to `^([A-Z][A-Z0-9]{1,3}-\d+(?:\.\d+)*):` so subagent delegation with dotted prefixes auto-attaches children at any depth, and `reconcileTaskSessions` builds its known-task set from the whole tree.
- **Auto-spawn per sub plan**: when a container appears (UI expand endpoint or agent file writes picked up by the watcher), the server spawns an opencode session bound to that task, primed task-scoped. The prompt teaches: bound to task X of change Y; read the change plan, the governing ledger, and your own task file first; your task is decomposed under its container — work or delegate children with dotted prefixes; keep the container ledger current; stop at `Test`; never `Done`; never scaffold; propose (never perform) further decomposition.
- Auto-spawn is best-effort and **once-only**: a per-node marker in `.lessmess/` tooling state (new file, one-file-per-feature pattern — `.lessmess/autosession.json`) prevents respawn storms; spawn failures log a warning, leave the marker unset, and the sub-board's Start/Continue session button is the manual retry. Unlinking a session later never respawns.
- Session titles use the `EXC-00: title` shape so they match the binding prefix.
- Sub-boards get a per-task Continue button (localStorage `tt-last-session:<change>/<task>`); the root board's sessions panel keeps the full change-wide list; a sub-board's panel shows sessions bound to that exact task. No terminals auto-open.

## Scope

- Workflow text (AGENTS.md) plus its embedded copy and drift test.
- `internal/model`: container-ledger parsing/rendering, dotted-ID helpers, shared task-row mutation reuse, scaffold templates.
- `internal/store`: recursive scan into a task tree; `DecomposeTask`; `CreateTask` with a parent; `MoveTask` via governing-ledger resolution; recursive validation; recursive close-out readiness; watcher coverage of new subdirectories.
- `internal/server`: nested task-detail route; expand endpoint; drill-down board (`?task=`); recursive index counts; dotted session binding; task-scoped prompt; auto-spawn reconciliation; per-task session surfaces.
- `web/`: card badges and expand affordance, drill-down wiring, breadcrumb, add-subtask form, sessions panel scoping, detail-modal link interception for nested hrefs, terminal panel mirroring, CSS.
- `README.md` for the new board UX and workflow concepts.

## Non-goals

- Re-parenting tasks between containers (delete/re-create instead); may be added later.
- Auto-derived statuses: no tooling ever writes a status on rollup's behalf.
- Per-container `plan.md` files.
- Cross-level drag-and-drop moves (moves stay within a governing ledger).
- Any migration of existing data.

## Design decisions

1. **Nested containers over flat-with-Parent-column**: additive (no ledger migration, existing changes stay byte-valid), recursion by construction, and matches the "task becomes a plan" model. The alternative broke the pinned schema for every existing ledger.
2. **Status in the governing ledger only** keeps a single source of truth and preserves the user-gated `Done` rule; all progress numbers are derived at render time.
3. **Display-only rollup** chosen over an enforced gate or auto-derivation during planning (2026-09-16).
4. **Re-parenting out of scope v1** to keep move semantics and ID stability simple.
5. **Auto-spawn sessions per sub plan** (user choice over no-auto-spawn and change-level-only): every decomposed node is immediately agent-ready; mitigations are the once-only marker, best-effort spawn, and no auto-opened terminals.
6. **User-instructed decomposition only** (user choice over agent-initiative and UI-only): agents propose, never act; planning-phase nesting is allowed as part of an approved plan.
7. **Task frontmatter stays `id`+`title`**: hierarchy is positional (directory nesting), so no schema extension of task files is needed.

## Detailed implementation approach

Layered bottom-up; each layer lands with tests before the next begins.

1. **Spec first** (NTD-00): write the structure, ID, ledger, validation, and governance rules into AGENTS.md; regenerate `internal/docs/assets/workflow_agents.md` with the documented `awk` command; keep the drift test green. This pins the contract the code implements.
2. **Model** (NTD-01): `ParseTaskLedger`/`RenderTaskLedger` with the minimal container headers and the same `TaskColumns` table; extract or share the row-mutation core (`AppendTask`, `MoveTask`, `syncTable`) so change and container ledgers reuse it; dotted-ID helpers (`ParentID`, `ChildID`, `LastSegment`, depth checks).
3. **Store** (NTD-02): recursive `scan` building a task tree on `Change` (node: task file, href, parent, children, governing ledger); `TaskByID`; `DecomposeTask` (creates `ledger.md` + `tasks/`, refuses if the container exists); `CreateTask(change, parent, title)` numbering within the container; `MoveTask` resolving the governing ledger; `SubtreeStats` for rollups; verify/extend `watch.go` so newly created nested directories are watched.
4. **Validation** (NTD-03): recursive Rule 2/3 (container ↔ sibling file match, dotted sequence agreement, per-level row/file consistency), stray-directory violations, recursive close-out readiness helper consumed by the server.
5. **Server** (NTD-04): `GET /changes/{id}/tasks/{file...}` (replace the single-segment route; keep the traversal guard); `POST /changes/{id}/expand` (`{task}`); `POST /changes/{id}/tasks` gains `parent`; board reads `?task=` and renders the subtree fragment with breadcrumb; `changeSummary`/index counts become recursive; JSON API exposes tree/stats.
6. **Sessions** (NTD-05): dotted `taskTitleRe`; task-scoped prime prompt; auto-spawn reconciliation on reload for container nodes without a session, gated by `.lessmess/autosession.json`; expand endpoint spawns directly and sets the marker; per-task Continue and panel scoping.
7. **Web UI** (NTD-06): board fragment badges (`x/y ✓` via `SubtreeStats`) and expand affordance; drill-down navigation; add-subtask form targets the viewed level; detail-modal link interception generalized for nested `tasks/…` hrefs and container `../ledger.md`; terminal panel keeps mirroring the current board; CSS for badges/breadcrumb.
8. **Docs + verification** (NTD-07): README section; full check suite; documented manual browser pass.

## File-level impact

- `AGENTS.md`, `internal/docs/assets/workflow_agents.md` (regenerated).
- `internal/model/`: `ledger.go`, `serialize.go`, `templates.go`, new task-ledger file, `taskfile.go` (unchanged contract), tests + new testdata fixtures.
- `internal/store/`: `store.go`, `validate.go`, `watch.go`, tests.
- `internal/server/`: `server.go` (routes/views), `mapping.go` (`taskTitleRe`, reconciliation), `changesession.go` (task prompt, governance wording), new autosession state file handling, `lifecycle.go` (recursive close-out), tests.
- `web/templates/`: `board.html`, `partials.html`; `web/static/`: `app.js`, `app.css`.
- `README.md`.

## Data, schema, and configuration changes

- New canonical-data shape only (additive); no existing file format changes except AGENTS.md prose. No settings, no `.lessmess` schema changes beyond the new `autosession.json` tooling-state file.

## Safety, migration, and rollback

- No migration: existing changes and ledgers remain byte-valid; the only behavioral edge is that directories under `tasks/` (previously silently ignored) are now validated — only affects repositories that stashed junk there.
- Rollup never writes, so no data-corruption path from display code. All writes keep the re-read/atomic-write discipline.
- Rollback = revert the commit and rebuild; nested trees created meanwhile parse as violations (stray dirs) rather than crashing — acceptable for a reverted feature.

## Testing and verification strategy

- Model: parse/render round-trips for container ledgers; dotted-ID helpers; strict rejection of malformed headers.
- Store: nested fixture trees — scan, decompose, create-child numbering, move via governing ledger, validation violations (stray dir, mismatched name, broken sequence), watcher recursion, `SubtreeStats`.
- Server: route tests for nested detail (including traversal rejection), expand endpoint, drill-down render, recursive counts, dotted binding regex, auto-spawn once-only behavior with a fake opencode client, recursive close-out gate.
- Repo-level: `go vet ./... && go test ./...`; rebuild binary; `lessmess validate` clean on this repository's own tree after the AGENTS.md edit; drift test green.
- Manual browser pass (NTD-07): expand → badge → drill-down → subtask lifecycle → close-out gate.

## Observability

- `slog` lines for auto-spawn attempts, failures, and reconciliation counts, following existing patterns; no new metrics surface.

## Rollout sequence

Single change, landed task-by-task in dependency order; rebuild the binary after NTD-00 (embed) and again at completion; no feature flags needed since structure is inert until a container exists.

## Risks and mitigations

- **Route wildcard**: `{file...}` replaces `{file}`; single-segment links must keep working — covered by route tests.
- **Watch recursion**: fsnotify must track newly created nested dirs — verify watch-set rebuild; covered by a store test.
- **Spawn storms**: once-only marker plus nil-service guard and per-reload batching cap.
- **Detail-modal link regressions**: the client-side `.md` interception is subtle; extend its tests-by-inspection during the manual pass and keep the mapping table exhaustive.
- **Drift test**: AGENTS.md and the embedded copy must regenerate together exactly; the documented `awk` command is the only sanctioned method.

## Acceptance criteria

1. A three-level nested change validates clean under `lessmess validate`; a flat legacy change behaves byte-identically to before.
2. Board: expand affordance creates a container (file writes via agent included); cards show correct recursive badges; drill-down with breadcrumb works; add-subtask form targets the viewed level.
3. Progress propagates visually to ancestors, board header, and index counts without any ledger write; statuses move only via explicit actions; `Done` remains user-gated.
4. Close change is blocked while any non-cancelled task at any depth is not `Test`/`Done`, and allowed when the tree is complete.
5. Each container gets exactly one auto-spawned, task-scoped session (once-only; unlink does not respawn; service-down degrades to manual Start); dotted-prefix delegation attaches subagent sessions to subtask cards.
6. `go vet ./... && go test ./...` green; drift test green; README documents the UX.

## Ordered task breakdown

1. [NTD-00 — Extend workflow spec](tasks/00-workflow-spec.md)
2. [NTD-01 — Model: container ledgers and dotted IDs](tasks/01-model-container-ledgers.md)
3. [NTD-02 — Store: recursive task tree and operations](tasks/02-store-task-tree.md)
4. [NTD-03 — Store: recursive validation and close-out readiness](tasks/03-store-validation.md)
5. [NTD-04 — Server: nested routes, expand, drill-down board](tasks/04-server-board-api.md)
6. [NTD-05 — Sessions: dotted binding, task prompt, auto-spawn](tasks/05-sessions-autospawn.md)
7. [NTD-06 — Web UI: badges, drill-down, modal links](tasks/06-web-ui.md)
8. [NTD-07 — README and end-to-end verification](tasks/07-readme-verification.md)

Execution status lives in [ledger.md](ledger.md).
