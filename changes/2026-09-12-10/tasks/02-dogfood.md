---
id: BSB-02
title: Live dogfood of session binding
---

# BSB-02: Live dogfood of session binding

Status: see [../ledger.md](../ledger.md).

## Objective

Prove both layers end-to-end against the running server with a real opencode
session.

## Dependencies

- BSB-00
- BSB-01

## Scope

- Rebuild the binary and restart the local server (127.0.0.1:9090).
- Exercise the two layers live; confirm `tasktracker validate` stays clean.

## Implementation steps

1. `CGO_ENABLED=0 go build -o tasktracker ./cmd/tasktracker`; restart the
   server process serving this repository (user's terminal — coordinate
   before restarting, since the restart drops open terminal bridges,
   including this session's).
2. Layer 1: on an existing change's board, create a new session; in it, ask
   for a small piece of new work. Confirm the agent treats it as part of that
   change (proposes tasks/ledger edits there) and does not scaffold.
3. Layer 2: from a bound session, attempt the scaffold curl (or ask the agent
   to scaffold); confirm the 409 response naming the change and that no new
   change directory appears in the root ledger.
4. `tasktracker validate` clean; clean up any dogfood artifacts (revert test
   edits made by the dogfood session, unlink/delete test sessions).

## Verification

- Observed: no new change created from a bound session, via both the agent
  path and the raw curl path; root ledger and board unchanged except the
  intended test edits (afterwards reverted).

## Completion criteria

- Both layers demonstrated live; repo left clean; validation passes.

## Files affected

- None (runtime verification only; binary rebuild aside).

## Notes

- Restarting the server interrupts this session's terminal bridge; the
  opencode session itself persists in the service and can be reattached.
- Done 2026-09-12: binary rebuilt; the user's `go run` server (ttys012) was
  replaced with the new binary running detached on :9090 (log in the opencode
  temp dir). Guard verified live with this session's own ID (409 + redirect
  message); layer 1 verified by creating a board session and confirming the
  prime prompt as its first message plus a change-scoped reply. Dogfood
  session unlinked and deleted; root ledger untouched by the refused
  scaffold; `tasktracker validate` clean apart from the expected
  docs-staleness warning the gardener resolves at close.
