---
id: LRN-01
title: Two-section gardener prompt with removal accounting
---

# LRN-01: Two-section gardener prompt with removal accounting

Status: see [../ledger.md](../ledger.md).

## Objective

The gardener prompt distinguishes update dirs from review-and-fix
ancestor dirs, licenses deleting learnings about removed surface in the
latter, and requires an accounting of every removal/edit in the reply.

## Dependencies

- LRN-00 (job carries the ancestor list).

## Scope

- `gardenerPrompt` in `internal/server/docssession.go` renders two
  sections from `job.Dirs` + `job.Ancestors` (resolved to `*docs.Dir`).
- Manual branch stays single-section (lint context arrives in LRN-04).
- Prompt table tests.

## Implementation steps

1. Thread the split into prompt construction: update targets from
   `job.Dirs`, review targets from `job.Ancestors` (both already
   validated as covered by LRN-00/02 work).
2. Review-and-fix section wording:
   - Read the change record first (plan/ledger/tasks) as today.
   - Fix or delete any learning referencing surface this change removed
     or materially changed; deletion is expected when the learning was
     purely about removed surface.
   - Add a learning only if the change taught something at that
     directory's level; new learnings carry the same `(changeID)` prefix.
   - Keep the shared hard rules (one marker pair, bytes outside markers
     preserved, no git, no other files).
3. Reply contract: one line per directory (both sections) **plus** one
   line per removed/edited learning naming it and why.
4. Update-section wording unchanged from today.

## Verification

- Table tests covering: both sections rendered with the right dirs;
  deletion license present only in the review section; accounting
  requirement present; manual jobs render exactly one section and no
  ancestor text; change-ID prefix rule intact; no `|`/marker leakage.

## Completion criteria

Prompt construction is pure and table-tested; wording matches the plan;
manual behavior is byte-comparable to today (modulo LRN-04).

## Files affected

- `internal/server/docssession.go`
- `internal/server/docssession_test.go`

## Notes

- Keep the prompt builder a pure function (current discipline) so tests
  stay table-driven.
