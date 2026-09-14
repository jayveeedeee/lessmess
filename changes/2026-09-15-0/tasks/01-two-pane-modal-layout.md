---
id: MTOC-01
title: Two-pane modal layout in templates and CSS
---

# MTOC-01: Two-pane modal layout in templates and CSS

Status: see [../ledger.md](../ledger.md).

## Objective

Restructure the plan, task, and ledger modals into head + two-pane body:
left TOC pane, 1px vertical separator, right content pane — each scrolling
independently.

## Dependencies

- MTOC-00 (the `ledgerDetail` partial must exist so all three partials are
  restructured in one pass).

## Scope

- `web/templates/partials.html` — `planDetail`, `taskDetail`, `ledgerDetail`.
- `web/static/app.css` — pane layout, separator, TOC styling scaffolding.
- No JS in this task (the TOC `<nav>` ships empty; MTOC-02 fills it).

## Implementation steps

1. Wrap the body of all three partials in:
   `<div class="modal-panes"><nav class="modal-toc" hidden></nav><div
   class="modal-body prose">{{markdown .Body}}</div></div>`.
2. CSS: `.modal-panes { display:flex; flex:1; min-height:0; }`;
   `.modal-toc { width:210px; flex:none; overflow-y:auto;
   border-right:1px solid var(--border); padding:1rem .9rem; }` with
   `[hidden] { display:none; }`; `.modal-body` keeps its own
   `overflow-y:auto` and gains `flex:1; min-width:0`.
3. `.modal.has-toc { width: min(880px, 100%); }` for the widened variant
   (class applied by MTOC-02 when the TOC is populated).
4. Confirm the notif/commit modals are unaffected: they use `.modal-body`
   without `.modal-panes`, so they keep their current layout.
5. Give `.modal-body h2/h3` a `scroll-margin-top` so scrolled-to headings
   are not flush against the pane edge.

## Verification

- Rebuild the binary (assets are embedded) and open plan + task + ledger
  modals: empty left rail visible once MTOC-02 unhides it; content scrolls
  in its own pane; separator is exactly 1px.
- Notif and commit modals render unchanged.
- `go vet ./... && go test ./...` green (template parse is covered by
  existing render tests).

## Completion criteria

- All three detail partials use the pane structure; layout CSS in place;
  other modals unaffected; tests green.

## Files affected

- `web/templates/partials.html`, `web/static/app.css`.

## Notes

- The embed means template/CSS changes are invisible until
  `CGO_ENABLED=0 go build -o lessmess ./cmd/lessmess` and a server restart —
  a known repo gotcha for verification.
- Verification evidence (2026-09-15): binary rebuilt and the 9090 server
  restarted; served HTML for plan, task, and ledger partials contains
  `.modal-panes` with the hidden `.modal-toc` rail; notif/commit modals use
  `.modal-body` without panes and are unchanged; template-parse covered by
  the existing render tests (green).
