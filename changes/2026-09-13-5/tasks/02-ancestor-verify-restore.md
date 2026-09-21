
# LRN-02: Snapshot/verify/restore for ancestor targets

Status: see [../ledger.md](../ledger.md).

## Objective

Ancestor dirs get exactly the same confinement cage as primary dirs:
snapshot before the session, `verifyGardener` after, restore on failure
— while the prompt still distinguishes the two roles.

## Dependencies

- LRN-00 (ancestor list on the job).
- LRN-01 (prompt consumes the split).

## Scope

- `gardenerRunner.RunDocsJob` in `internal/server/docssession.go`.
- No changes to the verification rules themselves; only the target set
  and prompt split.

## Implementation steps

1. Resolve update targets from `job.Dirs` and review targets from
   `job.Ancestors` against the walked tree (skip dirs no longer covered,
   as today).
2. Snapshot and verify over the union; pass both lists to
   `gardenerPrompt` for section rendering; restore covers the union
   (including deleting a created, marker-less `AGENTS.md` in an
   ancestor).
3. `verifyGardener` runs per dir over the union — no rule changes.
4. Settle pass (`RefreshSkeletons`) unchanged: already whole-tree.
5. Log lines name both sets (dirs + ancestors).

## Verification

- Fake-session runner tests: a confinement violation inside an ancestor
  (out-of-marker edit) restores the snapshot and fails the job; a
  created ancestor `AGENTS.md` without markers is a violation; a
  well-behaved pass over update + review targets succeeds and stale
  flags clear for both sets on success.
- Existing runner tests stay green.

## Completion criteria

Ancestors are protected and verified identically to primary dirs, with
tests proving restore-on-violation in the ancestor case.

## Files affected

- `internal/server/docssession.go`
- `internal/server/docssession_test.go`

## Notes

- The single sanctioned deletion (created file during rollback) now
  extends to ancestors — same rule, wider set.
