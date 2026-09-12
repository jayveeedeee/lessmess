# Ledger — 2026-09-12-8

- Change ID: 2026-09-12-8
- Plan: [plan.md](plan.md)
- Branch: —
- Overall status: Done
- Last updated: 2026-09-12

## Status definitions

| Status | Meaning |
| --- | --- |
| Not started | Work has not begun. |
| In progress | Implementation or verification is actively underway. |
| Blocked | Work cannot continue until a documented dependency, decision, approval, or external condition is resolved. |
| Done | All verification and completion criteria in the task file have passed. |
| Cancelled | The task was intentionally removed from scope and the reason is recorded. |

## Tasks

Row order is display and priority order; top row is highest priority.

| Task | Title | Status | Depends on | Updated | Notes |
| --- | --- | --- | --- | --- | --- |
| [EXP-00](tasks/00-dir-docs-reader.md) | Exported per-dir docs reader | Done | — | 2026-09-12 | internal/docs/readdocs.go; placeholder/missing/corrupt semantics tested |
| [EXP-01](tasks/01-explorer-page-fragment.md) | Explorer page and tree fragment | Done | EXP-00 | 2026-09-12 | /explorer + /explorer/tree + nav link; smoke-checked on this repo |
| [EXP-02](tasks/02-directory-chat.md) | Directory chat sessions | Done | EXP-01 | 2026-09-12 | explorerPrompt + POST /explorer/chat + JS; bad-dir/service-down covered |
| [EXP-03](tasks/03-docs-watcher-sse.md) | Docs watcher and SSE live refresh | Done | EXP-01 | 2026-09-12 | docswatch.go + /events merge + expansion-preserving swap |
| [EXP-04](tasks/04-dogfood-docs.md) | Dogfood and documentation | Done | EXP-01, EXP-02, EXP-03 | 2026-09-12 | Live: tree renders, chat grounded (transcript checked), SSE fires; README updated |

## Decision log

- 2026-09-12 — Agreed with user in discussion: docs-backed tree (covered dirs + STRUCTURE.md purposes/blurbs), dedicated /explorer page, explorer chats in the unassigned bucket (Discussions list), SSE live refresh (user's explicit choice over load-on-visit). Prefix EXP registered in root ledger.
