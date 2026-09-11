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
| Done | All verification and completion criteria in the task file have passed. |
| Cancelled | The task was intentionally removed from scope and the reason is recorded. |

Recommended overall statuses are `Planned`, `In progress`, `Blocked`, `Done`, and `Cancelled`.

### Status workflow

1. Create the plan, task files, and ledger before implementation begins.
2. Initialize every task as `Not started` and the overall change as `Planned`.
3. Before modifying implementation files for a task, change that task to `In progress`, update its date, and set the overall status to `In progress`.
4. Record material findings, decisions, scope changes, and blockers in the relevant task notes. Summarize important decisions in the ledger decision log.
5. When a blocker is resolved, return the task to `In progress` and record the resolution.
6. Mark a task `Done` only after its documented verification and completion criteria pass. Record concise verification evidence.
7. If verification fails, keep the task `In progress` or mark it `Blocked`; do not mark it `Done` based solely on implementation being written.
8. Only the user closes a change. When every non-cancelled task is `Done`, all change-level acceptance criteria pass, and no required work remains, agents leave the overall status `In progress` and report the change ready for close-out. Agents must never set the overall status to `Done` themselves; the user closes the change explicitly (via the board's Close button or a direct instruction).
9. The user may reopen a closed change (`Done` → `In progress`); agents then resume work from the ledger state. Close and reopen transitions are user actions and are recorded in both ledgers.
10. When cancelling a task or change, record the reason and any resulting scope adjustment.
11. Update ledger state in the same working change as the implementation it describes so status does not drift from the repository.

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

Tools (servers, validators, UIs) keep their own state in a `.tasktracker/` directory at the repository root, which must be gitignored. Tooling state must never live inside `changes/`; that tree contains only canonical, human/agent-authored data.

### Verification and handoff

When all tasks are complete, before reporting a change ready for close-out:

1. Run the tests and checks documented by each task.
2. Confirm the change-level acceptance criteria in `plan.md`.
3. Update every completed task row and record verification evidence.
4. Update the overall ledger's last-updated date, leaving the status `In progress` (only the user sets `Done`).
5. Ensure `plan.md`, `ledger.md`, and task files agree about scope and completion.
6. Report the change directory, implemented outcome, verification performed, and any remaining risks or follow-up tasks — and state that the change is ready for the user to close.

If implementation stops before completion, leave the ledger in the accurate current state and make the next executable step clear in the relevant task notes.
