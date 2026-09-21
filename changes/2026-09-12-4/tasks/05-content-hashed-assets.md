
# LIF-05: Content-hashed asset versioning (fix stale JS)

Status: see [../ledger.md](../ledger.md).

## Objective

Fix the Commit button (and any future JS/CSS change) being invisible to browsers because `app.js`/`app.css` are served immutable with a manually bumped `?v=` that was not bumped when the handlers were added. Replace manual bumps with automatic content-hashed asset URLs.

## Dependencies

None.

## Scope

In scope: startup-computed hash over the embedded app assets; templates reference `?v={{.AssetsV}}` for app.css/app.js (vendored pinned libs keep their pinned filenames + fixed `?v=1`); test that the hash is stable and content-sensitive.
Out of scope: endpoint changes (verified working in LIF-02).

## Implementation steps

1. Compute a short FNV hash over `web/static/app.css` + `app.js` (from the embedded FS) at renderer init.
2. Add `AssetsV` to `pageData`; use it in `layout.html` for `app.css` and `app.js`.
3. Test: same content → same hash; different content → different hash.
4. Verify served HTML shows hashed URLs and the fresh app.js contains `initLifecycle`.

## Verification

- Unit test passes; served asset URLs carry the content hash; the Commit button works in a live browser check.

## Completion criteria

- No manual `?v=` for app assets anywhere; button functional.

## Files affected

- `internal/server/render.go`, `web/templates/layout.html`

## Notes

- Root cause confirmed: `layout.html` referenced `app.js?v=10` while app.js gained handlers (sessions, lifecycle) without a bump; immutable caching kept the old script in the browser — the Commit button had no listener.
- Fix: `assetsVersion()` computes an FNV-32a hash over the embedded app assets at renderer init and injects it as `AssetsV` into every page render; `app.css`/`app.js` URLs now carry `?v={{.AssetsV}}` automatically. Vendored pinned libraries keep fixed `?v=1` (their content never changes without a versioned filename change).
- Verified: unit test (stable + content-sensitive); served HTML shows `app.js?v=32a39467`; served script contains `initLifecycle` and the `commit-btn` handler. An earlier check showing `?v=10` was yet another stale process on port 9090, not stale code.

