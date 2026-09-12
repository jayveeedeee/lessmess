---
id: TRM-00
title: Full-size terminal CSS
---

# TRM-00: Full-size terminal CSS

Status: see [../ledger.md](../ledger.md).

## Objective

Make the terminal window open edge-to-edge while keeping the header with the close button.

## Dependencies

None.

## Scope

In scope: `web/static/app.css` terminal overlay/window rules.
Out of scope: JS behavior.

## Implementation steps

1. `#terminal-overlay`: padding 0.
2. `.terminal-window`: width/height 100%, border-radius 0.
3. Verify with a rendered test page screenshot.

## Verification

- Screenshot shows edge-to-edge window with header and close button; `go test ./...` passes; validate OK.

## Completion criteria

- Plan acceptance criteria met.

## Files affected

- `web/static/app.css`

## Notes

- Verified (2026-09-12): fresh screenshot shows the terminal window truly edge-to-edge with the header (chip + title + ✕) fixed at the top; `go test ./...` green; `tasktracker validate` OK. An earlier screenshot showing a centered window was a stale file artifact, not the served CSS.

