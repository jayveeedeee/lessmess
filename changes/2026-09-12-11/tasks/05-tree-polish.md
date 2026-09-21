
# EXD-05: Remove tree chat buttons and square orange selection

Status: see [../ledger.md](../ledger.md).

## Objective

User review feedback on the merged design: remove the ✦ chat button from the
left folder tree (chat stays only in the detail pane header), and replace the
inset accent-bar selection style with a square orange highlight covering the
whole folder name, matching the UI's squared-off look.

## Dependencies

- EXD-01, EXD-02, EXD-03 (the shipped master/detail implementation)

## Scope

- `web/templates/explorer.html`: drop the `.explorer-chat` button from
  `explorerNode` (keep it in `explorerDetail`).
- `web/static/app.css`: restyle `.xtree summary.selected`; drop the
  tree-specific chat margin rule.
- `web/static/app.js`: comment refresh only (chat handler stays for the
  detail header button).
- `internal/server/explorer_test.go`: tree fragment must no longer render
  chat buttons.

## Implementation steps

1. Remove the chat `<button>` from the `explorerNode` summary in
   `explorer.html`.
2. CSS: delete `.xtree summary .explorer-chat { margin-left: auto; }`;
   replace the selected rule's surface background + inset box-shadow with a
   square orange highlight on the folder name itself
   (`.xtree summary.selected .xdir-name { background: var(--accent); … }`),
   no border-radius.
3. Update the capture-phase comment in `app.js` (chat no longer sits in
   summaries); keep the handler — it serves the detail-header button.
4. Update `TestExplorerTreeFragmentRender`: assert `explorer-chat` is absent
   from the tree fragment (detail fragment test still covers the button).
5. Rebuild, restart the server, screenshot-check the tree.

## Verification

- `go vet ./... && go test ./...` green.
- Headless screenshot: tree rows have no chat button; the selected folder
  name shows a full square orange highlight; hover still highlights rows;
  detail header chat button remains and works.

## Completion criteria

- No chat UI anywhere in the left tree; selection is a square orange
  highlight on the folder name; tests and live check pass.

## Files affected

- `web/templates/explorer.html`
- `web/static/app.css`
- `web/static/app.js`
- `internal/server/explorer_test.go`

## Notes

- `data-dir` left the tree with the chat button, so the tree test now pins
  `data-rel` for node identity and asserts `explorer-chat` is absent; the
  detail fragment test still covers the header chat button's `data-dir`.
- The selection handler in `app.js` needed no logic change — it toggles the
  `.selected` class on the summary and CSS now targets
  `.selected .xdir-name`.
- Verified 2026-09-12: `go vet` clean, `go test ./...` all green,
  `./tasktracker validate` OK; headless screenshot on the rebuilt :9090
  server shows a chat-free tree and the square orange folder-name highlight
  on the selected row; detail header chat button intact.
