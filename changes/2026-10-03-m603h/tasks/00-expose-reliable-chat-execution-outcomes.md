# QM-00: Expose reliable chat execution outcomes

## Why

The transcript drops idle execution outcomes and treats failed active-state reads as idle. Pending-message recovery needs a reliable distinction between running, stopped, failed, and unknown state.

## What

- Decode the published idle outcome without widening provider-error exposure.
- Add latest-snapshot outcome and active-state availability metadata. History pages must not change current execution state.
- Preserve current concurrent snapshot reads, last-good fallback, and compaction restoration work.

## Files affected

- `internal/opencode/client.go`, `internal/opencode/client_test.go`
- `internal/server/chat.go`, `internal/server/chat_test.go`
- `web/templates/partials.html`

## Verification

Adapter outcome decoding; latest versus history snapshots; failed active-state and last-good reads; generic assistant errors; existing chat and compaction regressions. Finish at Test with evidence.

### Evidence — 2026-10-03

New regressions reproduced the missing outcome/availability metadata before implementation. Adapter decoding now normalizes future outcomes to unknown. Latest snapshots preserve success/failure/interruption, suppress prior-turn failures after a new user input, and mark cached or unreadable active state unknown. History pages do not publish current outcome metadata. Focused and full Go tests pass; focused race checks pass.
