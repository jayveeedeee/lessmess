# 2026-09-12-5: Full-size terminal window

- Change ID: 2026-09-12-5
- Created: 2026-09-12
- Branch: main
- Status: see [ledger.md](ledger.md)

## Objective and context

The terminal overlay currently opens as a centered window (max 1000px × 85vh). The user wants it to open full width and full height, keeping the close button in the header as now.

## Current behavior

`.terminal-window` is `min(1000px,100%)` × `min(85vh,800px)`, centered with padding and a border radius; the overlay has 1.5rem padding.

## Target behavior

Terminal window is edge-to-edge (100% × 100%), header with session chip/title/close button unchanged.

## Scope

- `web/static/app.css`: overlay padding 0; window 100% × 100%, radius 0.

## Non-goals

- Any behavior changes (open/close/WS logic untouched).

## Acceptance criteria

1. Terminal opens edge-to-edge; close button still in the header.
2. Tests pass; `tasktracker validate` clean.

## Tasks

1. [TRM-00: Full-size terminal CSS](tasks/00-full-size-terminal.md)
