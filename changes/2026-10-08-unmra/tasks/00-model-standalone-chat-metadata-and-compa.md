# CHAT-00: Model standalone chat metadata and compatible listing

## Why

The current unassigned bucket mixes user conversations and automated helpers,
and its listing only exposes creation dates. A first-class Chats list needs
explicit eligibility, activity ordering, and safe title ownership without
losing existing personal session mappings.

## What

- Extend session metadata with optional kind, activity, and title/naming state
  as needed; keep old `.lessmess/sessions.json` files readable.
- Record creation context for general chats, planning/settings discussions,
  explorer chats, helper sessions, and forks. Classify legacy entries
  conservatively so existing conversations remain discoverable.
- Expose a project-scoped Chats feed, excluding bound change/task sessions and
  identifiable automated helpers/transient descendants. Preserve existing
  discussion API compatibility.
- Enrich activity and titles from OpenCode using bounded service reads, with
  persisted fallback values when the service is unavailable. Sort recently
  active first with deterministic tie-breaking.
- Supply atomic mapping operations for title ownership and promotion-related
  checks. Failed persistence must not leave an incorrect in-memory ownership
  or title that differs from disk.

## Files affected

- `internal/server/mapping.go` and `internal/server/mapping_test.go`
- `internal/server/server.go`
- `internal/server/chat.go` and `internal/server/changesession.go`
- `internal/server/explorer.go`, `internal/server/settingschange.go`, and
  `internal/server/gitcommit.go` (creation metadata)
- `internal/server/sessionlifecycle.go` and its tests (fork/rename handling)
- New narrowly scoped chat-listing code/tests under `internal/server/` if useful

## Verification

- Cover missing optional fields, mixed legacy entries, duplicate prevention,
  change-bound exclusion, helper exclusion, activity ordering, and deterministic
  ties with fixture mappings and fake OpenCode sessions.
- Verify service failures still return discoverable chats with safe stored
  titles/timestamps, without silently rewriting meaningful existing titles.
- Exercise failed atomic saves and concurrent ownership/title operations.
- Run focused server tests and workflow validation; record evidence before
  requesting a transition to Test.
