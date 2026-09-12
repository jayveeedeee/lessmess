# 2026-09-12-15: User-gated Test status before Done

- Change ID: 2026-09-12-15
- Created: 2026-09-12
- Branch: —
- Status: see [ledger.md](ledger.md)

## Objective and context

Add a sixth task status, `Test`, as a human acceptance gate before `Done`.
Today an agent marks its own tasks `Done` once verification passes; the user
wants agents to stop at `Test` instead, with `Done` reserved for the user —
either by dragging the card themselves or by explicitly telling the agent to
move it. This mirrors the existing task-of-change rule that only the user
closes a change, pushed down to the task level.

## Current behavior

- Task-status vocabulary is five values, defined in
  `internal/model/model.go` (`StatusNotStarted`, `StatusInProgress`,
  `StatusBlocked`, `StatusDone`, `StatusCancelled`); `TaskStatusOrder` is the
  kanban column order consumed by `internal/server/server.go`,
  `internal/server/render.go`, and the board templates.
- The workflow text (root `AGENTS.md`, drift-pinned to
  `internal/docs/assets/workflow_agents.md` by
  `internal/docs/init_test.go`) instructs agents to mark a task `Done` once
  its verification and completion criteria pass (Status workflow steps 6–7),
  and to report a change ready for close-out when every non-cancelled task is
  `Done` ("Verification and handoff").
- New change ledgers are scaffolded from `internal/model/templates.go` with a
  five-row status-definitions table.
- The board has five columns; status pills are styled per status in
  `web/static/app.css` (`--st-*-bg/fg` variables in both the dark `:root`
  block and the light theme block, plus `.status-<name>` classes).
- Tests hard-code the five-column expectation:
  `internal/server/server_test.go` (`len(resp.Columns) != 5`),
  `internal/server/render_test.go` (expected status names).
- `README.md` line 43 documents "kanban board with five columns".

## Target behavior

- Six task statuses; column order: `Not started`, `In progress`, `Blocked`,
  `Test`, `Done`, `Cancelled` — `Test` immediately before `Done`.
- Workflow text: an agent sets a task to `Test` when implementation,
  verification, and completion criteria pass. An agent sets `Done` **only on
  explicit user instruction**; the user may always move a card to `Done`
  manually. `Test` means: verification passed, awaiting user acceptance.
- Handoff rule: an agent reports a change ready for close-out when all
  non-cancelled tasks are `Test` **or** `Done`; the user reviews, promotes
  tasks to `Done`, and closes the change.
- New ledgers include the `Test` row in their status-definitions table.
- The board renders the `Test` column with a distinct pill color in both
  themes; dragging a card into it records `Test` in the ledger via the
  existing move endpoint (no endpoint changes).

## Scope

- `internal/model/model.go` — add `StatusTest`, insert into `TaskStatusOrder`
  before `StatusDone`.
- `internal/model/templates.go` — add the `Test` row to the scaffolded
  status-definitions table.
- `AGENTS.md` + `internal/docs/assets/workflow_agents.md` — status vocabulary
  table, status workflow steps 6–7, and the "Verification and handoff"
  section; kept byte-identical (drift test).
- `web/static/app.css` — `--st-test-bg/fg` in both theme blocks and a
  `.status-test` pill rule.
- Tests — update the column-count/name expectations; add coverage that `Test`
  validates and serializes like any status.
- `README.md` — board bullet: six columns, note the user-gated `Done`.

## Non-goals

- No server-side enforcement of the `Done` gate: the server cannot reliably
  distinguish agent edits from user edits and stays a neutral editor (user
  decision). The rule lives in the workflow text, like user-closes-change.
- No change to overall change statuses, close/reopen, or the docs-gardener
  triggers.
- No migration of existing ledgers: historical `Done` tasks stay `Done`;
  `Test` is additive vocabulary, not a required transition.
- No additional review states (no "Ready for review", "QA", etc.).
- This change's own ledger keeps the scaffolded five-row definitions table
  (authored before the change); only the template for future ledgers is
  updated.

## Design decisions

- **Name `Test`** — the user's word; the column header renders `Test`.
- **Position just before `Done`** (user pick): `Not started`, `In progress`,
  `Blocked`, `Test`, `Done`, `Cancelled`. `Blocked` keeps its current slot,
  minimizing diff.
- **User-gated, not user-only** (user pick): an agent may set `Done` when the
  user explicitly says so (chat instruction or manual drag); strictly
  user-only was rejected as impractical since the user often delegates the
  click.
- **Enforcement is workflow-text + UI only** (user pick), matching the
  existing user-closes-change convention.
- **Ready signal = all `Test`/`Done`** (user pick): close-out readiness no
  longer requires the user to have already promoted every task.
- **Pill color**: a blue/teal tone distinct from the existing grey, orange,
  red, olive, and muted pills; exact `--st-test-*` values chosen at
  implementation for both themes.

## Safety, compatibility, and rollback

- Additive vocabulary change: every existing ledger remains valid — old
  statuses are untouched and no ledger is required to contain `Test`.
- `model.TaskStatus` validation iterates `TaskStatusOrder`, so `Test` becomes
  accepted everywhere (parser, move endpoint, drag-and-drop) with no further
  code paths to touch; confirmed no server logic counts `Done` tasks.
- Rollback: revert the commit; no data migration either direction.

## Testing and verification strategy

1. `go vet ./...` and `go test ./...` with updated expectations (six columns,
   `Test` pill rendered, template table includes the new row, drift test
   green after re-syncing the embedded workflow file).
2. `lessmess validate` on this repository: no violations.
3. Manual: rebuild the binary, open a board, drag a card into `Test` and then
   `Done`; confirm the ledger file records each status; spot-check the pill
   in dark and light themes.

## Acceptance criteria

1. Boards render six columns in the order above; moving a card to `Test`
   persists `Test` in the change ledger.
2. Root `AGENTS.md` and `internal/docs/assets/workflow_agents.md` state the
   `Test` status, the user-gated `Done` rule, and the `Test`-or-`Done`
   close-out readiness rule; the two files stay byte-identical above the
   markers (drift test passes).
3. Newly scaffolded changes include the `Test` row in their ledger
   status-definitions table.
4. The `Test` pill is legible in both themes.
5. `go vet ./...`, `go test ./...`, and `lessmess validate` all pass.

## Tasks

1. [TST-00](tasks/00-model-test-status.md) — Add Test status to model and ledger template
2. [TST-01](tasks/01-workflow-text-user-gated-done.md) — Workflow text: Test vocabulary and user-gated Done
3. [TST-02](tasks/02-test-column-styling.md) — Test pill styling in both themes
4. [TST-03](tasks/03-verify-tests-readme.md) — Update tests, README, and run full verification
