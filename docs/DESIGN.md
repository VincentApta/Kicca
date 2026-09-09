# DESIGN — kicca

## Design Read

**Reading this as:** internal project-management tool (kanban task manager) for dev/ops teams, neumorphic ("soft UI") language, dark-default, leaning shadcn/ui + Tailwind custom tokens.

Neumorphism constraint: real neumorphism needs low-contrast monochrome surfaces, which fights WCAG AA and data density. Ruling: **soft-shadow, extruded-surface neumorphism** — tactile raised cards, inset inputs, no harsh borders — but text stays full contrast and surfaces keep distinguishable tones. No washed-out 5%-gray text on 10%-gray cards.

## Dials

| Dial | Value | Meaning |
|------|-------|---------|
| DESIGN_VARIANCE | 3/10 | calm, grid-true tool |
| MOTION_INTENSITY | 3/10 | drag feedback + subtle lifts only |
| VISUAL_DENSITY | 6/10 | board cards compact, detail drawer roomy |

## Design System

- **System:** shadcn/ui (Base UI primitives — NOT Radix) + Tailwind v4 custom tokens
- **Install:** `npx shadcn@latest init`
- **Why:** known stack, composable, tokens easy to bend neumorphic.

Known Base UI pitfalls (must respect): no `items` prop on Select.Root; no `DropdownMenuTrigger render={<Component/>}` composition (invert); `DropdownMenuLabel` needs `DropdownMenuGroup` wrapper.

## Color Palette

Dark default. Same tokens, flipped values in light mode.

| Token | Dark Hex | Light Hex | Usage |
|-------|----------|-----------|-------|
| bg-primary | #1c1f26 | #e8ecf1 | app background |
| bg-surface | #232733 | #f2f5f9 | cards, columns, modals |
| bg-inset | #1a1d24 | #dde2e9 | inputs, wells |
| text-primary | #edf0f4 | #1b1f27 | body, titles |
| text-muted | #97a0af | #5b6472 | labels, meta |
| accent | #2dd4bf | #0d9488 | teal — buttons, links, focus, active |
| border | #2e3442 | #c9d1dc | hairlines (used sparingly; shadows do the work) |

Priority colors: urgent `#f87171`, high `#fbbf24`, medium `#60a5fa`, low `#94a3b8`. Status colors: done `#34d399`, blocked `#f87171`, in_progress `#60a5fa`, review `#c084fc`, inbox `#fbbf24`, backlog `#97a0af`.

**Theme:** both, dark default, toggle persisted (localStorage + `class` strategy).

## Neumorphic Surface Recipes (canonical)

- **Raised card:** `bg-surface; shadow: 6px 6px 12px rgba(0,0,0,.45), -4px -4px 10px rgba(255,255,255,.04); radius 12px`
- **Inset input:** `bg-inset; shadow: inset 3px 3px 8px rgba(0,0,0,.45), inset -2px -2px 6px rgba(255,255,255,.05); radius 8px`
- **Pressed toggle/button:** inset variant of raised; accent glow on active: `0 0 0 1px accent, 4px 4px 10px rgba(45,212,191,.25)`
- Light mode flips shadow alphas (~.18 / white .9). Define as Tailwind component classes `card-neu`, `inset-neu`, `btn-neu` — one place, reused everywhere.

**Contrast floor overrides neumorphism:** all text uses text-primary/text-muted tokens; never rely on shadow alone to separate interactive elements (add 1px accent/border on focus + hover).

## Typography

| Role | Font | Weights | Size Scale |
|------|------|---------|------------|
| Display | Outfit | 600 | text-2xl (page titles only) |
| Body | Inter Tight | 400/500 | text-sm base, text-lg detail |
| Mono/Data | JetBrains Mono | 400/500 | text-xs — task keys `KIC-123`, numbers, dates |

Self-host via `@fontsource` packages. No Google Fonts `<link>`.

## Layout Direction

### Board (home of project)
Full-width horizontal scroll of 6 columns (Inbox, Backlog, In Progress, Review, Done, Blocked) — Trash not on board. Column = inset well; cards = raised neumorphic, compact (title, key-number mono, priority dot, assignee avatar, labels chips, GH link badge `GH #12`). Drag = dnd-kit; card lifts (scale 1.02 + stronger shadow). Column header: name, count, "+" add. Left sidebar (240px, collapsible): logo, project switcher, My Tasks, Trash, admin section (Teams/Users). Topbar: search, filters, theme toggle, user menu.

### Task detail (drawer, right, 480px)
Overlay raised panel slides in. Title large editable inline, mono key beside. Description markdown textarea + rendered. Fields grid (assignee, priority, due, labels) inset selects. Comments thread bottom, inset composer. "Create GitHub issue" button (or GH link card if linked).

### List view
Table toggle from board: columns Key, Title, Status, Priority, Assignee, Labels, Due, Updated. Rows raised-on-hover, sticky header, filters shared with board.

### Admin (Users / Teams / Project settings)
Simple inset panels, forms neumorphic inputs, member tables with role selects.

## Signature Element

Column wells: each kanban column an extruded-inset tray against the soft-embossed board background — cards sit *in* trays, not floating on flat list. Priority as small luminous dot + status underline on card key.

## Anti-Defaults

- ❌ No flat white SaaS board with 1px gray borders — surfaces must be tactile (raised/inset shadows)
- ❌ No purple/blue default gradient buttons, no glassmorphism/blur panels
- ❌ No giant centered hero/empty-state illustration; empty columns show compact CTA text
- ❌ No low-contrast gray-on-gray text regardless of neumorphism pressure
- ✅ Dense, calm, keyboard-reachable; skeletal loaders matching final layout

## Component States

- **Loading:** skeletons shaped like columns/cards; drawer skeleton shaped like detail.
- **Empty:** each column type gets one-line CTA ("Drop raw requests here"); board empty project → "Create first task".
- **Error:** inline under inputs; toast for transient (move failed, GH error incl. upstream message).
- **Success:** toasts verb-matched ("Moved to Review", "Issue created — GH #12").

## Icon System

- **Library:** lucide-react
- **Stroke width:** 1.5
- **Install:** shipped with shadcn

## Accessibility Floor

- WCAG AA both themes — neumorphic surfaces must not drop text contrast below 4.5:1
- Full keyboard nav: menus, drawer, forms, move-actions menu per card (DnD pointer-only acceptable v1)
- Visible focus ring (accent), `prefers-reduced-motion` disables lift/slide animations

## Shape Language

| Element | Radius |
|---------|--------|
| Buttons | 8px |
| Cards / columns / drawer | 12px |
| Inputs | 8px |
| Modals | 16px |

## Responsive Strategy

- **Desktop primary:** ≥1280px full board.
- **<1024px:** sidebar collapses to icon rail; board scrolls horizontally (native touch DnD later).
- **Mobile:** list view default; board optional horizontal scroll. `ponytail:` mobile DnD polish deferred.
