
# OCI-07: Dogfood and acceptance verification

Status: see [../ledger.md](../ledger.md).

## Objective

Prove the integration end-to-end against this repository and confirm every plan acceptance criterion.

## Dependencies

OCI-05, OCI-06.

## Scope

In scope: full manual + scripted pass over acceptance criteria 1–6 from [../plan.md](../plan.md); multiple sessions per change; terminal interactivity (prompt → agent edits → board updates live); reconnect behavior; password-leak check (browser-facing responses contain no Basic credentials); final `go vet`, `go test ./...`, `tasktracker validate`.
Out of scope: new features.

## Implementation steps

1. Start server; create two sessions on one change; verify mapping persists across a server restart.
2. Open the terminal; send a prompt that edits a ledger (e.g., add a task note); watch the board update live via SSE.
3. Run the new-change-session flow; confirm scaffold compliance via `tasktracker validate` and the primed session's first agent turn.
4. Inspect browser-facing proxy responses/headers for credential leakage.
5. Record per-criterion evidence in this task's notes.

## Verification

- Each acceptance criterion has recorded evidence; full suite green; validate OK.

## Completion criteria

- Plan acceptance criteria 1–6 all pass with evidence.

## Files affected

- None beyond evidence notes (cleanup of any throwaway changes/sessions created).

## Notes

- Per-criterion evidence (2026-09-12):
  1. Multi-session + persistence — ✅ two sessions created on 2026-09-12-0 via endpoint; after a server restart both listed with live service enrichment.
  2. Terminal + live board — ✅ WS bridge streamed ~393KB of real TUI output (OCI-04 probe); in this run the primed agent wrote `plan.md` (24→171 lines) and authored 3 task files; the board rendered all 3 cards (JSON + HTML verified) — the same canonical files drive both.
  3. New-change-session flow — ✅ `POST /changes/session` scaffolded 2026-09-12-1 (spec-compliant per `validate` OK), created + primed a session, mapped it, and the agent executed the primed instructions (plan refinement + task authoring with correct IDs/frontmatter).
  4. Password hygiene — ✅ password string occurs 0 times across index, board, sessions list, app.js, and validate responses.
  5. xterm vendored/pinned — ✅ VENDOR.md entries with SHA-256; all three assets serve 200 from the single embedded binary.
  6. Tests + permissions — ✅ `go test ./...` green; `opencode.json` verified live in OCI-06 (shell+edit turn, no prompt, no `--auto`).
- Cleanup: all dogfood sessions deleted from the service (204s), mapping unlinked, throwaway change 2026-09-12-1 removed (dir + root row); `validate` OK; `.tasktracker/sessions.json` back to `{}`.
- Residual: interactive terminal typing/rendering polish is best verified by a human in the browser; the mechanism is proven (spawn→PTY→WS bytes both ways, `cat` echo test + live TUI stream).

