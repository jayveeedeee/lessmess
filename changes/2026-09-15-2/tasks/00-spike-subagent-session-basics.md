---
id: PSB-00
title: Spike — subagent child promptability and ID visibility
---

# PSB-00: Spike — subagent child promptability and ID visibility

Status: see [../ledger.md](../ledger.md).

## Objective

Answer, against the live opencode service, the two questions the whole change rests
on, before any production code is written:

1. Can a finished subagent child session (one with `parentID` set) be prompted
   directly via `POST /api/session/{id}/prompt`, and does it continue under its
   subagent agent config?
2. Does the task-tool result text surfaced to the parent contain the child session
   ID (i.e. can a change session bind a sub by ID first-party)?

## Dependencies

- None. This is the go/no-go gate; run it first.

## Scope

- Live API calls only (via `opencode2 api` or the Go client in a scratch program).
- Recording findings in this task's Notes and the ledger decision log.
- Optionally: creating one throwaway subagent child in a scratch session to have a
  fresh specimen with a known parent/child pair.

## Implementation steps

1. Find an existing child session: `GET /api/session`, filter entries with
   `parentID`; note one whose parent still exists.
2. Prompt it directly: `POST /api/session/{childID}/prompt` with a short question
   referencing its prior task; then `POST /api/session/{childID}/wait` and read the
   reply via the message list. Record the agent used and whether the service
   accepted, queued, or rejected the prompt.
3. For question 2: inspect a parent transcript around a task-tool call — the
   `/message` list may strip tool outputs, so try `/message/{messageID}` for the
   full part. If transcript inspection is inconclusive, run one controlled spawn in
   a scratch session whose prompt asks the agent to echo the task-tool result
   verbatim.
4. Write both answers into Notes, and update the ledger decision log with the
   consequence for PSB-04's prompt wording (first-party bind curl vs. title
   convention only).

## Verification

- Both questions have a recorded yes/no with evidence (API responses or transcript
  excerpts) in this file's Notes.

## Completion criteria

- Question 1 answered with evidence; if no, the change is rescored before PSB-02+
  proceed.
- Question 2 answered with evidence; PSB-04 wording decision recorded.

## Files affected

- None (this file and the ledger only).

## Notes

- **Q1 — promptability: YES (verified live, 2026-09-15).** Finished `explore` child
  `ses_f829fe038ffeCqeoaMLtqm960D` (7 days idle, tevr-core project) accepted
  `POST /api/session/{id}/prompt` with `{"text": ...}`; run completed in ~0.5 s
  (`finish: "stop"`, content phase `final_answer`) and replied exactly `OK`, under
  its own agent config (explore / gpt-5.6-sol variant max). A system message
  re-injects date + project AGENTS.md diff, i.e. the sub continues as that agent in
  its own project context. Note: the prompt response reported
  `delivery: "steer"` yet processed normally on an idle session.
- **Q2 — child ID visible to the parent model: NO.** Controlled spawn (scratch
  parent `ses_f5db61133ffeaNMCexVsHW3KM7` in a temp dir, since deleted): the parent
  spawned an `explore` child via the task tool and, when explicitly instructed to
  echo the COMPLETE raw tool result character-for-character, produced only the
  child's final text (`DONE`). Zero `ses_` occurrences in parent-visible text across
  both inspected transcripts (tevr parent: 50 messages; spike parent: 3). The HTTP
  message API exposes no tool parts to double-check. Caveat: echo fidelity is
  probabilistic evidence, but the design now assumes the parent cannot supply the
  sub's session ID.
- **Title convention: CONFIRMED.** Child session title == task-tool `description`
  verbatim: spawning with description `PSB-99: title convention check` produced a
  child titled exactly that. So a prompt rule "prefix the description with
  `TSK-NN: `" puts the task ID where the reconciler can read it.
- **Consequences (recorded in the ledger decision log):** the parentID reconciler is
  the primary mapper; the bind endpoint is corrective/user-facing (caller `session`
  becomes optional); PSB-04 teaches description-prefixing, not a bind curl.
