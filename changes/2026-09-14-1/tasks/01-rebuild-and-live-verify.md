---
id: DSC-01
title: Rebuild and verify fresh opening live
---

# DSC-01: Rebuild and verify fresh opening live

Status: see [../ledger.md](../ledger.md).

## Objective

Prove in the running UI that a new discussion session opens fresh (one invite line, no repo reads) while the Settings Change flow still engages immediately.

## Dependencies

- DSC-00.

## Scope

- Rebuild the binary, restart the running server, and run the two live checks below. No code changes in this task.

## Implementation steps

1. Build: `CGO_ENABLED=0 go build -o lessmess ./cmd/lessmess`.
2. Restart the running `lessmess serve` process so the recompiled prompt text is live (prompt strings are compiled in).
3. Live check A: from the changes index page click **New change session**, open the terminal, and confirm the session's first and only output is a single short invite line with no tool calls and no mention of open changes. Then send a small idea and confirm normal planning behavior (investigation, options, recommendation).
4. Live check B: use a Settings-page **Change** button and confirm the discussion engages on the named field immediately instead of stalling on the invite.

## Verification

- Check A observed in the terminal overlay; record what the session opened with.
- Check B observed; record the field and the immediate response.
- Server log shows `discussion session created` for both, no errors.

## Completion criteria

- Both live checks behave as specified on newly created sessions.
- Existing sessions untouched (spot-check one old discussion still behaves as before).

## Files affected

- None (build output `lessmess` only).

## Notes

- If Check A still shows repository investigation, the precedence wording lost to the appended addendum: record the transcript here and fall back to a small `prompts.discussion` tweak via `PUT /api/settings?scope=project` (config-only, no rebuild) as agreed in the plan's risks.
