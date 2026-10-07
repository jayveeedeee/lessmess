# QM-01: Add explicit pending-message recovery and freshness states

## Why

Queued items currently offer only cancellation, and stale rows look authoritative. Users need explicit recovery after errors or Stop without duplicate prompts.

## What

- Render waiting, paused, failed/interrupted, and unknown pending states.
- Keep inbox freshness warnings separate from transient composer status; guard overlapping/late responses and session switches.
- Add capability-gated Resume/Send now to the pending-message action pill, updating the existing item to steer without resubmitting its text or attachments.
- Reconcile delivery and cancellation races conservatively. Never resume automatically, including after Stop or reconnect.

## Files affected

- `web/static/app.js`, `web/static/app.css`
- `web/templates/layout.html`
- `internal/server/render_test.go`, `internal/server/sessionlifecycle_test.go`
- `internal/server/sessionlifecycle.go`, `internal/server/reprime_test.go` (restore binding before resuming across compaction)
- Executable client regression tests under `web/`

## Verification

Executable UI tests for pending state/freshness, update-only recovery, duplicate clicks, stale-read and disappearance races, Stop, compaction, unavailable capabilities, and switching sessions; server endpoint contract tests. Finish at Test with evidence.

### Evidence — 2026-10-03

20 executable client tests exercise actual production functions against a DOM/fetch fixture. They cover waiting/idle/failure/Stop states; stale, expired and unavailable reads; ID reconciliation; late responses; double clicks; preflight consumption and 409 races; lost or accepted-but-unconfirmed responses; cancellation uncertainty; and read-only reconnect/capability refresh. Recovery sends only a delivery update for the existing item. Server tests cover published PATCH and beta steer routes, unsupported services, and disappearance races. Compaction tests verify restoration is admitted before the same queued item is awakened, without a new prompt. Focused Go and race checks, syntax, and formatting checks pass. No live browser or reported-session recovery was performed.
