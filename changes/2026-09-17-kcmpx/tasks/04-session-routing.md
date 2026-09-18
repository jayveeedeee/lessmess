---
id: WTP-04
title: Route change sessions, terminal, and commits into the worktree
---

# WTP-04: Route change sessions, terminal, and commits into the worktree

Status: see [../ledger.md](../ledger.md).

## Objective

Every session spawned *for* a change with an active worktree runs with the
worktree as its working directory — change sessions, task subagent sessions,
commit sessions, and board-created sessions — plus the per-change terminal.

## Dependencies

- WTP-03 (resolver tells us where a change lives).

## Scope

- `spawnSession` gains a directory-aware variant (e.g.
  `spawnSessionFor(ctx, dir, title)`); the existing `spawnSession` keeps
  delegating with `s.st.Dir` for unassigned/repo-wide flows.
- Call sites that spawn *for a change*: `createChangeSession` (mapping.go),
  `commitChange` (lifecycle.go), `reconcileTaskSessions` (autosession),
  handoff-spawned sessions, scaffold-bound discussion rename (unchanged — that
  session already exists). Each resolves the change's root via the store
  resolver.
- Terminal: PTY cwd selection follows the same resolution for change-scoped
  terminals (verify how terminal sessions bind today in `terminal.go` and keep
  unassigned terminals on the main tree).
- `changePrompt`/`taskPrompt` gains a worktree stanza when the change has one:
  "your working directory is the worktree for change X on branch Y; changes/
  edits happen here; the root ledger is maintained centrally in the main tree —
  never edit it" (full wording in WTP-07; minimal correct stanza here).
- `opencode.json` handling verification: confirm in a real worktree whether the
  copied `opencode.json` (WTP-01 `CopyProjectConfig`) yields the expected agent
  permissions; adjust copy/symlink strategy if not.

## Implementation steps

1. Add the directory-aware spawn variant and thread the resolver through the
   listed call sites (nil-disabled when no worktree).
2. Terminal cwd: locate where the PTY is created per session/change, resolve
   like spawns, keep unassigned main-tree.
3. Add the prompt stanza conditional on worktree presence.
4. Tests with the injectable spawn seam: capture requested directory per call
   site; terminal cwd test following `terminal_test.go` patterns.
5. Manual verification in a real worktree: spawn a change session, confirm the
   agent's file edits land in the worktree; open the terminal, confirm cwd;
   run the commit button, confirm the commit lands on the change branch.

## Verification

- Automated: per-call-site directory assertions with fake clients.
- Manual: the three real-worktree checks above, plus `opencode.json` discovery
  note recorded in this task's Notes.

## Completion criteria

- No session spawned for a worktree change runs in the main tree; unassigned
  and repo-wide flows are unchanged; prompts state the root-ledger rule.

## Files affected

- `internal/server/settings.go` (spawn variant), `mapping.go`, `autosession.go`,
  `lifecycle.go`, `changesession.go`, `terminal.go`
- `internal/server/*_test.go` for each touched flow

## Notes

- The scaffold-firing discussion session stays main-tree by design (directory
  is immutable post-creation); it received branch + worktree in the scaffold
  response (WTP-02). Record the `opencode.json`-in-worktree verification
  result here — it decides whether WTP-01's copy step needs a follow-up.
