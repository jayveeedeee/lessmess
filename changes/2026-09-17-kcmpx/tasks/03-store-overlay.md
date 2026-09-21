
# WTP-03: Store overlay — worktree-aware reads, cross-tree status, validation

Status: see [../ledger.md](../ledger.md).

## Objective

Make the workflow layer worktree-aware: when a change has an active worktree,
its docs (plan, ledgers, tasks, containers, handoffs) resolve there;
`SetChangeStatus` writes across both trees; validation understands the split.

## Dependencies

- WTP-01 (state file + `gitops` probes). WTP-02 depends on the creation half of
  this task; the read-resolution half is what the board (WTP-06) and close
  (WTP-05) build on.

## Scope

- A resolver interface in `internal/store` (e.g. `ChangeRoot(id) (string, bool)`)
  wired by callers: server passes a gitops-backed resolver; `cmd/lessmess
  validate` wires the same; default resolver = main tree only (back-compat).
- All change-doc path construction in `store` goes through the resolver:
  `Change`, `PlanFile`, `LedgerFile`, `TaskFile`, `ContainerLedgerFile`,
  handoff helpers, tree walks for a single change. Root-ledger reads/writes and
  the changes index listing stay main-tree.
- `SetChangeStatus`: change-ledger write in the resolved root, root-ledger row
  in the main tree, back-to-back; second-write failure logged and returned.
- Validation (`internal/store/validate.go` + `lessmess validate`): a root-ledger
  row whose directory resolves via the resolver is valid; validation of a
  change's tree reads through the resolver. Resolver-absent behavior unchanged.
- Existence probing: a stale worktree entry (path gone, `git worktree list`
  missing it) falls back to the main tree with a warning log.

## Implementation steps

1. Define the resolver interface + a `MainTreeResolver` default; thread it
   through `store.Open` (optional setter to avoid breaking existing callers).
2. Replace direct `filepath.Join(st.Dir, "changes", id, …)` construction with
   resolver calls; audit every call site (grep `changes` joins in `store`).
3. Split `SetChangeStatus` writes; keep today's behavior when resolver says
   main tree (single-dir fast path, unchanged atomicity).
4. Validation updates + tests: worktree change (valid), stale worktree entry
   (falls back, warns), disabled feature (unchanged).
5. `cmd/lessmess`: wire the gitops-backed resolver into the validate command.

## Verification

- `go test ./internal/store/`: resolution matrix (worktree active / absent /
  stale / feature off), cross-tree status write, validation cases.
- `lessmess validate` run in this repo with a real scaffolded worktree (from
  WTP-02's smoke test) passes.

## Completion criteria

- The board's data path resolves worktree changes transparently; no caller in
  `store` hand-builds change paths anymore; stale entries self-heal.

## Files affected

- `internal/store/store.go`, `tree.go`, `validate.go`, `*_test.go`
- `cmd/lessmess/*.go`

## Notes

- This task carries the plan's "weakened atomicity" risk: the two writes are
  individually atomic (WriteFileAtomic) but not jointly so; document that in
  the code comment at the split and rely on the board's visible state for
  drift detection.
