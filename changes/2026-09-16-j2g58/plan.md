# 2026-09-16-j2g58: Deterministic change status endpoint

- Change ID: 2026-09-16-j2g58
- Created: 2026-09-16
- Branch: —
- Status: see [ledger.md](ledger.md)

## Objective and context

Every change-overall-status transition should have a deterministic, code-mediated
path that keeps the change ledger and the root-ledger row in agreement. Today only
close (→ Done) and reopen (→ In progress) do — both go through
`store.SetChangeStatus`, which atomically rewrites both files. The remaining
transitions (start of work: Planned → In progress; In progress ↔ Blocked) exist
only as prose in AGENTS.md, so agent sessions hand-edit the change ledger and
routinely forget the root-ledger row, producing rule-6 violations. This was
observed on 2026-09-16-7ueiv (ACC): the session set its own ledger to
`In progress`, the root row stayed `Planned`, and validation flagged the drift.

## Current behavior

- `POST /changes/scaffold` → `CreateChange`: writes `Planned` to both ledgers.
- `POST /changes/{id}/close` → `SetChangeStatus(Done)`: both ledgers, atomic.
- `POST /changes/{id}/reopen` → `SetChangeStatus(In progress)`: both ledgers, atomic.
- `POST /changes/{id}/move` → task rows in the change ledger only (no root row).
- All other overall transitions are prose-guided agent file edits; the root row is
  a second place agents must remember, and miss. Rule 6 detects the drift only
  after the fact.

## Target behavior

- New endpoint `POST /changes/{id}/status` with JSON body `{"status":"..."}`.
  Accepted values: `Planned`, `In progress`, `Blocked`. The handler delegates to
  `store.SetChangeStatus`, so the change ledger and root row are always updated
  together, conflict-safe, with no store changes required.
- `Done` is rejected by the new endpoint (409) with guidance to use
  `POST /changes/{id}/close`, preserving the user-gated close workflow
  (AGENTS.md status rules 8–9) even though the endpoint is agent-callable.
- `Cancelled` is not accepted by the endpoint (no cancellation path exists today;
  see Non-goals).
- The board gets a minimal control to set the three accepted statuses without
  leaving the page, and SSE `write` events refresh open views as today.
- AGENTS.md's status-workflow prose points agents at the deterministic path
  (endpoint or board) instead of hand-editing the `Overall status` line, and the
  embedded copy `internal/docs/assets/workflow_agents.md` is regenerated in
  lockstep (pinned by a drift test).

## Scope

- One new route + handler in `internal/server` (reusing `store.SetChangeStatus`).
- Handler tests covering success per status, both-file agreement, and error paths.
- A small board UI control for Planned / In progress / Blocked.
- Documentation: AGENTS.md workflow wording, embedded asset sync, README.

## Non-goals

- No `Cancelled` support on the endpoint: change cancellation has no existing
  route, UI, or reason-recording flow; designing that is its own change.
- No auto-healing of root rows on store reload (silently rewriting curated files
  would mask genuine drift).
- No store-layer changes; `SetChangeStatus` already validates the vocabulary and
  writes both files atomically.
- No changes to the task-row (per-task status) flow — `/move` already covers it.

## Design decisions

1. **Reuse `SetChangeStatus` unchanged.** It validates the status vocabulary,
   re-reads fresh on-disk content, writes both files via `WriteFileAtomic`, and
   notifies `write` events. The handler only parses/validates input.
2. **Keep Done user-gated.** The generic endpoint accepts only `Planned`,
   `In progress`, `Blocked`; `Done` returns 409 pointing at `/close`, so an agent
   can never close a change through the back door (status rules 8–9).
3. **Idempotent same-status calls are allowed.** Re-setting the current status
   just rewrites both files with today's date; no special-casing.
4. **Agent-facing parity.** Agent sessions can call the endpoint the same way
   they call `/changes/scaffold`, so the prose rule can say "use the endpoint"
   rather than "edit two files and hope".
5. **UI stays minimal.** One control on the board; no index-page or modal changes.

## Acceptance criteria

- `POST /changes/{id}/status` with `Planned`, `In progress`, or `Blocked` updates
  the change ledger's `Overall status` line **and** the root-ledger row; both
  agree afterwards (rule 6 stays clean for any sequence of calls).
- `Done` → 409; unknown/missing status → 400/422; unknown change → 404.
- `/close` and `/reopen` behave exactly as before.
- Handler tests cover each accepted status, the Done rejection, invalid input,
  and both-file agreement; `go vet ./...` and `go test ./...` are green;
  `lessmess validate` reports no rule violations.
- Board control sets the three statuses; open views refresh via SSE.
- AGENTS.md, `internal/docs/assets/workflow_agents.md` (drift test), and README
  all describe the endpoint and the updated agent guidance.

## Tasks

1. [STS-00](tasks/00-status-endpoint.md) — Status endpoint, handler tests
2. [STS-01](tasks/01-board-status-control.md) — Board status control (depends on STS-00)
3. [STS-02](tasks/02-docs-workflow-sync.md) — Workflow wording, embedded asset, README (depends on STS-00)
