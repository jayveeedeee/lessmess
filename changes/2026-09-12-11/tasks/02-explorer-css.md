
# EXD-02: Explorer CSS — two-pane layout, guide lines, selection

Status: see [../ledger.md](../ledger.md).

## Objective

Restyle the explorer section of `app.css` for the master/detail layout:
split panes, tree guide lines, selection highlight, always-visible
right-aligned chat button, and readable detail typography.

## Dependencies

- EXD-01 (the new markup structure and class hooks)

## Scope

- `web/static/app.css` (explorer section only).

## Implementation steps

1. `.explorer-split`: flex row with a gap; left pane (`.xtree` wrapper /
   `#explorer-tree`) fixed width ~300px, independently scrollable; right pane
   `#explorer-detail` flexes to fill.
2. Guide lines: nested `.xtree ul` gets a left border (var(--border)) and
   tuned padding/margin so vertical lines connect parents to children.
3. Chat button: drop the hover-only `visibility` rule; keep it visible and
   push it right with `margin-left: auto` inside the flex summary row.
4. Selection: `.xtree summary.selected` style (surface background + subtle
   accent) applied by JS.
5. Detail pane: header block (name, muted `.Rel` path, chat button),
   `.xpurpose` as a full-width paragraph, `.xdetail-files` rows — file name
   on one line, `.xblurb` on the line below, muted placeholder styling kept.
6. One simple media query: stack the panes vertically on narrow screens.

## Verification

- Rebuild, run the server, open `/explorer`: panes sit side by side, guide
  lines show nesting depth, chat button is right-aligned and visible without
  hover, selected row is highlighted after clicking, detail pane typography
  reads as name-then-blurb.
- Resize the window narrow: panes stack without horizontal overflow.

## Completion criteria

- All six steps visible in the running UI; no CSS changes outside the
  explorer section.

## Files affected

- `web/static/app.css`

## Notes

- Reused existing CSS variables (`--surface-2`, `--border`, `--muted`,
  `--accent`) for consistency.
- Tree pane is `position: sticky` with viewport-bounded scroll instead of a
  header-dependent height calc.
- Verified 2026-09-12 via headless Chrome screenshots against the rebuilt
  server: wide (1400px) shows the split with guide line, selected root row
  with accent edge, right-aligned always-visible chat buttons, and
  name-then-blurb file rows; narrow (640px) stacks panes without horizontal
  overflow.
