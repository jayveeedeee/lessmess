
# SPS-02: Settings split-shell layout and content de-clutter

Status: see [../ledger.md](../ledger.md).

## Objective

Restructure the settings page into a proper app-style two-pane layout: a roomy
left side menu (title, group navigation, footer help) flush under the scope strip,
and a content pane filling the rest — removing the 860px page cap and the
intro-paragraph stack that make the page feel condensed.

## Dependencies

SPS-01 (the shell positions the scope strip and sidebar together; the strip must
exist first).

## Scope

- `web/templates/settings.html` —
  - Wrap the page in a split-shell structure modeled on the explorer page:
    sidebar pane + content pane.
  - Sidebar: `h1` "Settings" as its header, the existing `.settings-nav` group
    buttons (unchanged `data-group` values), and a small footer help block
    receiving the moved text: the layer explanation (project → `lessmess.json`
    committed, personal → `.lessmess/settings.json` gitignored, personal wins;
    badges show each value's source — may be condensed) and the
    "Re-run the onboarding wizard" link to `/setup`.
  - Keep all five `.settings-section` blocks, their `data-section` values,
    `[data-save]` buttons, `data-field`/`data-kind`/`[data-input]` hooks,
    datalists, error/hint elements, and the exclusions editor markup unchanged.
- `web/static/app.css` —
  - Split-shell rules: sidebar ~13–14rem, sticky with `top` offset =
    header (52px) + scope strip height, `max-height` + `overflow: auto` like
    `#explorer-tree`; content pane `flex: 1; min-width: 0`.
  - Restyle `.settings-nav` buttons: more padding, roomier typography (mono names
    may stay if they read well), clearer active accent state.
  - Sidebar footer help styling (small, muted).
  - Remove the superseded rules: `.settings { max-width: 860px }`,
    old `.settings-body` layout, old `.settings-head` remnants.
  - Responsive fallback (<640px): sidebar stacks above content, nav becomes a
    wrapping horizontal row (as today), scope strip remains sticky.

## Implementation steps

1. Restructure `settings.html` into the split shell; move the `h1`, intro
   paragraphs, and wizard link into the sidebar per scope.
2. Write the shell, sidebar, footer, and responsive CSS; sweep superseded rules.
3. Verify `initSettings` still finds every hook (`#settings-page`,
   `.settings-nav`, `data-*`, radios, badges, status, datalists) with zero JS
   changes.
4. Check the longest section (Prompts, six textareas) scrolls sensibly beside the
   pinned sidebar; confirm section switching and URL-hash restore still work.

## Verification

- Rebuild the binary; visually confirm the two-pane layout with the sidebar
  pinned under header + strip while content scrolls.
- Toggle scope, switch all five sections, save a section, use a per-setting
   Change button and the exclusions editor — all behave as before.
- `go test ./...` — all existing settings assertions (including "Prompt addenda",
  `class="settings-nav"`, "lessmess.json", ".lessmess/settings.json") pass
  unchanged.
- Both themes; narrow viewport shows the stacked fallback.

## Completion criteria

- Settings renders as a split shell with a proper side menu; no intro-paragraph
  stack above the body; page no longer capped at 860px.
- All settings interactions work unchanged with no `app.js` edits.
- Tests green without assertion edits.

## Files affected

- `web/templates/settings.html`
- `web/static/app.css`

## Notes

- The explorer page (`#explorer-tree` / `.explorer-split`, `app.css:681-723`) is
  the reference implementation for the sticky-panel + flex-detail pattern.
- Evidence (2026-09-14): served HTML has `settings-split`/`settings-side` with
  h1 + `.settings-nav` + footer help inside the sidebar, error/hint paragraphs
  atop `.settings-content`, and all five sections unchanged; every string
  asserted by `render_test.go` verified present in the served page ("Prompt
  addenda", `class="settings-nav"`, `lessmess.json`, `.lessmess/settings.json`,
  `name="settings-scope"`, `data-field="docs.autoGardenerOnClose"`,
  `class="settings-change"`, `id="settings-page"`); `grep` confirms zero stale
  `settings-head`/`scope-toggle`/`settings-body` references anywhere; sidebar
  sticky offset uses `--scopebar-h` (2.6rem) + 52px header for consistency.
