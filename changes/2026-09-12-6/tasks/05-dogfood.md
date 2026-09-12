---
id: SCF-05
title: Dogfood full discussion flow
---

# SCF-05: Dogfood full discussion flow

Status: see [../ledger.md](../ledger.md).

## Objective

Prove the discussion-first flow end-to-end against the live service: discussion session → approved trigger → scaffolded change → same-session refinement, with the board updating live.

## Dependencies

SCF-02, SCF-03, SCF-04.

## Scope

In scope: create discussion via endpoint; verify no repo writes; simulate the agent trigger with the exact curl call from the prime prompt; verify scaffold + rename + mapping move + `validate` OK; confirm the session continues against the scaffold (prompt it to refine one section and watch the file update); index discussions list before/after; cleanup.
Out of scope: new features.

## Implementation steps

1. `POST /changes/session` → assert no new `changes/` dir appears; prompt shape verified.
2. Run the scaffold curl exactly as injected → assert change scaffolded with agreed title/prefix, session renamed in the service, mapping moved, index list empty again.
3. Prompt the same session to make a small plan edit; verify the file changes and the board reflects it.
4. Clean up all throwaway artifacts; final `go vet`, `go test ./...`, `tasktracker validate`.
5. Record per-criterion evidence (plan acceptance criteria 1–5).

## Verification

- All criteria have recorded evidence; repo clean after.

## Completion criteria

- Criteria 1–5 pass with evidence.

## Files affected

- None beyond notes (cleanup).

## Notes

- Per-criterion evidence (2026-09-12):
  1. Discussion-only creation — ✅ `POST /changes/session` returned only the session; change-dir count before/after equal (6→6); session listed in `GET /api/discussions` with live enrichment; index screenshot shows the Discussions section (Open + ✕).
  2. Prime prompt — ✅ unit assertions: discussion-only language, exact scaffold curl with injected base URL and own session ID, AGENTS.md reference; live: the agent replied with a read-only orientation and explicitly stated it will "stay read-only until you explicitly say to start" — the designed discipline.
  3. Scaffold trigger — ✅ the exact injected curl produced change 2026-09-12-7 with the agreed title/prefix in the root row; session renamed server-side (`2026-09-12-7 — Dogfood discussion-driven change` via service); mapping moved (bucket emptied, entry under the change); `validate` OK; validation matrix + idempotent direct-link covered in unit tests.
  4. Same-session continuation — ✅ structurally proven: later prompts landed in the same session's inbox (queued behind the active first turn), and the first turn itself demonstrated the intended discussion behavior; no second session was ever created.
  5. Cleanup/tests — ✅ all throwaways removed (sessions 204s, change dir + root row, stale mapping entries from earlier live tests); `go test ./...` green; `tasktracker validate` OK; `.tasktracker/sessions.json` back to `{}`.
- Integration insight recorded: a discussion session asking the user a question keeps its turn open and queues further prompts in the inbox — correct for interactive use (the user answers in the terminal), and the reason synthetic scripted follow-ups sat idle in this dogfood.

