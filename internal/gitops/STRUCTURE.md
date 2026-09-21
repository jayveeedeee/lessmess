<!-- tasktracker:begin -->
# Structure: internal/gitops

<!-- tasktracker-meta: refreshed=2026-09-17 source=2026-09-16-j2g58 tree=c28c84ccd024 -->

Executes the mutating git and GitHub CLI mechanics behind the worktree-per-change pipeline: branches, worktrees, push, and PRs.

## Entries

| Entry | Purpose |
| --- | --- |
| `gitops.go` | git/gh client for branches, worktrees, push, PR create/comment |
| `gitops_test.go` | client tests against real temp git repositories |
| `state.go` | worktrees.json persistence (PR URL, review state) |
| `state_test.go` | round-trip and malformed-state tests for worktrees.json |
<!-- tasktracker:end -->
