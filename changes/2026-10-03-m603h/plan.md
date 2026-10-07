# 2026-10-03-m603h: Recover queued chat messages

- Change ID: 2026-10-03-m603h
- Created: 2026-10-03
- Branch: —
- Status: tracked in the tool-owned JSON state (.lessmess/workflow/)

## Objective and context

Queued follow-ups must remain visible and recoverable after an execution fails or is stopped. A reported conversation ended with a generic response failure while the user's instruction remained labelled Queued. The exact incident has not been inspected directly; repository investigation confirms that the current UI only offers cancellation and silently retains old inbox rows after many polling failures.

## Current behavior

- `web/static/app.js` fetches the transcript and inbox separately. Every known pending item is labelled Queued or Steering regardless of execution outcome or read freshness.
- `internal/opencode/client.go` omits the published idle-message `outcome` (`succeeded`, `failed`, `interrupted`), so the UI cannot reliably explain why execution stopped.
- Inbox polling failures preserve the last list but usually hide the failure. Transcript polls clear the shared status line, including unrelated inbox errors.
- An existing pending item can only be cancelled in the composer. The server already supports capability-gated delivery updates; changing the existing item's delivery to steer wakes execution under the published V2 contract.
- Existing uncommitted work affects the chat snapshot, compaction restoration, and UI. Preserve those changes and build on the current working tree.

## Target behavior

- Distinguish active work, idle execution, failed/interrupted execution, and unavailable execution state. Never call an unknown active-state read Idle.
- Show genuine queued work as waiting while execution is active, idle/pending between runs, and paused after failure or interruption. Do not mislabel the normal gap between successful runs as a failure requiring recovery.
- Retain last-known inbox rows on read failure but mark them stale and explain that delivery status is unconfirmed. Successful refresh removes the warning.
- Selecting a pending user message offers Resume when idle and Send now while busy, capability-gated and disabled when authoritative state is unavailable. Recovery updates that item's delivery to steer; it does not submit a new prompt, change text, or lose attachments/skills.
- Refresh after recovery/cancellation and accurately describe races: no longer pending is not proof of cancellation or successful model consumption.
- No polling path or failed execution automatically resumes pending work. Stop remains respected until a new explicit user action.

## Scope

The narrow OpenCode message adapter; server-normalized transcript state; the pending-message/composer UI and freshness tracking; adapter/server/render and executable client regressions; README documentation.

## Non-goals

Automatic retries or queue draining after errors; changing OpenCode's scheduler; editable pending text; a second queue store; automatic completion of workflow tasks; modifying the reported ARC change; new chat event streams or a client framework.

## Design decisions

1. User selected Option A: truthful state plus explicit recovery, not automatic recovery.
2. Use the existing delivery-update endpoint to steer the same `msg_` item. Capability detection stays method/path based; unsupported services display unavailable recovery rather than silently resending.
3. Inbox freshness has its own persistent display, independent of transient composer status. Both transcript and inbox responses are session/request guarded so late results cannot overwrite a newer session or refresh.
4. Failed reads are uncertainty, not an empty inbox, successful delivery, or idle execution. Recovery must wait for a fresh-enough authoritative snapshot/inbox.
5. Keep normal queue ordering and compaction restoration unchanged; tests must cover them alongside failure and Stop behavior.

## Acceptance criteria

1. Idle outcome survives decoding and is exposed on latest snapshots, without importing historical outcomes into history pages or exposing raw provider errors.
2. A pending follow-up shows waiting during active execution and an explicit paused state after failure/Stop. Unknown execution state is visibly unknown and blocks recovery.
3. A failed inbox poll retains known messages with an enduring stale warning, including when transcript polls succeed. Reconnect refresh clears it.
4. Resume/Send now performs one delivery update on the selected existing item, never a new prompt POST. Repeated clicks, disappearance races, and session switches do not create duplicates or corrupt UI state.
5. Queue consumption reconciles against transcript IDs and the authoritative inbox, and cancellation does not claim an unsupported outcome.
6. Adapter/server tests and executable client tests cover successful completion, failure, interruption, compaction, stale/overlapping reads, capability absence, and recovery races. `go vet ./...`, `go test ./...`, client checks, and lessmess validation pass.

## Tasks

1. QM-00 — Expose reliable chat execution outcomes.
2. QM-01 — Add explicit pending-message recovery and freshness states.
3. QM-02 — Verify queue recovery regressions and document the contract.

## Verification and handover — 2026-10-03

Implementation is ready at Test. Full Go tests/vet, the temporary build, 20 executable client regressions, focused race checks, JavaScript syntax, formatting, and diff checks pass. Workflow validation reports no violations, only doc-gardener warnings. A broader race run additionally exposes the unchanged WaitDone fixture counter race, outside this scope. Tests use controlled service/DOM fixtures; no live browser or exact ARC-thread diagnosis is claimed. The live server and original session are untouched; rebuild/restart is required for UI acceptance. Only the user accepts Done and closes the change.
