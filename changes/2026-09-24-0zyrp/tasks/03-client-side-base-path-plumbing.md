# MP-03: Client-side base-path plumbing

## Goal

Templates and `app.js` build every browser-side URL from one injected base constant, so the whole UI works unchanged under `/p/<slug>/`.

## Approach

- `layout.html`: `<body data-base="{{.BasePath}}">`; `app.js` reads it once (`const BASE = document.body.dataset.base || ""`).
- Replace all root-absolute client calls — every `fetch("/…")` (≈55) and `new EventSource("/events")` — with `BASE + "/…"`.
- Prefix static template URLs in all templates (`layout.html`, `index.html`, `board.html`, `explorer.html`, `settings.html`, `opencode.html`, `partials.html`): `href`/`src` for `/static/…`, `/icon.svg`, nav links, and every `hx-get`/`hx-post` attribute, using `{{.BasePath}}`.
- Grep audit at the end: no remaining un-prefixed root-absolute URL strings in `web/static/app.js` or `href="/`/`hx-*="/` in `web/templates` (whitelist: none expected; document any deliberate exception here).
- Legacy mode (`BasePath == ""`) must produce identical markup and behavior — no double slashes when concatenating.

## Files affected

- `web/templates/` (all templates)
- `web/static/app.js`

## Verification

- Automated grep audit passes (record output as evidence).
- Manual: hub mode with a fixture repo — board loads, drag/move, SSE live update, chat open/prompt, settings save, explorer navigation all work under `/p/<slug>/`; legacy `--dir` mode spot-checks the same flows.
- `go build` + rebuild reminder: template/static changes require binary rebuild before manual checks.
