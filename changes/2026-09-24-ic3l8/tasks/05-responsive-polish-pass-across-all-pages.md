# UI-05: Responsive polish pass across all pages

## Why

The user asked for the whole app to be mobile-first (desktop still fine). The structural work
( cards, chat sheets, board removal) lands in UI-00…UI-04; this task is the layout/ergonomics
sweep over everything else — no functional changes.

## Current behavior

- The chat overlay already has 840px/480px rules; explorer stacks under 720px; the header densifies
  under 480px. Modals (detail, commit, notifications) center with fixed max-widths and feel cramped
  on phones; settings, setup, and the opencode page have partial 640px rules; no safe-area insets
  despite `viewport-fit=cover`.

## Target behavior

- Modals (`.modal` — task/plan/review/ledger detail, commit window, notifications): near-full-screen
  sheets on ≤640px with sticky headers/footers where content scrolls; the TOC rail stays hidden
  there (it already hides under 2 headings).
- Chat: verify the composer, sheets (Work / Sessions / Agents / Controls), reference picker, and
  inbox remain thumb-friendly after UI-01's additions; touch targets ≥44px on rows, chips, and
  menu items.
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
