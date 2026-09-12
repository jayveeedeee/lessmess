---
id: EXP-04
title: Dogfood and documentation
---

# EXP-04: Dogfood and documentation

Status: see [../ledger.md](../ledger.md).

## Objective

Prove the explorer end-to-end on this repository and document it in README.

## Dependencies

- EXP-01, EXP-02, EXP-03

## Scope

- Run the explorer against this repo: verify tree content against the seeded
  docs, expansion, chat session quality (ask a directory question, check the
  grounding), Discussions listing, and SSE refresh (trigger a gardener run or
  touch a doc).
- Fix/tune what the run reveals (styling, prompt wording).
- README: explorer section (what it shows, chat, live updates).
- Record evidence in Notes.

## Implementation steps

1. Serve with the new binary; walk the acceptance criteria in plan.md.
2. Tune as needed; re-verify.
3. README update; final `go vet ./... && go test ./...` and
   `tasktracker validate`.

## Verification

- All acceptance criteria in plan.md checked with evidence recorded.

## Completion criteria

- Explorer demonstrably works live on this repo; README accurate; suites green.

## Files affected

- `README.md`
- Any tuning touch-ups discovered during the run

## Notes

- 2026-09-12 — DOGFOOD PASSED on this repo (serve on :9097, real service):
  - Tree: `/explorer` renders the full covered tree with seeded purposes and
    blurbs; chat buttons present per node (smoke-verified via curl).
  - Chat: POST /explorer/chat {"dir":"internal/store"} → session
    `ses_f6a0d154cffe2k1wJa8un1Igcq` (root-scoped, mapped to Discussions,
    live). The primed agent produced a grounded, accurate orientation of
    internal/store citing files and line numbers (input 27k, output 1.5k
    tokens). Discovery: the service HAS a transcript endpoint
    (`GET /api/session/{id}/message`) — useful for future verification;
    recorded for potential client addition later.
  - SSE: curl on /events + creating a probe file in internal/model →
    `event: docs` received (debounced single event). Probe removed; the
    name-based tree hash returns to its prior value, so no lingering
    staleness.
  - The pre-existing staleness warnings (EXP implementation files) are the
    designed behavior and will clear when this change closes and the gardener
    runs.
- 2026-09-12 — README gains a "Project explorer" subsection under repo docs
  management (tree, chat, live updates). Final: `go vet ./...` clean, full
  `go test ./... -count=1` green (5 packages), `gofmt` clean,
  `tasktracker validate` OK apart from the by-design staleness warnings.