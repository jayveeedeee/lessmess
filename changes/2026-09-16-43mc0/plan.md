# 2026-09-16-43mc0: Curated doc learnings

- Change ID: 2026-09-16-43mc0
- Created: 2026-09-16
- Branch: —
- Status: see [ledger.md](ledger.md)

## Objective and context

The marker-guarded auto sections of the per-folder `AGENTS.md` files have drifted
into change logs: the doc gardener appends 1–5 learnings per close-out, each
prefixed with its change ID, and nothing consolidates. The result is ~244
entries across 14 files (root: 54, `internal/server`: 64), many narrating what a
change did rather than stating how the code works now, with near-duplicates and
unbounded growth.

The `changes/` ledger is the log of what changed; `AGENTS.md` learnings should be
durable, current-state knowledge for agents working in a folder. Change-ID
attribution is redundant anyway — the docs are committed, so git blame answers
"which change added this line."

This change (a) redefines the convention and the prompts that enforce it, and
(b) rewrites the existing sections to match.

## Current behavior

- `gardenerPrompt` (`internal/server/docssession.go`) instructs change jobs to
  "add 1–5 brief learnings … prefix each with the change ID", manual jobs to
  prefix with `(manual)`, and the seed prompt (`internal/docs/summarize.go`)
  writes unprefixed first-pass learnings.
- Removal happens only in the close-out REVIEW-AND-FIX pass for ancestor dirs,
  and only for learnings the change invalidated. No merging, no dedup, no cap.
- The root `AGENTS.md` workflow text (and its embedded twin,
  `internal/docs/assets/workflow_agents.md`, byte-pinned by a drift test) states
  learnings "cite their source change ID, `seed`, or `manual`".
- `README.md` (docs section, ~lines 306–316) documents the citing convention.
- Tests pin the old wording: `TestGardenerManualPrompt` asserts `"(manual)"`;
  `TestGardenerPromptReviewSection` asserts `"(2026-09-10-0)"`.

## Target behavior

- Learnings are current-state statements ("X is Y", "use Z when W"), never
  change narration. A learning that would read "(since change D, X does Y)"
  becomes "X does Y".
- No provenance prefixes of any kind: no change IDs, no `(seed)`, no `(manual)`.
  Attribution is git blame's job.
- Sections are bounded: at most 15 learnings per file. When a change would push
  a section past the cap, the gardener merges or prunes the weakest existing
  entries instead of appending.
- The gardener's default move is consolidation: reword, merge, or delete
  superseded entries in place; add only what is genuinely new and durable.
- The REVIEW-AND-FIX pass keeps its fix/delete license, under the same
  convention and cap.

## Scope

1. Convention and enforcement surfaces:
   - Root `AGENTS.md` workflow text (above the markers) and
     `internal/docs/assets/workflow_agents.md` — the "machine-maintained"
     sentence and any adjacent wording — updated together (drift test).
   - `gardenerPrompt`: manual, change, and review sections in
     `internal/server/docssession.go`.
   - Seed prompt in `internal/docs/summarize.go` (step 3).
   - `README.md` docs section.
   - Tests pinning old wording in `internal/server/docssession_test.go` (and any
     other phrasing pins surfaced by the test run).
2. One-time cleanup of every covered `AGENTS.md` auto section (all 14 files) to
   the new convention: reword to current-state, merge duplicates, delete
   superseded/obsolete entries, enforce the ≤15 cap, drop all prefixes.

## Non-goals

- No changes to `STRUCTURE.md` format, marker syntax, or the docs queue /
  refresh / seed mechanics.
- `StaleLearningRefs` lint semantics stay as-is (path lints only).
- No tooling enforcement of the 15-cap (a `ValidateDocs` over-cap warning is a
  possible follow-up, not done here).
- No changes to the workflow's status/ledger rules (only the learnings sentences
  of the workflow text are reworded).

## Design decisions

- Drop all provenance prefixes rather than demoting them to trailing tags: they
  encouraged changelog framing, and git blame already provides attribution.
- Cap = 15 per file, enforced by prompt wording only. One number, stated in the
  gardener and seed prompts and the workflow text.
- The cleanup is performed directly by the change session (reviewable diff),
  not via gardener sessions — the user wants to review the curation.
- Sequencing: convention first (LRN-00), cleanup second (LRN-01), so the
  rewritten sections and the text describing them land consistently.

## Acceptance criteria

1. `go vet ./... && go test ./...` green, including the drift test that pins
   root `AGENTS.md` workflow text against `internal/docs/assets/workflow_agents.md`.
2. Rebuilt `./lessmess validate` reports no docs errors; the stale-learning
   lint is clean.
3. Every covered `AGENTS.md` auto section: no `(change-id)`, `(seed)`, or
   `(manual)` prefixes; entries read as current-state facts; no near-duplicate
   pairs; ≤15 entries per file.
4. Gardener prompts (manual, change, review) instruct consolidation under the
   cap and carry no provenance-prefix instruction; `TestGardenerManualPrompt`
   and `TestGardenerPromptReviewSection` updated to pin the new wording.
5. `README.md` describes the curated, bounded convention.
6. The workflow-text sentence describing the auto section states the new
   convention identically in root `AGENTS.md` and the embedded asset.

## Tasks

1. [LRN-00](tasks/00-convention.md) — Curated-learnings convention in workflow text, prompts, and README.
2. [LRN-01](tasks/01-cleanup.md) — One-time curation of all AGENTS.md auto sections.
