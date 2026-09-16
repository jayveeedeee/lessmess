---
id: LRN-00
title: Curated-learnings convention in workflow text, prompts, and README
---

# LRN-00: Curated-learnings convention in workflow text, prompts, and README

Status: see [../ledger.md](../ledger.md).

## Objective

Redefine the AGENTS.md learnings convention from append-with-change-ID to
curated, current-state, bounded sections — everywhere the convention is stated
or enforced: the workflow text, the gardener and seed prompts, README, and the
tests pinning their wording.

## Dependencies

None; this is the first task and LRN-01 builds on it.

## Scope

- Workflow text: the "machine-maintained" sentence (and adjacent learnings
  wording) in root `AGENTS.md` above the markers, mirrored byte-for-byte into
  `internal/docs/assets/workflow_agents.md`.
- `gardenerPrompt` (`internal/server/docssession.go`): the manual branch, the
  change branch, and the REVIEW-AND-FIX section.
- Seed prompt step 3 (`internal/docs/summarize.go`).
- `README.md` docs section (the bullets describing what the gardener writes and
  cites).
- Tests pinning old wording (`internal/server/docssession_test.go`
  `TestGardenerManualPrompt`, `TestGardenerPromptReviewSection`; any others the
  run surfaces).

## Implementation steps

1. Reword the workflow-text sentence to: the auto section is machine-maintained
   — current-state learnings only, no provenance prefixes, at most 15 entries,
   consolidated in place when a change supersedes existing content.
2. Regenerate `internal/docs/assets/workflow_agents.md` from the edited root
   `AGENTS.md` (`awk '/^<!-- tasktracker:begin/{exit} {print}' AGENTS.md >
   internal/docs/assets/workflow_agents.md`) so the drift test passes.
3. In `gardenerPrompt`:
   - Manual branch: replace the "(manual)" prefix instruction with
     consolidate-in-place guidance under the 15 cap.
   - Change branch: replace "add 1–5 brief learnings … prefix each with the
     change ID" with: add only durable new knowledge; reword, merge, or delete
     existing entries the change superseded; keep every entry phrased as how the
     code works now; keep the section at ≤15 entries.
   - REVIEW-AND-FIX section: keep the fix/delete license and the
     account-for-removals reply requirement; align wording with the new
     convention (no new-ID prefixing, respect the cap).
4. Seed prompt step 3: keep "2–5 brief learnings" but require current-state
   phrasing and note the section should stay minimal.
5. Update `README.md`'s docs bullets to describe curated, bounded, unprefixed
   learnings and blame-based attribution.
6. Update the pinning tests to the new wording (drop `"(manual)"` and
   `"(2026-09-10-0)"` assertions; pin the new consolidation/cap phrases
   instead).

## Verification

- `go vet ./... && go test ./...` green from the repo root, including the
  drift test in `internal/docs/init_test.go`.
- `grep` over `internal/server/docssession.go` and `internal/docs/summarize.go`
  shows no remaining "prefix" instructions tied to change IDs, `(seed)`, or
  `(manual)`.

## Completion criteria

All verification passes; the prompts, workflow text, asset, README, and tests
agree on one convention.

## Files affected

- `AGENTS.md` (workflow text only, above the markers)
- `internal/docs/assets/workflow_agents.md`
- `internal/server/docssession.go`
- `internal/docs/summarize.go`
- `README.md`
- `internal/server/docssession_test.go`

## Notes

- The workflow text is embedded in the binary; until rebuild, running servers
  keep old behavior — irrelevant here, since prompts are read at spawn time
  from the rebuilt binary after restart.
- Learnings INSIDE the root markers are LRN-01's business, not this task's.
- Verification evidence (2026-09-16): `go vet ./...` clean; `go test ./...`
  green (`internal/server` 11.9s, `internal/docs` ok), including
  `TestWorkflowAssetDrift`; grep confirms no "prefix each", "change ID in
  parentheses", "(manual)", "(seed)", or "cite their source" wording remains in
  `internal/`+`cmd/` Go sources or README. Server binary rebuilt and restarted
  (the stale pre-STS process 404'd `/changes/{id}/status`).
