# Repository Agent Instructions

## Change management

Use the repository's `changes/` directory to plan and track implementation work. Apply this workflow to new code, configuration, schema, infrastructure, and other repository changes unless the user explicitly requests a different structure.

Read-only investigation, explanation, and review work does not require a change record unless it is being turned into an implementation plan.

### Change directory naming

Every new change gets one directory directly under `changes/` using this exact format:

```text
changes/YYYY-MM-DD-ChangeNumber/
```

Rules:

1. Use the current local date in `YYYY-MM-DD` format.
2. `ChangeNumber` is a zero-based integer scoped to that date.
3. The first change created on a date is `0`.
4. Each additional change on the same date increments the highest existing number by one.
5. Do not reuse an earlier number if a directory was removed or a numbering gap exists.
6. Before allocating a number, inspect existing `changes/YYYY-MM-DD-*` directories.
7. Continue using an existing change directory when the request is a continuation of the same objective. Create a new number for an independent objective.

Example:

```text
changes/
├── 2026-09-09-0/
├── 2026-09-09-1/
└── 2026-09-10-0/
```

### Required structure

The `changes/` tree has this layout:

```text
changes/
├── ledger.md
├── archive/                  (optional; see "Archival")
├── YYYY-MM-DD-ChangeNumber/
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
3. Task statuses live exclusively in each change's `ledger.md`.
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
- The file header must state: "Task statuses live exclusively in each change's `ledger.md`."

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

2. The frontmatter `id` must match the task's row in the change ledger, and its numeric suffix must match the filename sequence.
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

### `ledger.md`

The ledger is the single source of truth for execution status. Do not rely on task-file headings, chat updates, or the plan to represent current status.

The ledger must include:

- Change ID.
- Link to `plan.md`.
- Branch name when applicable.
- Overall status.
- Last-updated date.
- Status definitions.
- One row for every task, linking to its task file.
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
3. Before modifying implementation files for a task, change that task to `In progress`, update its date, and set the overall status to `In progress`.
4. Record material findings, decisions, scope changes, and blockers in the relevant task notes. Summarize important decisions in the ledger decision log.
5. When a blocker is resolved, return the task to `In progress` and record the resolution.
6. Mark a task `Test` only after its documented verification and completion criteria pass. Record concise verification evidence. `Test` means the agent considers the task complete; it awaits user acceptance.
7. If verification fails, keep the task `In progress` or mark it `Blocked`; do not mark it `Test` based solely on implementation being written.
8. Only the user moves a task to `Done` — by dragging the card on the board or by explicitly instructing the agent. Agents must never set a task to `Done` on their own initiative, even when all criteria pass.
9. Only the user closes a change. When every non-cancelled task is `Test` or `Done`, all change-level acceptance criteria pass, and no required work remains, agents leave the overall status `In progress` and report the change ready for close-out. Agents must never set the overall status to `Done` themselves; the user closes the change explicitly (via the board's Close button or a direct instruction).
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

1. Change directories match `changes/YYYY-MM-DD-N/` with correct zero-based per-date allocation.
2. Every change directory contains `plan.md`, `ledger.md`, and `tasks/`.
3. Every task file has exactly one ledger row and vice versa; frontmatter `id`, ledger `Task` cell, and filename sequence agree.
4. Task and change statuses use only the defined vocabularies.
5. Ledger tables match the pinned schemas, with no literal `|` inside cell text.
6. The root ledger has one row per change directory (including archived ones), and root-ledger overall status agrees with each change's `ledger.md`.
7. Ledger row order reflects the intended priority order.

### Tooling state

Tools (servers, validators, UIs) keep their own state in a `.lessmess/` directory at the repository root, which must be gitignored. Tooling state must never live inside `changes/`; that tree contains only canonical, human/agent-authored data.

### Repository docs (STRUCTURE.md and per-folder AGENTS.md)

lessmess maintains agent-facing docs in every covered folder (coverage is set by the committed `agentsdocs.json`; hidden dirs and `changes/` are never covered):

- `STRUCTURE.md` is a machine-owned map of the folder's entries, their purposes, and child rollups, plus freshness metadata. It is regenerated wholesale. Never hand-edit inside its `tasktracker:begin` / `tasktracker:end` HTML-comment markers.
- `AGENTS.md` (in a covered folder) holds curated learnings and instructions for that area. Content outside the markers is human/agent-authored and preserved byte-for-byte; the marker-guarded auto section is machine-maintained (new learnings cite their source change ID, `seed`, or `manual`).

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

- (seed) Root holds only entry points and docs: `cmd/` the CLI, `internal/` the packages, `web/` the embedded assets; `changes/` is the canonical workflow tree and `.lessmess/` is gitignored tooling state.
- (seed) Build with `CGO_ENABLED=0 go build -o lessmess ./cmd/lessmess`; verify with `go vet ./... && go test ./...`; check workflow data with `lessmess validate`.
- (seed) Covered folders carry a doc pair: `STRUCTURE.md` (machine-owned inside its markers) and `AGENTS.md` (curated learnings, marker-guarded auto section). Read them when entering a folder; keep them accurate when changing that area.
- (seed) The workflow text of this file (everything above the markers) is embedded in the binary at `internal/docs/assets/workflow_agents.md` (go:embed); a drift test pins them — update both together (`awk '/^<!-- tasktracker:begin/{exit} {print}' AGENTS.md > internal/docs/assets/workflow_agents.md`).
- (manual) CLI subcommands are `serve`, `validate`, `init`, and `docs seed`; `--dir` selects the repository root and one process serves one repository.
- (manual) `agentsdocs.json` controls docs coverage with include and exclude globs; `web/static` is the only exclusion, and hidden dirs plus `changes/` are never covered, so the whole docs subsystem is inert without that file.
- (manual) Root PNGs are ad-hoc screenshots, not build inputs: `overlap.png` and `mine.png` are gitignored, while `ui.png` is committed.
- (manual) `opencode.json` pre-approves unattended agent sessions with allow-all inside the project and denies for external directories, `.env` reads, and `git push`.
- (2026-09-12-7) The docs subsystem is opt-in via a committed root `agentsdocs.json`; closing a change enqueues one serialized gardener job (`.lessmess/docs-queue.json`) for its touched covered dirs, and the server's `POST /docs/refresh` endpoint reconciles dirs left stale when the opencode service was unavailable.
- (2026-09-12-7) `lessmess init` bootstraps a workflow-ready repo (root `AGENTS.md` workflow text, `changes/` skeleton, `.gitignore`, starter `opencode.json`, default `agentsdocs.json`) merge-safely and idempotently; `docs seed` generates the first-pass doc pairs bottom-up and is resumable via `.lessmess/docs-seed.json`.
- (2026-09-12-8) `GET /explorer` renders the covered-directory tree straight from the docs system (each node's purpose and entry blurbs come from STRUCTURE.md) and adds a per-directory chat action; it is the UI dogfood of `STRUCTURE.md`/`AGENTS.md` content, so its value tracks docs quality.
- (2026-09-12-8) The explorer is served by `internal/server/explorer.go` with a live-updating tree (`docs` events on the existing `/events` SSE stream) and tree/chat behavior in `web/static/app.js`; it degrades to guidance when the repo has no `agentsdocs.json`.
- (2026-09-12-11) The explorer is a master/detail UI: a dirs-only navigation tree on the left and a reading pane on the right loaded per directory from `GET /explorer/detail?dir=<rel>`; selection is client-side state and the directory chat button now lives only in the detail-pane header.
- (2026-09-12-9) Docs findings now surface in a header notification bell plus modal (hidden at zero, amber normally, red if any error finding) whose "Refresh stale docs" button posts `/docs/refresh`; the red banner is reserved for `changes/` violations. The endpoint enqueues one manual gardener job for the union of queue-stale and hash-stale dirs (empty union reports nothing to refresh), and docs SSE events re-check findings and the explorer tree live.
- (manual) The root `lessmess` entry is the local compiled binary from `cmd/lessmess` (gitignored build output), not a source directory; rebuild it with the documented `go build` command when testing CLI behavior.
- (manual) `README.md` is the human-facing overview of the same CLI, board, and docs system; update it when user-visible commands, endpoints, or workflows change.
- (manual) The project was renamed from tasktracker to lessmess: the module is `lessmess`, the binary builds from `cmd/lessmess`, and tooling state lives in `.lessmess/`; startup calls `store.MigrateStateDir` to rename a legacy `.tasktracker/` dir when `.lessmess/` does not yet exist.
- (manual) Root `changes/ledger.md` is the change-level index only: each row links to that change's `plan.md`, and per-task status lives solely in the change's own `ledger.md`.
- (2026-09-12-12) `README.md` documents the board UI affordances introduced here: the header Changes/Explorer menu with a server-applied active-route highlight, the newest-first change list, and the board's one-click Continue/Start session button.
- (2026-09-12-14) The rename is Tier 1+2 only: module `lessmess`, binary at `cmd/lessmess`, and `.lessmess/` tooling state; the doc-marker syntax (`tasktracker:begin`/`tasktracker:end`), `tt-` frontend prefixes, and the repository folder name stay unchanged.
- (2026-09-12-14) State migrates automatically: `store.MigrateStateDir` renames a legacy `.tasktracker/` to `.lessmess/` only when the new dir is absent (no merge, no delete), and is called by `serve`, `validate`, and `docs seed` before state access plus defensively in `server.New`.
- (2026-09-12-14) lessmess's visual identity is `web/static/icon.svg` (white "lm" on the `#e8641f` accent) with generated rasters `icon-512.png`, `favicon.ico` (16/32/48), and `apple-touch-icon.png`; `layout.html` links the favicon set and the header renders the icon as the brand.
- (2026-09-12-15) The workflow's task vocabulary is now six statuses: `Test` sits between `Blocked` and `Done`, agents stop at `Test` once verification passes, `Done` is user-gated (set only on explicit user instruction or a manual card drag), and close-out readiness is all non-cancelled tasks `Test` or `Done`.
- (2026-09-13-0) Committing is available two ways: per change from the board and repo-wide via the index page's Commit all flow (`GET /api/git/status` for a preview, `POST /api/git/commit` to spawn the session, both documented in `README.md`); the server only reads git for preview and delegates the actual commit to an opencode session.
- (2026-09-13-0) Adding render-time `git` scans to the index page exposed a self-sustaining reload loop: macOS atime updates from git reading modified `changes/` files surface as attribute-only fsnotify `CHMOD` events, so both the store and docs watchers must ignore those (`ev.Op&^ fsnotify.Chmod == 0`) while still notifying on content ops.
- (manual) A manual docs reconciliation needs no `changes/` record: it fills STRUCTURE.md placeholder purposes and appends annotated AGENTS.md learnings for the listed directories only, leaving implementation files and `changes/` untouched.
- (manual) Each covered STRUCTURE.md is regenerated wholesale from the tree, so entries are never hand-added, removed, or reordered; only an em dash purpose cell may be replaced with prose, and table cells must avoid pipe characters and newlines.
- (2026-09-13-1) Embedded browser terminals are now chrome-free: `internal/server/tuiconfig.go` generates `.lessmess/xdg/opencode/cli.json` per spawn and `internal/terminal` injects `XDG_CONFIG_HOME` into the PTY child, while `web/` adds a client-side task panel beside the terminal; the generated config is tooling state under the existing gitignored `.lessmess/`.
- (manual) User-configurable settings are layered and optional: committed root `lessmess.json` is project policy and gitignored `.lessmess/settings.json` is a personal override, with effective values merging personal over project over built-in defaults. Fields are tri-state so an unset value inherits and clearing a value restores inheritance.
- (manual) Settings are read statelessly (every use re-reads the small files, so external edits need no watcher) and written atomically per layer; malformed files fail open to defaults. The server exposes them at `/settings` and `/api/settings` (plus `/api/settings/options`).
- (manual) Root `lessmess.json` is written section-scoped by the Settings page, so empty `prompts`/`git`/`ui`/`docs` objects alongside a populated section are normal; the file itself is optional and may be deleted to restore built-in defaults.
- (manual) `.gitignore` anchors the binary ignore to the root (`/lessmess`) so `cmd/lessmess/` source stays tracked, and it also ignores `.lessmess/`, the ad-hoc screenshots (`ui.PNG`, `overlap.png`, `mine.png`), and `.DS_Store`.
- (manual) The web UI is embedded at build time (`//go:embed templates static` in `web/web.go`, served via `internal/server/render.go`), so template and static-asset changes are invisible in the running server until the binary is rebuilt with the documented `go build` command.
- (2026-09-13-3) Root `lessmess.json` sets `prompts.discussion` to a question-policy addendum: discussions open with zero questions, anything resolvable from the repo or a stated assumption is not asked about, and an unavoidable question is multiple choice only (2–5 labeled options, one recommended). The authoritative text is pinned in `changes/2026-09-13-3/plan.md`.
- (2026-09-13-3) Prompt addenda are appended verbatim at session-creation time by `Server.promptWith` (`internal/server/settings.go`); base prompts are never modified and existing sessions keep their original prime, so addendum changes only affect newly created sessions.
- (2026-09-13-3) Iterating on or reverting the discussion policy is config-only: `PUT /api/settings?scope=project` with `{"prompts":{"discussion":"<text>"}}` (empty string restores the built-in default); no rebuild is needed since settings are re-read on every use.
- (2026-09-13-2) Save-time validation exists because the live opencode service silently accepts unknown agent/model values at session creation — the session then never runs and no 400 fires the spawn fallback — so `PUT /api/settings` validates submitted values against the live, repo-scoped agent/model lists (422 on unknown; skipped when the service is unreachable).
- (2026-09-13-2) Session agent/model settings apply only to sessions created after the save — never retroactively — and degrade gracefully: `Server.spawnSession` retries a plain create when the service rejects the configured values with 400, so a stale name committed in `lessmess.json` never blocks session creation.
- (manual) `lessmess serve` on a repo without a `changes/` tree starts in setup mode: `store.Open` fails with `ErrNoChanges`, `main` builds `server.NewSetup` instead, and bootstrap hot-swaps the full handler in-process; the index page also shows a finish-setup banner until completed or dismissed via `/api/setup/dismiss`.
- (2026-09-13-4) First-run onboarding is a web wizard: `GET /setup` walks five steps (prereqs, bootstrap, agent/model, docs, finish) over `/api/setup/*`; bootstrap creates the init artifacts and hot-opens the store in-process, and completion or dismissal persists to `.lessmess/onboarding.json` so the wizard never nags again (Settings re-entry reopens it).
- (2026-09-13-4) Docs coverage and docs generation are separate onboarding choices: bootstrap writes `agentsdocs.json` only when coverage stays enabled (`docs.InitWithOptions`, with the wizard's exclusion picks folded in), while initial seeding is an explicit, budgeted opt-in step — skipping it means zero LLM sessions, and `POST /docs/seed` on the normal server can resume or redo a run later.
- (2026-09-13-4) Every docs-seed path now honors the configured session agent/model: the wizard's seed job and the CLI `docs seed` (via `server.SessionDefaults`) pass them through `docs.NewOpenCodeSummarizerWith`, so seeded sessions run with the same defaults the server would use.
- (2026-09-13-5) Change-close gardener jobs now also carry the covered ancestors of every touched dir (`DocsJob.Ancestors`) as review-and-fix targets: the prompt explicitly licenses fixing or deleting learnings the change invalidated, and the reply must account for each removal. Manual reconciliation jobs stay single-section — the lint routes parent-dir rot into refresh instead.
- (2026-09-13-5) A stale-reference lint flags learnings whose backticked, file-extension-bearing paths resolve to nothing: `ValidateDocs` warns `learning cites missing path` (bell + `lessmess validate`) and `POST /docs/refresh` unions the flagged dirs, naming the refs in the manual job's prompt. Resolution is generous (base-name match anywhere, sibling variants like `x.yaml` beside `x.yaml.template`, `.lessmess` state), so only literally-missing files flag.
- (2026-09-13-5) `docs.gardenerModel` (either settings layer) overrides the model for every queue-driven docs job — precedence gardener override → `session.model` → service default, 422 on unknown values at save; seed sessions keep the session defaults.
<!-- tasktracker:end -->
