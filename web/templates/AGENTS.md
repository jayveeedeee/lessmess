# web/templates

<!-- tasktracker:begin -->
- Purpose: Go `html/template` sources for the tasktracker web UI; each file defines named templates rather than serving standalone pages.
- `layout.html` defines the base `layout` shell and renders the `content` template; page templates (`index.html`, `board.html`) only define `content`.
- `partials.html` holds shared fragments (`boardFragment`, `taskDetail`, `planDetail`, `banner`) pulled in via `{{template ...}}`, often for htmx-swapped partial responses.
- Conventions: server data lives under `.Data`, htmx attributes (`hx-get`, `hx-post`, `hx-target`) drive interactivity, and custom funcs like `statusClass`, `base`, and `markdown` are registered by the Go renderer.
- Gotcha: template names must stay globally unique across files since they are parsed into one set; renaming a `define` can break handlers that reference it.
- (2026-09-12-2) `index.html` now exposes only the opencode-driven "New change session" form (`hx-post="/changes/session"`); the plain "New change" form was removed from the UI, but the `POST /changes` endpoint remains for API clients, so do not resurrect that form without a decision.
- (2026-09-12-2) When verifying a template edit in the running UI, a stale server process holding the port can serve old HTML and make the change look unapplied; restart the server before concluding an edit did not take.
- (manual) `board.html` carries the change-lifecycle and session controls (Commit, Close change, Reopen, Sessions, New session); they are wired by DOM id in `web/static/app.js`, so keep ids such as `commit-btn`, `close-change-btn`, `reopen-btn`, `sessions-btn`, and `new-session-btn` stable when editing.
- (manual) `layout.html` owns the full-size terminal overlay (`#terminal-overlay` / `.terminal-window` with a single `#terminal-container`) where `app.js` mounts xterm; vendored htmx/Sortable/xterm assets stay at pinned `?v=1` while only `app.css`/`app.js` use the content-hashed `.AssetsV`.
- (manual) `index.html`'s Discussions section is a client-rendered `<ul id="discussions-list">` populated by `app.js`; the only server-submitting form there is the opencode session POST.
- (manual) `explorer.html` defines `content`, the dirs-only `explorerTree`/`explorerNode` fragments, and `explorerDetail`; keep the recursive `{{template "explorerNode" .}}` reference intact or the tree loses its nested directories.
- (manual) Explorer templates read `.Data.Enabled`, `.Root`, `.Dirs`, `.Files`, `.Purpose`, and `.Blurb`, which map to `explorerView`/`explorerNode` built in `internal/server/explorer.go`; a missing purpose renders the "no description yet" placeholder.
- (2026-09-12-8) Each tree `<details>` carries `data-rel` so a tree refresh can restore which nodes were open; the `.explorer-chat` button is no longer in tree summaries (removed in 2026-09-12-11), so clicking a row only selects it.
- (2026-09-12-11) The explorer page is a split shell: `#explorer-tree` holds the dirs-only tree (each `<summary>` carries `hx-get="/explorer/detail?dir={{.Rel}}"` targeting `#explorer-detail`) and `#explorer-detail` starts with root's server-rendered `explorerDetail`; the tree renders no purposes, files, or chat buttons.
- (2026-09-12-9) `layout.html`'s header carries the docs bell (`#notif-bell` plus `#notif-badge`, hidden until findings exist) and the `#notif-modal` containing `#notif-list`, `#docs-refresh-btn`, and `#notif-refresh-status`; its z-index sits below the terminal overlay, and the shared `banner` partial is now rendered only for `changes/` violations.
<!-- tasktracker:end -->
