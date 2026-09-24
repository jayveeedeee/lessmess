# UI-05: Responsive polish pass across all pages

## Why

The user asked for the whole app to be mobile-first (desktop still fine). The structural work
( cards, chat sheets, board removal) lands in UI-00…UI-04; this task is the layout/ergonomics
sweep over everything else — no functional changes.

## Current behavior

- The chat surface was reshaped by change 2026-09-21-aw36x (commit be42a6c): the composer floats
  over the transcript with ResizeObserver-tracked clearance; its options expand in place as a
  quick-action grid (Plan / Tasks / Runtime / Compact) toggled by a button carrying the
  context-usage ring (MAC-76 centers that control); send/interrupt is one stateful icon button;
  the controls sheet split into a compact Runtime variant plus `.chat-controls-extended` sections;
  transcript standalone events render as compact rows. The app bar gained the `#location-back`
  chevron beside the location dropdown. Explorer stacks under 720px; the header densifies under
  480px. Modals (detail, commit, notifications) center with fixed max-widths and feel cramped on
  phones; settings, setup, and the opencode page have partial 640px rules; no safe-area insets
  despite `viewport-fit=cover`.

## Target behavior

- Modals (`.modal` — task/plan/review/ledger detail, commit window, notifications): near-full-screen
  sheets on ≤640px with sticky headers/footers where content scrolls; the TOC rail stays hidden
  there (it already hides under 2 headings).
- Chat: verify the floating composer, quick-action grid, Sessions sheet (UI-01), Work panel,
  Agents drawer, Runtime variant + extended controls, reference picker, and inbox remain
  thumb-friendly; touch targets ≥44px on rows, chips, quick actions, and the composer buttons (the
  options control's 44px box lands with MAC-76 — build on it, don't resize it again).
- Explorer: tree/detail stacking and the per-directory chat button at 390px; Settings: scope bar,
  section nav, field rows, swatches; Setup: step nav and controls; opencode status page: existing
  640px rules extended to the new layout.
- Header: brand/location/actions density at 360–480px; safe-area insets (`env(safe-area-inset-*)`)
  for the sticky header, chat composer, and sheets.
- Desktop (≥1200px) remains visually unchanged apart from the card list, which renders as a
  comfortable wide list; light theme spot-check on all new surfaces.

## Verification

- Manual pass at 390×844 and 768px over: Changes, Chat + every sheet, task/plan/commit/notif
  modals, Explorer, Settings, Setup, opencode page — no horizontal scroll, no clipped controls,
  keyboard push-in works on the composer (`interactive-widget=resizes-content` is already set).
- Desktop sanity at 1440px: nothing functionally moved for existing flows.
- `go vet ./... && go test ./...` pass (CSS-only changes; assets version bump handled by the
  existing content-hash `AssetsV`).
