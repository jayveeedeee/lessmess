# web

<!-- tasktracker:begin -->
- Purpose: Go package holding the tasktracker web UI's embedded assets — `templates/` (html/template sources) and `static/` (CSS, JS, vendored libraries).
- `web.go` is the whole package: it exposes `FS embed.FS` via `//go:embed templates static`, with no runtime logic of its own.
- Static assets include app CSS/JS plus vendored htmx, Sortable, and xterm libraries; see `static/VENDOR.md` before updating them.
- Gotcha: files must exist at build time; adding a new top-level asset directory or file requires updating the `//go:embed` list in `web.go`.
- Template naming and rendering conventions live in `templates/AGENTS.md`; template names are parsed into one global set.
- (manual) Root `agentsdocs.json` excludes only `web/static`, so JS/CSS there is outside docs coverage while `templates/` remains covered by the doc gardener.
- (manual) Because `//go:embed templates static` is a directory embed, new template and static files are picked up automatically as long as they live under those trees; `explorer.html` is parsed into the same global template set as the rest.
- (2026-09-12-8) The explorer's client behavior lives in `static/app.js` (a `docs` SSE listener that refreshes `#explorer-tree` and restores open nodes, plus a delegated `.explorer-chat` click that opens the created session in the terminal overlay) and its tree styling in `static/app.css`; both are under docs-excluded `web/static`.
- (2026-09-12-11) The explorer is now master/detail: `static/app.js` tracks the selected directory, restores open nodes plus the selection after a tree swap, and re-fetches `#explorer-detail`; tree fragments injected via `innerHTML` must be passed to `htmx.process` so the new `hx-get` summaries are wired, and the chat button moved to the detail header so its propagation guard runs in the capture phase.
- (2026-09-12-9) The docs notification bell/badge and findings modal are client-rendered in `static/app.js`: `checkValidation` caches docs findings (the banner is now violations-only), the badge re-renders on each check and on the `tt:docs-event` custom event, and the modal's refresh button posts `/docs/refresh` with a busy/result state that re-enables on the next docs SSE event, with styling in `static/app.css` (both under docs-excluded `web/static`).
<!-- tasktracker:end -->
