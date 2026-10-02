# 2026-10-02-s4p21: Fix stale compaction re-prime detection

- Change ID: 2026-10-02-s4p21
- Created: 2026-10-02
- Branch: —
- Status: tracked in the tool-owned JSON state (.lessmess/workflow/)
- Supersedes: 2026-09-30-f5rdx (same diagnosis and target rule; this change
  carries the implementation)

## Objective and context

The compaction re-prime (introduced 2026-09-27, commit `8efb17e`) wraps the
next prompt to a bound session in a "Context restoration: the conversation
above was compacted…" preamble plus a full state snapshot
(`maybeReprime`, `internal/server/reprime.go`). When detection fires at the
wrong moment, the user's message is rewritten to
preamble + ledger snapshot + `-----` + their text — one stored user message
whose body fuses compaction content with the user's words. Users see their
message "appended to the last compaction message", seemingly at random.

It is not random: the trigger correlates with server restarts and with
mid-session (auto-/queued) compactions completing, not with anything the
user does. This change replaces the memory-based detector with a stateless
transcript rule so the wrap fires exactly when a compaction actually needs
compensating.

## Current behavior

`compactionWatch` (`internal/server/reprime.go`) tracks three memory-only
maps: `pending` (lessmess queued a compaction), `lastSeen` (newest completed
compaction observed in a snapshot walk, `internal/server/chat.go:266-282`),
`primed` (newest compensated compaction). `needsReprime` = pending OR
lastSeen ≠ primed. Three defects:

1. **Restart amnesia** — after a restart `primed` is empty; opening any bound
   session whose recent ~50 messages contain a completed compaction arms
   `lastSeen`, so the next prompt re-primes even though that compaction was
   compensated by a previous process.
2. **Failed compactions arm the wrapper** — `sessionCompact`
   (`internal/server/sessionlifecycle.go:444-446`) calls `markPending`
   unconditionally, even when the service reports the compaction `failed`;
   the context was never compacted yet the next prompt claims it was.
3. **Double-fire** — a prompt sent while a compaction is still in flight
   consumes `pending` before `lastSeen` is set, so `primed` stays empty; when
   the snapshot later observes the completed compaction, the next prompt
   re-primes a second time for the same compaction.

Defect 1 explains the spurious wrap after restarting `lessmess serve`;
defect 3 is the mid-session case (auto-compaction completes while working,
the next ordinary prompt fuses with it).

## Target behavior

Re-prime detection becomes stateless and transcript-based: a bound session
needs a re-prime iff its newest **completed** compaction message is strictly
newer than its newest **user** message. Evaluated at prompt time from one
small descending `ListMessagesPage` fetch:

- scanning the desc page, if a user message is found before any completed
  compaction → the conversation already continued past the compaction → no
  re-prime;
- if a completed compaction is found first → re-prime;
- equal timestamps count as continued (no re-prime) to avoid spurious fires;
- page exhausted with a completed compaction but no user message → re-prime
  (conservative: compacted and no user turn since).

Properties: restart-proof (no memory, nothing persisted), failure-proof
(`failed` status never counts), self-healing across fork/revert, immune to
the double-fire race, and correct for auto-compaction mid-turn (the service
inserts the compaction after the triggering user message, so the *next*
prompt compensates).

## Scope

- `internal/server/reprime.go` — replace `compactionWatch` with a transcript
  evaluator; `maybeReprime` gains a request context for the transcript read
  and keeps its fail-open behavior (unbound sessions never wrapped;
  ledger-read failures send the plain text).
- `internal/server/server.go` — drop the `compacts` field and its wiring
  (lines 43, 60).
- `internal/server/sessionlifecycle.go` — drop the `markPending` call.
- `internal/server/chat.go` — drop the compaction-observation block from the
  snapshot walk (lines 266-282).
- `internal/server/reprime_test.go` — rewrite the fixtures for the new rule.

## Non-goals

- No change to the re-prime preamble content or the ledger snapshot it
  carries.
- No collapsing of the preamble in transcript rendering (possible follow-up).
- No idempotency work on the plain prompt path or changes to the client busy
  flag (separate hardening concerns; they do not cause this bug).
- No persistence layer and no changes to `internal/opencode` beyond reuse of
  the existing paginated message list.
- No board/UI changes and no workflow-state edits (removal of the superseded
  2026-09-30-f5rdx change is a user board action, not part of this change).

## Design decisions

- **Stateless transcript rule over persisted markers**: the rule needs only
  data the transcript already holds, so a `.lessmess/reprime.json` marker
  would add state that can go stale for no benefit; the rule also fixes all
  three defects with one mechanism and deletes code.
- **Strictly-newer comparison** on `Time.Created` (millisecond floats): ties
  are treated as "already continued" — a missed re-prime in a tie is safer
  than a spurious one.
- **Fail-open preserved**: any fetch error or unreadable ledger sends the
  user's text unwrapped; a user prompt is never blocked by this path.

## Acceptance criteria

- Reopen a bound, previously compacted session after a server restart and
  prompt it: no re-prime preamble (compaction already continued past).
- A completed compaction as the newest event in a bound session: the next
  prompt carries the re-prime, and the prompt after it does not.
- A failed manual compaction: the next prompt is never wrapped.
- Prompt racing an in-flight compaction: exactly one re-prime, at the first
  prompt after the compaction completes.
- Unbound sessions and unreadable-ledger failures: text sent unwrapped.
- `go vet ./...`, `go test ./...`, and `lessmess validate` clean.

## Tasks

1. (task breakdown is maintained by the tool; see the board)
