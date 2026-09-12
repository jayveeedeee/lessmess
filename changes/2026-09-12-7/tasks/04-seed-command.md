---
id: DOC-04
title: tasktracker docs seed command
---

# DOC-04: tasktracker docs seed command

Status: see [../ledger.md](../ledger.md).

## Objective

Add `tasktracker docs seed [--dry-run] [--budget N] [--dir .]`: the one-time initial
run-through for an existing codebase — bottom-up generation of `STRUCTURE.md`
(deterministic) plus LLM-written purpose blurbs and first-pass `AGENTS.md`
learnings, incremental and resumable.

## Dependencies

- DOC-02 (structure generator)

## Scope

- Bottom-up seed over covered dirs: render `STRUCTURE.md` via DOC-02, then ask the
  LLM for purpose blurbs of placeholder entries and an initial `AGENTS.md` for the
  dir (area purpose, key files, conventions discovered from code).
- LLM access via the existing `internal/opencode` client directly from the CLI,
  behind a `Summarizer` interface so tests use a stub (no live service in unit
  tests).
- Resume cursor in `.tasktracker/docs-seed.json` (completed dirs, budget consumed);
  interrupted runs resume where they stopped.
- `--dry-run`: print the planned dir order and what would be written, no writes.
- `--budget N`: cap on LLM sessions/tokens for one run; exhaustion stops cleanly
  with the cursor saved.
- Idempotent: fully seeded tree re-seeds to byte-identical files.

## Implementation steps

1. `internal/docs/summarize.go`: `Summarizer` interface + opencode-backed
   implementation (session per folder batch, tight prompt with dir inventory +
   child summaries + file excerpts).
2. `internal/docs/seed.go`: planner (bottom-up order, work remaining vs. cursor),
   executor (merge LLM output into docs via DOC-00 merge), cursor persistence
   (atomic).
3. Wire `docs seed` into `cmd/tasktracker/main.go`.
4. Tests: stub summarizer over fixture trees; dry-run output; resume after
   simulated interruption; budget exhaustion; idempotence.

## Verification

- `go test ./internal/docs` passes (stub-based; no live service).
- Manual dry run on this repo prints a sane plan (recorded in DOC-08).

## Completion criteria

- Seed completes a fixture repo end-to-end with a stub and is byte-identical on
  rerun.
- Cursor + budget behavior verified by tests.

## Files affected

- `internal/docs/summarize.go`, `seed.go` (new)
- `internal/docs/seed_test.go` (new)
- `cmd/tasktracker/main.go` (wiring)
- `.tasktracker/docs-seed.json` (runtime, gitignored)

## Notes

- Keep per-folder prompts small (inventory + child summaries + limited excerpts);
  large repos rely on bottom-up composition, not big contexts.
- 2026-09-12 — Implemented in `internal/docs/seed.go` + `summarize.go`, wired as
  `tasktracker docs seed [--dry-run] [--budget N] [--dir .]`. Decisions:
  - `Summarizer` writes files directly (agent-native; the service API has no
    transcript reading, same pattern as the commit flow). `OpenCodeSummarizer`:
    one session per dir (create → prompt → WaitDone 10 min → delete).
  - Confinement: snapshot → pass → verify (STRUCTURE.md parses with meta hash
    intact; AGENTS.md parses) → restore on violation. Restore may remove a
    newly created AGENTS.md — a documented, narrow exception to the
    "never deletes" posture (rollback of an unreviewed creation).
  - Three phases — skeletons, summarize (with purpose read-back into the tree),
    settle rollups — after tests showed a single interleaved pass embeds stale
    placeholder purposes in parents (child purposes land on disk after the
    parent's Build). The settle pass makes run-1 output final and run-2
    byte-identical.
  - Cursor (`.tasktracker/docs-seed.json`) tracks summarized dirs only;
    skeletons are rewritten every run (cheap, idempotent, carries annotations).
  - `--budget` counts summarizer sessions per run (0 = unlimited); `--dry-run`
    writes nothing (not even the cursor). Offline (no service) = skeletons only,
    dirs stay pending. `SeedOptions.Date` exists for deterministic tests.
- Verification evidence: `go test ./internal/docs -count=1` — happy path
  (post-order calls, placeholders filled, AGENTS.md written, uncovered dirs
  untouched), idempotent rerun (byte-identical, no repeat summarizer calls),
  budget=1 then resume, dry-run (no files, no cursor, no calls), bad-agent
  rollback + sibling continuation + cursor correctness, offline mode, prompt
  content. Manual: `docs seed --dry-run` on this repo prints 14 dirs in correct
  post-order with no writes. `gofmt`/`go vet` clean; full suite green.
