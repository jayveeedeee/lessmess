# Repository Agent Instructions

## Change management

Use the repository's `changes/` directory to plan and track implementation work. Apply this workflow to new code, configuration, schema, infrastructure, and other repository changes unless the user explicitly requests a different structure.

Read-only investigation, explanation, and review work does not require a change record unless it is being turned into an implementation plan.

### Change directory naming

Every new change gets one directory directly under `changes/` using this exact format:

```text
changes/YYYY-MM-DD-xxxxx/
```

Rules:

1. Use the current local date in `YYYY-MM-DD` format.
2. `xxxxx` is a random suffix of exactly five lowercase alphanumeric characters (`[a-z0-9]`), generated when the change is scaffolded. Never choose it by hand or encode meaning or ordering in it.
3. Uniqueness comes from random generation: scaffolding checks existing directories for the date (including `changes/archive/`) and regenerates on the astronomically unlikely collision. Never allocate a sequential counter.
4. Directories with legacy numeric suffixes (`YYYY-MM-DD-N`, any number of digits) remain valid and are never renamed; mixed formats are normal.
5. Continue using an existing change directory when the request is a continuation of the same objective. Create a new change directory for an independent objective.

Example:

```text
changes/
├── 2026-09-09-k3x9q/
├── 2026-09-09-mz7t2/
└── 2026-09-10-0/          (legacy numeric IDs remain valid)
```

### Required structure

The `changes/` tree has this layout:

```text
changes/
├── ledger.md
├── archive/                  (optional; see "Archival")
├── YYYY-MM-DD-xxxxx/
│   ├── plan.md
│   ├── ledger.md
│   └── tasks/
│       ├── 00-short-task-name.md
│       ├── 01-short-task-name.md
│       └── ...
└── ...
```

Every change directory must contain `plan.md`, `ledger.md`, and `tasks/` as shown above.

Do not leave the authoritative plan at the repository root after creating the change directory. Related supporting artifacts may be added inside the same change directory when needed.

### Root ledger (`changes/ledger.md`)

The root ledger is the change-level index of the repository. It answers "what changes exist, and what is their overall status?" in one file.

Rules:

1. One row per change directory. Add the row when the change directory is created.
2. The root ledger is authoritative for change existence, task-ID prefixes, and overall change status only. It must never contain task rows.
3. Task statuses live exclusively in the ledger that governs the task: the change's `ledger.md` for top-level tasks, the parent task container's `ledger.md` below that.
4. Update a row only when the change is created, its overall status changes, or it is archived.
5. Rows are append-mostly; edit existing cells only for status, date, or link updates.

Exact table schema (column names are fixed):

```markdown
| Change | Title | ID prefix | Branch | Status | Created | Last updated |
| --- | --- | --- | --- | --- | --- | --- |
| [2026-09-11-0](2026-09-11-0/plan.md) | Example change | EXC | feat/example | In progress | 2026-09-11 | 2026-09-11 |
```

- `Change` links to the change's `plan.md` (or `archive/<id>/plan.md` after archival).
- `ID prefix` is the change-specific task-ID prefix (see "Task files"), registered here so prefixes stay unique across changes. Use `—` if the change uses unprefixed numeric IDs.
- `Status` is the overall change status: `Planned`, `In progress`, `Blocked`, `Done`, or `Cancelled`.
- Use `—` for empty cells; do not use a literal `|` inside cell text.
- The file header must state: "Task statuses live exclusively in the ledger that governs the task — the change's `ledger.md` for top-level tasks, the parent task container's `ledger.md` below that."

### `plan.md`

The plan is the authoritative description of the change's intended outcome and design. It must link to `ledger.md` and every task file.

Include, when applicable:

- Change ID, creation date, and branch.
- Objective and context.
- Current behavior.
- Target behavior or architecture.
- Scope and explicit non-goals.
- Design decisions and compatibility expectations.
- Detailed implementation approach.
- File-level impact.
- Data, API, message, configuration, or schema changes.
- Safety, security, rate-limit, migration, and rollback considerations.
- Testing and verification strategy.
- Observability requirements.
- Rollout sequence.
- Risks and mitigations.
- Acceptance criteria.
- Ordered task breakdown linking to the task files.

Keep the plan synchronized with material scope or design changes discovered during implementation. Do not silently implement a materially different design.

### Task files

Break the plan into concrete, reviewable tasks under `tasks/`.

Naming rules:

1. Prefix filenames with a zero-padded sequence beginning at `00`.
2. Follow the number with a short lowercase kebab-case description.
3. Assign each task a stable ID. The ID may use a short change-specific prefix, such as `ACC-00`, but its numeric suffix must correspond to the filename sequence. Register the change's prefix in the root ledger's `ID prefix` column.
4. Keep a task focused enough that its completion can be verified independently.

Format rules:

1. Every task file starts with YAML frontmatter containing exactly `id` and `title`:

   ```markdown
   ---
   id: EXC-00
   title: Short task title
   ---
   ```

2. The frontmatter `id` must match the task's row in its governing ledger (see "Task decomposition"), and its last numeric segment must match the filename sequence within its directory.
3. Frontmatter carries identity only. Status, ordering, and dependencies are recorded solely in the change ledger — never duplicate them into task files.
4. After the frontmatter, use this skeleton (content grows under each heading, but the heading set and order stay fixed):

   ```markdown
   # EXC-00: Short task title

   Status: see [../ledger.md](../ledger.md).

   ## Objective

   ## Dependencies

   ## Scope

   ## Implementation steps

   ## Verification

   ## Completion criteria

   ## Files affected

   ## Notes
   ```

Each task file must contain:

- Task ID and title.
- A link back to `../ledger.md` as the status source.
- Objective.
- Dependencies.
- Scope.
- Detailed implementation steps.
- Verification steps.
- Completion criteria.
- Expected files or areas affected.
- Notes for blockers, decisions, findings, and validation evidence.

Add a new task file and ledger row when implementation reveals material work that is not represented by an existing task. Do not hide unplanned work inside an unrelated task.

### Task decomposition (sub plans)

Any task may be decomposed into subtasks when it proves to need detailed work. A decomposed task keeps its file and gains a sibling container directory:

```text
tasks/
  00-engine.md            the task, unchanged
  00-engine/              container, exists only for decomposed tasks
    ledger.md             rows for the container's child tasks only
    tasks/
      01-urls.md          subtask
      01-urls/            recursion, any depth
```

Rules:

1. A directory under `tasks/` is valid only as the container of the sibling task file with the same name (minus `.md`). A task file without a container is a plain task.
2. Every container contains `ledger.md` and `tasks/`. Loose supporting artifacts may live inside a container, as in a change directory. A container never holds a `plan.md`; the task file itself remains the task's detail document.
3. Child IDs extend the parent's with dotted segments: `EXC-00` → `EXC-00.00` → `EXC-00.00.01`. Each level numbers from `00`, zero-padded to two digits, and the final segment must match the filename sequence within the container.
4. A task's status lives in its governing ledger: the change ledger for top-level tasks, the parent container's ledger below that. A container ledger uses the same pinned task-table schema, status vocabulary, and row-order-as-priority semantics as the change ledger, with minimal headers (`- Task: <id> (change <change-id>)` and `- Last updated:`) and one row per child task.
5. Decomposition is user-instructed only: a board action, an explicit instruction in a session, or part of a plan the user approved. Agents may propose decompositions when work reveals complexity, but never create containers unprompted. Removing a decomposition means deleting the container directory; its children go with it.
6. Progress rollup is display-only. Tools may compute and show aggregate progress for a task from its descendants (`Test` and `Done` count as complete; `Cancelled` leaves the denominator), but never write a status on rollup's behalf — statuses change only through the normal manual workflow, including the user-gated `Done`.

### `ledger.md`

The ledger is the single source of truth for execution status. Do not rely on task-file headings, chat updates, or the plan to represent current status. Each ledger governs one level of the task tree: the change ledger holds top-level tasks; each container's ledger holds that container's children (see "Task decomposition").

The ledger must include:

- Change ID.
- Link to `plan.md`.
- Branch name when applicable.
- Overall status.
- Last-updated date.
- Status definitions.
- One row for every task it governs, linking to its task file.
- Task dependencies.
- Per-task last-updated date.
- Concise notes describing current progress or blockers.
- A decision log for material implementation and scope decisions.

The task table must use this exact schema (column names are fixed):

```markdown
| Task | Title | Status | Depends on | Updated | Notes |
| --- | --- | --- | --- | --- | --- |
| [EXC-00](tasks/00-example.md) | Example task | Not started | — | 2026-09-11 | — |
```

- `Task` links to the task file; `Depends on` lists task IDs or `—`; use `—` for empty cells.
- Do not use a literal `|` inside cell text.
- Row order is the display and priority order: the top row is the highest priority. Reorder work by moving rows, not by adding priority fields.

Use these task statuses exactly:

| Status | Meaning |
| --- | --- |
| Not started | Work has not begun. |
| In progress | Implementation or verification is actively underway. |
| Blocked | Work cannot continue until a documented dependency, decision, approval, or external condition is resolved. |
| Test | Implementation and verification are complete; awaiting user acceptance before Done. |
| Done | Accepted by the user; agents set this only on explicit user instruction. |
| Cancelled | The task was intentionally removed from scope and the reason is recorded. |

Recommended overall statuses are `Planned`, `In progress`, `Blocked`, `Done`, and `Cancelled`.

### Status workflow

1. Create the plan, task files, and ledger before implementation begins.
2. Initialize every task as `Not started` and the overall change as `Planned`.
3. Before modifying implementation files for a task, change that task to `In progress`, update its date, and set the overall status to `In progress`. Set overall statuses only through the deterministic path — the board's status control or `POST /changes/{id}/status` with `{"status":"Planned"|"In progress"|"Blocked"}` — which updates the change ledger and the root-ledger row atomically; never hand-edit an `Overall status:` line. `Done` remains user-gated via close (rule 9); `Cancelled` has no endpoint, so cancelling a change still means editing both ledgers by hand and recording the reason (rule 11).
4. Record material findings, decisions, scope changes, and blockers in the relevant task notes. Summarize important decisions in the ledger decision log.
5. When a blocker is resolved, return the task to `In progress` and record the resolution.
6. Mark a task `Test` only after its documented verification and completion criteria pass. Record concise verification evidence. `Test` means the agent considers the task complete; it awaits user acceptance.
7. If verification fails, keep the task `In progress` or mark it `Blocked`; do not mark it `Test` based solely on implementation being written.
8. Only the user moves a task to `Done` — by dragging the card on the board or by explicitly instructing the agent. Agents must never set a task to `Done` on their own initiative, even when all criteria pass.
9. Only the user closes a change. When every non-cancelled task at every depth of the task tree is `Test` or `Done`, all change-level acceptance criteria pass, and no required work remains, agents leave the overall status `In progress` and report the change ready for close-out. Agents must never set the overall status to `Done` themselves; the user closes the change explicitly (via the board's Close button or a direct instruction).
10. The user may reopen a closed change (`Done` → `In progress`); agents then resume work from the ledger state. Close and reopen transitions are user actions and are recorded in both ledgers.
11. When cancelling a task or change, record the reason and any resulting scope adjustment.
12. Update ledger state in the same working change as the implementation it describes so status does not drift from the repository.

### Dependencies and execution order

- Respect task dependencies recorded in the ledger.
- Tasks without unmet dependencies may proceed independently.
- When execution order changes, update the ledger and plan rather than relying on conversation context.
- A blocked task does not block unrelated ready tasks unless the change cannot make meaningful progress without it.
- A change has a single active executor at a time; its `ledger.md` is a single-writer file. Coordinate handoffs through the ledger so concurrent edits do not conflict. The same applies to the root ledger across changes.

### Archival

1. Changes in `Done` or `Cancelled` for 30 days or more may be moved from `changes/<id>/` to `changes/archive/<id>/`. `archive/` is created on first use.
2. When archiving, update the change's root-ledger row so the link points to `archive/<id>/plan.md`; keep the status unchanged. Fix inbound links from other changes where practical.
3. The root ledger keeps rows for archived changes so the index stays complete.
4. Default views (human or tooling) treat non-archived changes as the active set.

### Validation

The workflow rules are machine-checkable. Tooling (validators, servers) must enforce them, and agents should self-check against them before marking work done:

1. Change directories match `changes/YYYY-MM-DD-(N|xxxxx)/` with a valid date — `N` is the legacy numeric suffix (any digits) and `xxxxx` is exactly five lowercase alphanumeric characters.
2. Every change directory contains `plan.md`, `ledger.md`, and `tasks/`; every task container likewise contains `ledger.md` and `tasks/`.
3. Every task file has exactly one row in its governing ledger and vice versa; frontmatter `id`, ledger `Task` cell, and filename sequence agree; every container directory matches its sibling task file, and dotted ID segments agree with the container nesting.
4. Task and change statuses use only the defined vocabularies.
5. Ledger tables match the pinned schemas, with no literal `|` inside cell text.
6. The root ledger has one row per change directory (including archived ones), and root-ledger overall status agrees with each change's `ledger.md`.
7. Ledger row order reflects the intended priority order.

### Tooling state

Tools (servers, validators, UIs) keep their own state in a `.lessmess/` directory at the repository root, which must be gitignored. Tooling state must never live inside `changes/`; that tree contains only canonical, human/agent-authored data.

### Repository docs (STRUCTURE.md and per-folder AGENTS.md)

lessmess maintains agent-facing docs in every covered folder (coverage is set by the committed `agentsdocs.json`; hidden dirs and `changes/` are never covered):

- `STRUCTURE.md` is a machine-owned map of the folder's entries, their purposes, and child rollups, plus freshness metadata. It is regenerated wholesale. Never hand-edit inside its `tasktracker:begin` / `tasktracker:end` HTML-comment markers.
- `AGENTS.md` (in a covered folder) holds curated learnings and instructions for that area. Content outside the markers is human/agent-authored and preserved byte-for-byte; the marker-guarded auto section is machine-maintained — it holds only current-state learnings (how the code works now, never change narration), with no provenance prefixes, at most 15 entries per file, consolidated in place whenever a change supersedes existing content. Attribution lives in git history, not in the text.

When entering a folder, read its `STRUCTURE.md` for orientation and its `AGENTS.md` for local learnings before editing. Keep both accurate when you change that area (per the update rule above): edit only outside the markers; the doc gardener maintains the auto sections when a change closes.

### Verification and handoff

When all tasks are complete, before reporting a change ready for close-out:

1. Run the tests and checks documented by each task.
2. Confirm the change-level acceptance criteria in `plan.md`.
3. Move every completed task to `Test` (unless the user has already accepted it as `Done`) and record verification evidence.
4. Update the overall ledger's last-updated date, leaving the status `In progress` (only the user sets `Done`).
5. Ensure `plan.md`, `ledger.md`, and task files agree about scope and completion.
6. Report the change directory, implemented outcome, verification performed, and any remaining risks or follow-up tasks — and state that the change is ready for the user to review (`Test` → `Done`) and close.

If implementation stops before completion, leave the ledger in the accurate current state and make the next executable step clear in the relevant task notes.

<!-- tasktracker:begin -->
## Learnings

- Root holds only entry points and docs: `cmd/` the CLI, `internal/` the packages, `web/` the embedded assets; `changes/` is the canonical workflow tree and `.lessmess/` is gitignored tooling state.
- Build with `CGO_ENABLED=0 go build -o lessmess ./cmd/lessmess`; verify with `go vet ./... && go test ./...`; check workflow data with `lessmess validate`. The root `lessmess` entry is gitignored build output, not a source directory.
- The web UI is embedded at build time (`//go:embed templates static` in `web/web.go`, served via `internal/server/render.go`), so template and static-asset changes are invisible in the running server until the binary is rebuilt and restarted.
- CLI subcommands are `serve`, `validate`, `init`, and `docs seed`; `--dir` selects the repository root and one process serves one repository.
- The module is `lessmess` (renamed from tasktracker): binary at `cmd/lessmess`, tooling state under `.lessmess/`; the doc-marker syntax (`tasktracker:begin`/`tasktracker:end`), `tt-` frontend prefixes, and the repository folder name are unchanged. `store.MigrateStateDir` renames a legacy `.tasktracker/` to `.lessmess/` only when the new dir is absent, called by `serve`, `validate`, `docs seed`, and defensively in `server.New`.
- The docs subsystem is opt-in: a committed root `agentsdocs.json` sets coverage with include/exclude globs (`web/static` is the only exclusion; hidden dirs and `changes/` are never covered), and without that file everything docs-related no-ops.
- Each covered folder carries a doc pair: `STRUCTURE.md` (machine-owned map, regenerated wholesale inside its markers — only an em dash purpose cell may be hand-replaced with prose, and table cells must avoid pipes and newlines) and `AGENTS.md` (current-state learnings, machine-maintained inside the markers, at most 15 per file). The root workflow text above the markers is embedded at `internal/docs/assets/workflow_agents.md` (go:embed) and drift-pinned — update both together (`awk '/^<!-- tasktracker:begin/{exit} {print}' AGENTS.md > internal/docs/assets/workflow_agents.md`).
- `lessmess init` bootstraps a workflow-ready repo (workflow text, `changes/` skeleton, `.gitignore`, starter `opencode.json`, default `agentsdocs.json`) merge-safely and idempotently; `docs seed` generates first-pass doc pairs bottom-up and resumes via `.lessmess/docs-seed.json`.
- Closing a change enqueues one serialized gardener job (`.lessmess/docs-queue.json`) for the touched covered dirs plus their covered ancestors (fix/delete license, removals accounted in the reply); `POST /docs/refresh` unions queue-stale, hash-stale, and lint-flagged dirs, where the stale-reference lint warns `learning cites missing path` (bell + `lessmess validate`, never an error) when a learning's backticked path resolves to nothing.
- `opencode.json` pre-approves unattended agent sessions with allow-all inside the project and denies for external directories, `.env` reads, and `git push`.
- Settings are layered (committed root `lessmess.json` = project policy, gitignored `.lessmess/settings.json` = personal override, defaults beneath), read statelessly on every use, and fail open when malformed. `prompts.*` addenda append at session creation only (existing sessions keep their prime), and `docs.gardenerModel` overrides the model for queue-driven docs jobs.
- `README.md` is the human-facing overview of the CLI, board, and docs system; update it when user-visible commands, endpoints, or workflows change.
- Root PNGs are ad-hoc screenshots, not build inputs (`ui.png` committed; `overlap.png`, `mine.png` gitignored); `.gitignore` anchors the binary ignore to the root (`/lessmess`) so `cmd/lessmess/` source stays tracked.
- `changes/` documents never record session IDs: session-to-change/task mappings live only in `.lessmess/sessions.json` tooling state.
- The explorer is the docs-system UI: a dirs-only navigation tree with a per-directory detail pane loaded from `GET /explorer/detail`, live via `docs` events on the `/events` SSE stream; its value tracks docs quality.
<!-- tasktracker:end -->
