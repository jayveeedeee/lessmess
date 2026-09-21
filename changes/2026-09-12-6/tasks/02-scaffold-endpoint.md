
# SCF-02: Scaffold trigger endpoint

Status: see [../ledger.md](../ledger.md).

## Objective

Implement `POST /changes/scaffold`: the deterministic trigger the agent calls with `{title, prefix, session}` to build the change, rename the session, and move the mapping.

## Dependencies

SCF-00, SCF-01.

## Scope

In scope: endpoint + validation (title non-empty, no `|`; prefix `—` or 2–4 uppercase letters; session ID format), allocation via existing `CreateChange`, root-row title/prefix (CreateChange already writes both), session rename via client, mapping move. Errors: 400 bad input, 404 session not in `_unassigned` (still scaffolds? — no: move is best-effort after scaffold; see steps).
Out of scope: user-facing scaffold button (per user decision).

## Implementation steps

1. Parse + validate body.
2. `CreateChange(title, prefix, today)` (existing store op writes the root row with title/prefix).
3. Rename the opencode session to `<id> — <title>` via the client (warn-only on failure).
4. `moveToChange(session, id)` — if the session isn't in `_unassigned`, log and continue (idempotent for retries); also append directly when missing.
5. Respond `{change, session, title}`; tests with fake service + tempdir store: full effect, validation errors, idempotent move.

## Verification

- Endpoint tests pass; validate clean on the produced change.

## Completion criteria

- One call yields a compliant change, renamed session, and correct mapping.

## Files affected

- `internal/server/changesession.go` or new file (+ tests)

## Notes

- None yet.
