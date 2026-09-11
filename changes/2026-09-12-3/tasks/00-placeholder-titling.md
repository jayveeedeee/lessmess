---
id: GEN-00
title: Placeholder titling flow and button-only UI
---

# GEN-00: Placeholder titling flow and button-only UI

Status: see [../ledger.md](../ledger.md).

## Objective

Make change-session creation input-free: scaffold with a random placeholder title, prime the agent to assign the real title/prefix/session name, and reduce the UI to a single button.

## Dependencies

None.

## Scope

In scope: `internal/server/changesession.go` (placeholder + prompt + optional title), `web/templates/index.html` (button-only), tests.
Out of scope: agent behavior beyond the prime prompt.

## Implementation steps

1. Add a small `untitled-<adjective>-<noun>` generator (time-seeded).
2. `createChangeWithSession`: accept empty title → placeholder; empty prefix → `—`; rewrite `primePrompt` with titling/prefix/session-rename instructions (agent uses `opencode2 api post /api/session/{id}/rename`).
3. Index: remove the two inputs; keep one button posting an empty body.
4. Update flow tests: placeholder path (assert prompt contains the titling instructions), explicit-title path, HX-Redirect, failure modes; drop the title-required test.

## Verification

- Tests pass; served index has one button and no text inputs; `tasktracker validate` clean.

## Completion criteria

- Plan acceptance criteria 1–3 met.

## Files affected

- `internal/server/changesession.go`, `internal/server/changesession_test.go`, `web/templates/index.html`

## Notes

- Verification (2026-09-12): served index has zero `<input>` elements and one orange button (confirmed by curl and a fresh headless screenshot after `kill -9` cleared a stale port-9090 process that had been serving pre-edit output — the recurring zombie trap); `TestChangeSessionPlaceholderTitle` asserts placeholder scaffold (`untitled-*` title, `—` prefix) and prompt contents (titling, prefix-registration, session-rename instructions with the real session ID); explicit-title path still covered by `TestChangeSessionFlow`; `tasktracker validate` OK.

