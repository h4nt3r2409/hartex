# Hartex — brand, dark-neon console, and performance design

**Status:** implemented (first pass) — the shipped summary is in `CHANGELOG.md`; §11 lists what remains deferred.
**Date:** 2026-10-26
**Repo:** `/data/home/artex-global`
**Scope owner decisions already made:** Hartex is the product name in the UI, title, login, and docs; dark is the primary theme with light as a secondary option; the neon direction is our own; the mark is a geometric H monogram with a blue→violet gradient and a HARTEX wordmark; the flagship screens are Dashboard, Tasks, Findings, and System → LLM.

---

## 1. Intent

The product already works. This is a quality pass, not a rewrite. It has three outcomes:

1. **One identity.** Every visible surface says Hartex, with a new mark that is crisp at 16 px and unmistakable at 160 px.
2. **One visual system.** A dark-first, neon-accented operator console that feels designed for long security sessions — not a generic shadcn skin and not a game HUD. Light remains a usable daylight mode.
3. **Remove measured ceilings.** Ship less, compress what ships, stop polling work nobody is looking at, and take the O(history) queries off the five-second path.

### Non-goals

- No new product features, no framework migration, no database schema redesign.
- No rename of internal identifiers: Go module path, `artex` binary/service, `ARTEX_*` env vars, config/database names, or the built-in login credential.
- No rewrite of historical records: `CHANGELOG.md` entries, prior `docs/superpowers/` specs, or detection content that intentionally names the upstream ARTEX threat.
- No change to what the product does; behavior is preserved by every change.

### Anti-goals

- A neon theme that trades legibility for atmosphere.
- A rebrand that breaks existing deployments, sessions, or the upstream-attribution notice.
- A performance change that alters data semantics or hides data behind a limit the UI does not disclose.
- Churn in generated `graphify-out/graph.json` misread as noise (the project expects whole-file rebuilds).

---

## 2. What the audit found

Four read-only specialists audited the backend, frontend, UI/UX, and brand surfaces. High-severity findings were adversarially re-checked against the cited lines; the confirmed ones below are the basis of this design.

### 2.1 Delivery and weight

| Finding | Evidence | Effect |
|---|---|---|
| Embedded UI is served with no compression and no immutable caching | `server/webui_embed.go:58`; no `Content-Encoding` or `Cache-Control` in `server/` or `cmd/`; `embed.FS` reports zero `ModTime` and no ETag | Dashboard first load is ~2.06 MB raw JS+CSS (~578 KB gzipped); repeat visits re-download or revalidate all 78 hashed chunks |
| The production bundle ships the mock dataset on 26 of 30 pages | `web/src/lib/api.ts:8`; chunk `2phlf804f3174.js` is 192,573 B raw / 52,910 B gzip, referenced by 26 exported pages | ~53 KB gzip on almost every page for a path production never takes |
| The whole English catalog ships to the client on all 30 pages | `web/src/i18n/intl-runtime-provider.tsx:7`; `en.json` is 178,108 B / 2,993 strings; chunk `2u7vue631a2ji.js` is 153,300 B raw / 49,875 B gzip | ~50 KB gzip per page |
| A 1920×1920, 492,065 B logo is rendered at 40 px and 160 px, and duplicated as `icon.png` | `web/public/logo.png`, `web/src/app/icon.png` are byte-identical; served three times in `server/webui/dist` | ~1.7 MB decoded for a logo; favicon is 1920 px |
| Seven font files (272,852 B) are preloaded on every page; five are never used together | `web/src/lib/fonts/registry.ts:5` imports the Geist pixel barrel; only one pixel face is registered | ~273 KB of font preloads |
| No component-level lazy loading | `web/src/app/(main)/function/tasks/detail/page.tsx:45`; all ten detail tabs and the hidden XYFlow graph load on first paint | Heavy tabs and graphs download before they are opened |
| 28 polling timers, none visibility-gated | `web/src/app/(main)/_components/sidebar/nav-main.tsx:187` | Polls keep running in hidden tabs; duplicate intercept-pending requests |

### 2.2 Backend hot paths

All of the following were confirmed against the cited code:

| Finding | Evidence | Effect |
|---|---|---|
| `/api/stats` loads up to 100k intents and 10k goals every 5 s just to count them | `server/server.go:1071`/`:1078`; `db/exploration.go:741` selects payload; dashboard polls at `dashboard/page.tsx:194` with `setInterval(load, 5000)` | O(history) work every five seconds |
| Dashboard fetches 500 full findings including evidence text every 5 s for counts and 8 visible items | `web/src/lib/api.ts:560`; `server/server.go:840` → `ListFindings(500)` at `:2006`; `db/findings.go:105-108` selects `evidence` | Large payload, repeated |
| Evidence GC holds a global advisory lock while scanning hashes and walking the blob directory | `evidence/store.go:347-400` takes `pg_advisory_xact_lock` then scans and walks | Blocks concurrent evidence work |
| The whole exploration graph is loaded twice per planner round and again per worker round | `agent/compaction.go:338`; `loadColdGraph` runs `Nodes(allRows)`/`Edges(allRows)` with `allRows=1<<30`; `planner.go:360` and `:382` call it again | Repeated full-graph reads |
| Traffic capture holds one mutex across hashing, file writes, and the SQLite transaction | `traffic/traffic.go:439-457`; `record` runs before the proxy replies | Capture serializes behind disk I/O |
| Cold-stamp update issues one UPDATE per node per planner round | `db/digest.go:67` | Many small writes |
| Conversation token aggregate scans all chat history on every dashboard poll | `db/conversation.go:24` | O(history) every 5 s |
| Traffic page polling repeats full-table `COUNT` and `GROUP BY` | `traffic/traffic.go:915` | Repeated scans |
| Scheduler runs cross-history event scans every 5 s even for disabled trigger types | `server/scheduler.go:117` | Wasted scans |
| Side-question SSE re-reads the full answer row four times per second per open panel | `server/side_questions.go:566` | Repeated row reads |

### 2.3 UI/UX

The foundation is good: oklch tokens with light/dark pairs, shadcn primitives with focus-visible rings, a centralized status tone map (`web/src/lib/status.ts`), 128 tabular-num usages, tooltips in the collapsed sidebar, and consistent chart tooltips. The problems concentrate on the dashboard:

- It renders zeros and empty copy before data arrives and swallows fetch errors (`dashboard/page.tsx:136`).
- The token chart says "No data" while the same card totals 2.9 M tokens (`dashboard/page.tsx:821`).
- Type is routinely 9–10 px: 97 ten-pixel and 19 nine-pixel labels (`dashboard/page.tsx:1013`).
- Light mode uses dark-mode status shades, failing contrast (`dashboard/page.tsx:525`).
- The six-column task table clips instead of scrolling (`dashboard/page.tsx:1000`).
- Header utility buttons use solid primary fill and outrank page content (`layout-controls.tsx:102`).
- Icon-only controls lack accessible names and pressed state (`layout-controls.tsx:102`).
- No reduced-motion handling anywhere.
- Empty states are hand-rolled in five styles while `components/ui/empty.tsx` sits unused.

---

## 3. Brand system

### 3.1 The mark

A geometric **H** built from two vertical bars and a shared crossbar. The bars are cut at 45° on opposing corners so the mark reads as forward motion without becoming a literal arrow. The crossbar is slightly thicker than the bars to hold the shape at 16 px. Gradient runs indigo → violet (`#4F46E5` → `#A855F7`), matching the existing identity's blue→violet bridge.

Requirements:

- Reads at 16 px (favicon), 24 px (collapsed sidebar), 40 px (expanded sidebar), and 160 px (auth panel).
- No thin strokes below 2 px at 16 px; no detail that disappears at small sizes.
- Clear space equal to the crossbar thickness on all sides.
- Variants: full-colour gradient (transparent), monochrome white/`currentColor`, and a dark-surface lockup.
- The old infinity/A artwork is retired, not kept alongside. One master vector, no divergent variants.

### 3.2 Wordmark and lockup

- Wordmark: `HARTEX`, uppercase, Geist Sans 600, letter-spacing `0.16em`, optically aligned to the mark's crossbar.
- Lockup: mark + wordmark, used on the sidebar header and auth panel.
- The mark alone is used in the collapsed sidebar, favicon, app icon, and PWA icons.
- Do not render the wordmark below 10 px; below that, use the mark alone.

### 3.3 Asset plan

| Asset | Purpose | Format / size | Notes |
|---|---|---|---|
| `web/assets/brand/hartex-mark.svg` | master vector | SVG, transparent | single source of truth; not served |
| `web/public/brand/hartex-mark.svg` | colour mark | SVG | served; used at 40/160 px |
| `web/public/brand/hartex-mark-mono.svg` | white/`currentColor` mark | SVG | auth panel, dark surfaces |
| `web/public/brand/hartex-lockup.svg` | mark + wordmark | SVG | sidebar expanded, auth |
| `web/src/app/icon.png` | Next app icon | 512×512 PNG | generated from the mark; replaces the 492 KB 1920 px file |
| `web/src/app/apple-icon.png` | iOS home screen | 180×180 PNG | new |
| `web/src/app/favicon.ico` | favicon | 16/32/48 ICO | regenerate from the mark |
| `web/src/app/opengraph-image.png` | social preview | 1200×630 PNG | new; dark background, mark, wordmark, tagline |
| `web/src/app/manifest.ts` | PWA manifest | TS | name/short_name/icons 192/512, theme/background colors |
| `server/webui/dist/**` | embedded copy | generated | build step copies from `web/out`; never hand-edited |

The 492 KB `logo.png`, the byte-identical `icon.png`, and the unused `logo.svg` are removed from the served tree. The old master is kept only if the owner wants an archive; otherwise delete it.

**Generation:** write the SVG master, render the PNG/ICO set once with an available vector renderer (`rsvg-convert`, ImageMagick, or headless Chromium), commit the outputs, and add a checksum/size check to CI. No runtime dependency is added.

### 3.4 Visible-name surfaces

| File | Change |
|---|---|
| `web/src/config/app-config.ts` | `name: "Hartex"`; title `"Hartex: Autonomous Penetration Testing Console"`; copyright `© {year}, Hartex.` |
| `web/messages/en.json` | product-name strings → Hartex; keep legal text accurate |
| `web/src/app/(auth)/login/page.tsx` | logo/lockup, `alt`, welcome copy; account display per §12 |
| `web/src/app/(auth)/setup/page.tsx` | logo/lockup and `alt` |
| `web/src/app/(main)/_components/sidebar/app-sidebar.tsx` | swap image for lockup; name from `APP_CONFIG` |
| `web/src/app/layout.tsx` | metadata icons, OpenGraph, Twitter, manifest |
| `cmd/artex/main.go` | startup banner → Hartex; keep version/runtime line |
| `web/src/lib/api.ts` | report template title → Hartex |
| `web/src/hooks/use-current-user.ts`, `web/src/lib/auth.ts` | visible fallback names → Hartex |
| `web/src/app/(main)/system/logs/page.tsx`, `system/notify/*`, `settings/update-card.tsx`, `chat/page.tsx`, `tasks/detail/_tabs/findings-tab.tsx` | visible example/fallback strings → Hartex |
| `README.md`, `CONTRIBUTING.md`, `MAINTAINING.md`, `SECURITY.md`, `CODE_OF_CONDUCT.md` | visible product name → Hartex |
| `docs/defense-en.md`, `docs/finding-traffic-evidence-en.md`, `screenshots/README.md` | product name → Hartex |
| `install.sh`, `start.sh`, `start.bat`, `start-cloud.sh`, `update.sh`, `reset-password.sh` | user-facing messages → Hartex |
| `server/notify_api.go`, `selfupdate/stage.go`, CLI help/flags in `cmd/artex/main.go` | backend and CLI text a person actually sees (notification test message, update progress/errors, `-h`) → Hartex |
| `web/src/lib/mock/data.ts` | demo-mode strings shown in the UI → Hartex; the `NEXT_PUBLIC_MOCK=1` path must keep working |
| `CHANGELOG.md` | add a new entry; do not rewrite history |

### 3.5 Protected surfaces — do not change

- **Internal identifiers:** Go module path, `artex` binary/service, `ARTEX_*` env vars, `artex-web` package name, database/config names, log keys, and code comments.
- **Backend-emitted user-facing text is in scope.** The distinction is whether a person sees it: notification test messages, update progress/errors, CLI help and startup output, and demo-mode content all change; internal key names and comments do not.
- **The built-in login credential:** the backend authenticates a fixed operator account; display treatment is in §12, but the credential itself is not renamed without a coordinated auth/DB migration.
- **Upstream attribution:** every "upstream ARTEX" / "Autumn-27/ARTEX" / "English-only global edition" reference in README and legal text stays, because it is both a license fact and an accuracy fact.
- **Detection content:** `detections/**` intentionally names the upstream ARTEX threat (Sigma rules, Suricata rules, indicator files, detection tests). Renaming these would break the defensive material.
- **Historical records:** prior `docs/superpowers/specs/` and old `CHANGELOG.md` entries.
- **Repository/image names:** already `hartex`; no change needed.

---

## 4. Design system

### 4.1 Principles

1. **Dark is the instrument panel.** Near-black layered surfaces, restrained contrast steps, one electric accent. Light is a daylight mode, not an inverted copy.
2. **Neon signals state.** Glow marks live data, active navigation, focus, and the primary action. It never decorates body text or every card.
3. **Density with air.** The console is data-dense; keep 4/8 px rhythm, 12–16 px between groups, and 48 px table rows. Density comes from information, not from 9 px type.
4. **One primary action per surface.** Everything else is secondary or ghost.
5. **Color is never the only signal.** Pair every status colour with a label, icon, or shape.

### 4.2 Colour tokens

Dark (default). Values are oklch, matching the existing token style.

| Token | Value | Use |
|---|---|---|
| `--background` | `oklch(0.13 0.018 285)` | app canvas |
| `--foreground` | `oklch(0.96 0.005 285)` | body text |
| `--card` | `oklch(0.17 0.02 285)` | panels |
| `--card-foreground` | `oklch(0.96 0.005 285)` | panel text |
| `--popover` | `oklch(0.19 0.022 285)` | menus, sheets |
| `--primary` | `oklch(0.72 0.22 295)` | neon violet: primary action, active nav, focus |
| `--primary-foreground` | `oklch(0.14 0.02 285)` | text on violet |
| `--secondary` | `oklch(0.24 0.025 285)` | secondary controls |
| `--muted` | `oklch(0.22 0.02 285)` | quiet surfaces |
| `--muted-foreground` | `oklch(0.72 0.02 285)` | metadata |
| `--accent` | `oklch(0.82 0.15 205)` | electric cyan: live data, secondary emphasis |
| `--accent-foreground` | `oklch(0.14 0.02 285)` | text on cyan |
| `--destructive` | `oklch(0.68 0.24 25)` | destructive/severity high |
| `--border` | `oklch(1 0 0 / 10%)` | 1 px separators |
| `--input` | `oklch(1 0 0 / 14%)` | field borders |
| `--ring` | `oklch(0.72 0.22 295)` | focus ring |
| `--sidebar` | `oklch(0.15 0.02 285)` | rail |
| `--sidebar-primary` | `oklch(0.72 0.22 295)` | active item |

Supporting tokens:

```css
--surface-2: oklch(0.19 0.022 285);
--surface-3: oklch(0.22 0.025 285);
--glow-primary: 0 0 0 1px oklch(0.72 0.22 295 / 0.35),
                0 0 24px oklch(0.72 0.22 295 / 0.18);
--glow-accent:  0 0 0 1px oklch(0.82 0.15 205 / 0.30),
                0 0 20px oklch(0.82 0.15 205 / 0.14);
--grid-line: oklch(1 0 0 / 6%);
```

Charts: `--chart-1` violet, `--chart-2` cyan, `--chart-3` mint, `--chart-4` amber, `--chart-5` coral. Grid lines use `--grid-line`; data lines must sit at ≥3:1 against the card.

Status tones (dark): success `oklch(0.82 0.18 160)`, warning `oklch(0.84 0.16 85)`, danger `oklch(0.70 0.23 25)`, info `oklch(0.82 0.15 210)`, neutral `oklch(0.72 0.02 285)`.

Light (secondary). The neon accent becomes a deeper, accessible violet/cyan; glow is removed.

| Token | Value |
|---|---|
| `--background` | `oklch(0.98 0.005 285)` |
| `--foreground` | `oklch(0.18 0.02 285)` |
| `--card` | `oklch(1 0 0)` |
| `--primary` | `oklch(0.52 0.22 295)` |
| `--primary-foreground` | `oklch(0.99 0 0)` |
| `--accent` | `oklch(0.50 0.13 210)` |
| `--muted` | `oklch(0.96 0.008 285)` |
| `--muted-foreground` | `oklch(0.48 0.02 285)` |
| `--border` | `oklch(0.89 0.01 285)` |
| `--ring` | `oklch(0.52 0.22 295)` |

Status tones (light): success `oklch(0.52 0.16 160)`, warning `oklch(0.55 0.14 75)`, danger `oklch(0.55 0.20 25)`, info `oklch(0.52 0.12 220)`.

**Rule:** every raw Tailwind colour utility used for status text must be replaced by a semantic token or the existing `toneClasses` map in `web/src/lib/status.ts`. No `text-blue-400` in light mode; no dark-mode shade on a light surface.

**Legacy presets.** Brutalist, Soft Pop, and Tangerine stay available and are not redesigned. They must define the new supporting tokens (`--surface-2/3`, `--grid-line`, chart/status values) with values consistent with their own palette, and set `--glow-primary`/`--glow-accent` to `none`, so the new components do not leak Hartex violet/glow into a legacy theme. The default preset is Hartex.

### 4.3 Typography

Keep Geist Sans and Geist Mono — both are already loaded, so the redesign adds no font weight. The pixel face is optional and stays off the default path.

| Token | Size / line | Weight | Use |
|---|---|---|---|
| `--text-page` | 20 px / 1.2 | 600 | page titles |
| `--text-section` | 14 px / 1.3 | 600 | section headers |
| `--text-body` | 13 px / 1.5 | 400 | body |
| `--text-meta` | 12 px / 1.4 | 400 | secondary metadata |
| `--text-micro` | 11 px / 1.3 | 500 | uppercase labels, badges |

Rules:

- Minimum 11 px for any label; no 9 px or 10 px text.
- Uppercase micro-labels get `letter-spacing: 0.08em`.
- Headings use `letter-spacing: -0.01em`.
- All metrics, IDs, timestamps, token counts, and table numerics use `font-variant-numeric: tabular-nums`; IDs and code use Geist Mono.
- Line length for prose stays 60–75 characters.

### 4.4 Surfaces, elevation, radius

- Radius: `10px` cards, `8px` controls, `6px` badges, full pills for status.
- Table rows: `40px` desktop, `44px` on touch; header rows `36px` with 11 px uppercase labels.
- Dark elevation: 1 px border + `inset 0 1px 0 oklch(1 0 0 / 4%)`; shadows only for sheets/popovers.
- Light elevation: `shadow-sm` on cards, no glow.
- Background: one very subtle radial wash (violet 4% top-left, cyan 3% top-right), CSS-only, no image.
- Do not animate box-shadow or width/height.

### 4.5 Motion

```css
--dur-fast: 120ms;
--dur-base: 180ms;
--dur-slow: 280ms;
--ease-standard: cubic-bezier(0.2, 0.8, 0.2, 1);
```

- Hover/press transitions on colour, opacity, and transform only.
- Page enter: 8 px translate + fade, 180 ms; no stagger on dense tables.
- Live dot: pulse only when data is live; static dot + "live" label under `prefers-reduced-motion`.
- Global rule: `@media (prefers-reduced-motion: reduce) { *, *::before, *::after { animation-duration: 0.01ms !important; transition-duration: 0.01ms !important; } }` plus `motion-reduce:animate-none` on the live dot.
- Charts do not animate entrance by default; data is readable immediately.

### 4.6 Shared components to extract

| Component | Responsibility |
|---|---|
| `PageHeader` | h1 (20 px), subtitle (13 px muted), right-side actions; used by all routes |
| `StatCard` | micro label, 28 px tabular value, context line, optional sparkline, optional accent |
| `SectionCard` | 14 px header, 16 px icon, action link, content slot |
| `DataTable` | `overflow-x-auto` wrapper, `min-w`, sticky header, 40 px rows (44 px on touch), `aria-sort`, empty slot |
| `StatusBadge` | existing `toneClasses` + dot + text; no colour-only meaning |
| `EmptyState` | `components/ui/empty` with icon, title, one-line explanation, optional action |
| `FilterBar` | search, selects, reset; wraps to a sheet on mobile |
| `ChartFrame` | title, scope label, legend, loading skeleton, empty state, error retry |
| `ProfileCard` | LLM profile: name, model (mono), health dot + label, actions |
| `LiveDot` | pulse with reduced-motion fallback |

### 4.7 Accessibility contract

- Normal text ≥ 4.5:1 in both themes; large text ≥ 3:1; non-text controls ≥ 3:1.
- Visible `focus-visible` ring (2 px primary, 2 px offset) on every interactive element.
- Icon-only buttons get `aria-label`; toggles get `aria-pressed`; sortable headers get `aria-sort`.
- No colour-only status; severity always has a label or icon.
- Tables and charts get a text summary or accessible name.
- Reduced motion respected globally.
- Touch targets ≥ 44 px on mobile; 32 px controls on desktop with an enlarged hit area where needed.

---

## 5. Screen designs

### 5.1 Shell

- **Sidebar:** a 240 px expanded rail (down from the current 272 px), Hartex lockup at the top; Features/System groups with 11 px uppercase labels; active item gets a violet left indicator, a subtle violet fill, and the label in full foreground; inactive icons are muted. Collapsed mode shows the mark and tooltips. The rail is `--sidebar` with a 1 px border.
- **Header:** 48 px, sticky, translucent background with backdrop blur; sidebar trigger, command search with `⌘K`, version badge, update badge, layout controls, theme switcher, account. Utility buttons become `ghost`; the solid primary fill is reserved for real page actions.
- **Page container:** `max-w-screen-2xl`, 24 px desktop / 16 px mobile padding; `PageHeader` on every route; no horizontal page scroll.
- **Mobile:** sidebar becomes a sheet; filters collapse into a sheet; tables scroll inside their own container.

### 5.2 Dashboard

Layout top to bottom:

1. **PageHeader:** "Overview" + subtitle; live/stale indicator on the right.
2. **KPI row (5 cards):** Active tasks (accent card), Confirmed findings, Asset nodes, Traffic exchanges, Token usage (with sparkline). Each card has a micro label, 28 px tabular value, and a context line. Loading uses skeletons with the same dimensions; errors show an inline alert with Retry, not zeros.
3. **Token consumption panel:** 200 px chart, 30-day default, range buttons 7/30/90/180/365. Stacked violet/cyan/mint bars. Legend is clickable. The scope label always matches the total in the KPI card. Empty state: "No usage in the last 30 days from the activity source" + "All time" action. Error state: retry.
4. **Activity stream + Findings panel:** two columns on desktop, stacked on mobile. Worker badges: planner violet, mainagent cyan, other blue; timestamps mono/tabular; live dot only when streaming.
5. **Task table:** wrapped in `DataTable` with horizontal scroll and `min-w-[52rem]`; Engine and In-flight hide below `md`; row hover; status badges; goal progress with violet fill.

Remove all 9–10 px type. Keep the existing polling cadence but gate it on visibility.

### 5.3 Tasks

- **PageHeader + toolbar:** search, status/category filters, view controls, "New task" primary button. Toolbar is sticky under the header on desktop; filters collapse to a sheet on mobile.
- **List:** the existing table structure and drag/reorder behavior stay. Rows are 52 px, task name 13 px medium, description 12 px muted, target 12 px mono, IDs mono. Status uses `StatusBadge`; progress uses a 4 px violet bar; favorites/pins use a clear icon state. Bulk selection reveals a floating action bar; destructive actions are separated and confirmed.
- **Empty state:** `Empty` with "No tasks yet" + "New task".
- **Task detail:** keep the page's own header and tabs. Convert non-default tabs (graph, coverage, traffic, findings, etc.) to `next/dynamic` so they load on demand. The graph tab uses neon edges on dark with a muted grid. Preserve all current behavior.
- **Mobile:** table scrolls; bulk actions become a sheet.

### 5.4 Findings

- **PageHeader:** total count, severity distribution as a segmented bar (critical/high/medium/low with labels), export CSV action.
- **FilterBar:** severity, status, vulnclass, task, search; reset. View tabs: Flat / Grouped / Asset.
- **Table:** severity badge with label, title, asset, task, time, status. Sticky header, 48 px rows, sortable headers with `aria-sort`, row expansion for details. Pagination at the bottom. Horizontal scroll inside the table container.
- **Detail:** the existing drawer/detail page gets the new tokens, a summary block, evidence viewer, retest panel, and traffic panel. Keep the current retest polling but gate it on visibility.
- **Empty state:** "No findings match these filters" + "Clear filters"; no finding at all → "No findings yet" + link to Tasks.

### 5.5 System → LLM

- **PageHeader:** "LLM profiles" + pool status summary (healthy / degraded / tripped) + "Pool" and "New profile" actions.
- **Profile grid:** one card per profile. Card shows name (14 px), model (12 px mono), base URL host (12 px mono), key status, health dot + label, priority, and actions (test, edit, delete). The active profile gets a violet border + subtle glow; unhealthy gets a danger border + text. Cards are keyboard-focusable and have an accessible name.
- **Editor sheet:** grouped fields (Identity, Endpoint, Rate limits, Behaviour, Retry). Labels are visible; helpers sit below; errors are inline and announced; API key has a show/hide toggle. Save is the only primary action; Test connection is outline.
- **Pool sheet:** chain order, active/tripped/excluded states, cooldown countdown, recover one/all. Poll only while the sheet is open and something is unhealthy.
- **Empty state:** "No LLM profiles yet" + "Add profile".

### 5.6 Login and setup

- Left panel (desktop): dark near-black with a subtle violet/cyan radial wash and a faint grid; Hartex lockup at 160 px, tagline, and a one-line product description. No `brightness-0 invert` filter — use the mono/colour variant intentionally.
- Right panel: centered form card (max 420 px), 20 px title, 13 px helper, visible labels, password show/hide, inline error, loading state on submit, terms checkbox.
- Setup mirrors the same shell with the first-run copy.
- Mobile: single column, mark at 64 px, form full width.
- The read-only account field is treated as an identifier, not a brand surface (see §12).

### 5.7 Everything else

Traffic, Chat, Tool runs, LLM records, Assets, Sync, Workspace, Agents, MCP, Skills, Tools, Notifications, Intercept, Approvals, Logs, Settings inherit the token system, `PageHeader`, `DataTable`, `EmptyState`, and focus/motion rules. They are not individually redesigned, but they must not regress: no 9 px type, no clipped tables, no colour-only status, no missing empty/error states.

---

## 6. Performance work

### 6.1 Phase A — delivery and assets (independent, highest leverage)

| # | Change | Files | Acceptance |
|---|---|---|---|
| A1 | Add gzip/brotli middleware to the embedded UI; set `Vary: Accept-Encoding`, compress only text types, skip `woff2/png/ico/zip`; add `Cache-Control: public, max-age=31536000, immutable` for `/_next/static/*`; `no-cache` for HTML. Because `embed.FS` has zero `ModTime` and no ETag, add a content-hash ETag/`Last-Modified` for non-hashed assets so revalidation actually works | `server/webui_embed.go`, new middleware + tests | Dashboard first-load transfer ≤ 600 KB gzip; repeat visit revalidates only HTML and non-hashed assets; Range/HEAD still work |
| A2 | Replace the 1920 px logo/icon with the Hartex SVG + sized PNG/ICO; add apple icon, OG image, manifest | `web/assets/brand`, `web/public/brand`, `web/src/app`, `web/src/app/manifest.ts` | Mark ≤ 20 KB per rendered size; no 1920 px raster in `web/out` |
| A3 | Preload only the default Geist Sans + Mono; `preload: false` for optional faces; remove the barrel import | `web/src/lib/fonts/registry.ts` | ≤ 2 preloaded faces on the default path |
| A4 | Load the mock handler only when `NEXT_PUBLIC_MOCK === "1"`; alias to an empty stub in production | `web/src/lib/api.ts`, `next.config.*` | Mock chunk absent from production HTML |
| A5 | Reduce the client catalog payload: pass only the namespaces a route uses, or keep English messages server-side | `web/src/i18n/intl-runtime-provider.tsx`, `web/src/app/layout.tsx` | Catalog chunk ≤ 15 KB gzip on the dashboard, or absent |
| A6 | One visibility-aware polling hook; pause on `document.hidden`; consolidate duplicate intercept-pending; the sessions tab's unconditional 1.5 s tick runs only while a stream is live | new `web/src/hooks/use-polling.ts`, dashboard/tasks/findings/nav-main, sessions tab | A hidden tab drops from ~10 requests per 5 s across 28 intervals to near zero; one intercept request per interval |

### 6.2 Phase B — frontend structure

| # | Change | Files | Acceptance |
|---|---|---|---|
| B1 | Dashboard: skeletons/error retry, chart empty-state fix, dynamic Recharts card (the chart chunk is 104,104 B gzip), table scroll, type scale, status tokens; replace the 500 ms width animation on token bars with `scaleX` | `web/src/app/(main)/dashboard/page.tsx` | No zeros before load; no "No data" beside a non-zero total; table scrolls at 768 px; chart chunk absent from first paint |
| B2 | Lazy-load non-default task-detail tabs and the graph; shared async chunk for XYFlow (first paint is currently 2,204 KB raw / 674 KB gzip across 28 chunks; `@xyflow/react` is duplicated at 187,572 B) | `web/src/app/(main)/function/tasks/detail/page.tsx`, `web/src/components/exploration-graph.tsx` | Hidden tab chunks absent from first paint; one shared XYFlow chunk |
| B3 | Findings: header distribution, table scroll, empty states, tokens | `web/src/app/(main)/function/findings/**` | No clipped table; clear empty/error states |
| B4 | LLM profiles: cards, pool sheet, form accessibility | `web/src/app/(main)/system/llm/**` | Every control labeled; active/unhealthy states readable |
| B5 | Shared `PageHeader`, `StatCard`, `DataTable`, `EmptyState`, `ChartFrame`; global reduced-motion | `web/src/components/**`, `web/src/app/globals.css` | Components used by the four flagships; reduced-motion verified |
| B6 | Remove dead dependencies and the inlined `package.json` in the client (it costs 2,510 B in a 65,634 B chunk served on ~20 routes) | `web/package.json`, `web/src/config/app-config.ts`, `web/src/components/calendar/**` | Build passes; no unused dependency warnings; version comes from a build-time constant |

### 6.3 Phase C — backend hot paths

| # | Change | Files | Acceptance |
|---|---|---|---|
| C1 | Replace `ListByKind` counts in `/api/stats` with `COUNT` aggregates | `server/server.go`, `db/exploration.go` | Same JSON fields; query does not materialize rows |
| C2 | Dashboard findings: use the existing stats endpoint for counts and a paged call for the visible list; omit evidence from list responses | `server/server.go`, `db/findings.go`, `web/src/lib/api.ts` | Poll payload bounded; UI unchanged |
| C3 | Evidence GC: snapshot hashes and walk outside the advisory lock; delete in bounded batches | `evidence/store.go` | Lock held only for short metadata transactions |
| C4 | Load the exploration graph once per planner round; project only needed columns | `agent/compaction.go`, `agent/planner.go`, `db/exploration.go` | One graph load per round; same planner inputs |
| C5 | Traffic capture: hash/write blob before `wmu`; hold the lock only for the SQLite transaction | `traffic/traffic.go` | No capture serialization behind file I/O; ordering preserved |
| C6 | Batch cold-stamp updates into one or two statements | `db/digest.go` | Same round stamps; fewer round trips |
| C7 | Conversation token totals: partial index or maintained totals; slow the dashboard poll | `db/conversation.go`, `server/conversations.go` | Dashboard total unchanged; no full-history scan per poll |
| C8 | Traffic page: avoid duplicate `COUNT`, cache summary counts, make the host filter index-friendly | `traffic/traffic.go` | Same counts; no full-table scan per poll |
| C9 | Scheduler: read state once per tick; skip disabled trigger types with cheap watermarks | `server/scheduler.go` | Same triggers; no scans for disabled types |
| C10 | Side-question SSE: poll a light projection (sequence/status), fetch the full row only on change | `server/side_questions.go` | Same SSE payload; fewer full-row reads |

Each backend change preserves the existing API shape and is covered by the existing tests plus one focused test for the new query path. Changes are independent; they can land in any order, but C1–C2 first because they feed the dashboard.

---

## 7. Implementation order

1. **Baseline and harness.** Record current bundle sizes and page-weight numbers; run existing Go and web tests; capture screenshots at 1440/1024/375 in light and dark.
2. **Tokens and shell.** New dark/light tokens, `theme_mode` default `dark`, glow/surface/motion tokens, reduced-motion rule, `PageHeader`, sidebar lockup, header cleanup. Everything else inherits immediately.
3. **Brand assets and surfaces.** Master SVG, generated icons, sidebar/auth/login/setup, metadata/OG/PWA, config and visible-string pass.
4. **Four flagships.** Dashboard, Tasks, Findings, LLM profiles against the new components.
5. **Phase A delivery.** Compression/caching, mock/i18n payload, font preload, polling hook.
6. **Phase B structure.** Dashboard/tasks/findings/LLM lazy loading and state handling.
7. **Phase C backend.** The ten hot-path fixes, in the order above.
8. **Docs and verification.** README/CONTRIBUTING/MAINTAINING/SECURITY/docs/scripts; screenshot refresh; full test/build/measure pass; `graphify update .`.

---

## 8. Verification

### Automated

- `go test ./...` and `go vet ./...`.
- New Go tests: compression negotiation, immutable cache headers, `/api/stats` aggregate shape, findings pagination, evidence GC ordering, planner single-load, traffic lock scope.
- `cd web && npm run check` (Biome), `npm run test` (Vitest), `npm run build:static`.
- Compare `web/out` asset sizes before/after; assert the mock/catalog chunks are absent or below thresholds.
- Rebrand gate: a grep/CI check that user-visible `ARTEX` appears only in the allowlist (upstream attribution, internal identifiers, detections, historical docs).
- `graphify update .` after code changes.

### Manual

- Run the app (Docker Compose or `./dev.sh`) and screenshot Dashboard, Tasks, Findings, LLM profiles, Login, Setup in dark and light at 1440, 1024, and 375 px.
- Keyboard-only pass: sidebar, command search, filters, table sort, sheets, forms.
- Contrast spot checks for body, muted, primary, status, and chart lines in both themes.
- `prefers-reduced-motion: reduce` pass: live dots static, no page/stagger animation.
- Measure with `curl --compressed -I` / `-o /dev/null -w` for `Content-Encoding`, `Cache-Control`, and transfer size.
- Empty/loading/error pass on the four flagships by blocking API responses.

### Acceptance

- No user-visible ARTEX outside the allowlist.
- Dashboard first-load transfer ≤ 600 KB gzip before the asset/bundle work, ≤ 500 KB after; logo ≤ 20 KB per rendered size; ≤ 2 font preloads.
- No 9 px/10 px text on the four flagships or the shell.
- No clipped table at 768 px; no horizontal page scroll at 375 px.
- Both themes pass the contrast and focus checks.
- Existing behavior and tests unchanged.

---

## 9. Risks and mitigations

| Risk | Mitigation |
|---|---|
| Compression middleware breaks static export, Range, or HEAD | Unit-test negotiation and byte ranges; skip already-compressed types; keep `Content-Length` correct |
| Neon accent fails contrast or looks pastel in light mode | Accent is for fills, borders, and live state; light mode uses deeper tokens; measure every pair |
| New mark loses legibility at 16 px | Design at 16 px first; test favicon and collapsed sidebar before committing |
| Rebrand search/replace damages upstream attribution or detection signatures | Explicit allowlist and protected-surface list; review diffs file by file |
| i18n payload split drops a message | Build every route in CI; compare rendered strings; add a missing-message test |
| Backend query changes alter semantics | Keep JSON shapes; add focused tests; land independently; no implicit limits |
| Theme presets clash with the new identity | Keep them as legacy options or hide them (see §12); default is Hartex |
| Static export limits dynamic imports | Use `next/dynamic` with `ssr: false`; verify `NEXT_EXPORT=1 next build` |

---

## 10. What is already good and must be preserved

- The oklch token/preset architecture and light/dark pairing.
- The centralized `toneClasses` status semantics in `web/src/lib/status.ts`.
- shadcn primitives and their focus-visible rings.
- The tabular-numeral habit (128 usages) and consistent chart tooltips.
- The sidebar's collapsed-mode tooltips and the existing command search.
- The findings page's loading gate and its `min-w-[60rem]` table inside an `overflow-x-auto` wrapper — copy this pattern instead of inventing a new one.
- The coverage tab's `await import("@antv/g6")` — the deferred-loading pattern already exists; extend it to the other tabs.
- The three views (flat/grouped/asset) and their pagination model.
- The tasks page's drag/reorder, archives, templates, and bulk delete behavior.
- The LLM page's pool chain, health states, and retry policy fields.
- The English-only edition's upstream-attribution and legal notices.

---

## 11. Explicitly deferred

- Removing the three legacy theme presets (decide in §12).
- Rewriting the English-only i18n layer out of the client entirely (A5 is the measured step; a full removal is a separate project).
- Replacing AntV G6/Graphin/XYFlow or any chart library.
- Renaming the Go module, binary, service, database, or login credential.
- Rewriting detection content or historical changelog entries.
- A marketing site, landing page, or docs theme.

---

## 12. Settled calls and resolved scope

### Settled calls

- **Theme presets stay.** Brutalist, Soft Pop, and Tangerine remain available with their own palettes and no Hartex glow; they get the new supporting tokens so the new components do not hybridize. The default is Hartex.
- **Login account display.** The login page shows "Operator account" with a helper line; the internal credential and JWT subject stay `ARTEX`, so existing sessions keep working. The page's brand identity comes from the Hartex lockup, not from the account string.
- **Docs scope.** Current user-facing docs and scripts change: README, CONTRIBUTING, MAINTAINING, SECURITY, CODE_OF_CONDUCT, `docs/defense-en.md`, `docs/finding-traffic-evidence-en.md`, `screenshots/README.md`, and the install/start/update/reset scripts. `CHANGELOG.md` history, prior `docs/superpowers/` specs, `detections/**`, and upstream-ARTEX attribution stay intact.
- **Wordmark casing.** `HARTEX`, uppercase.
- **Backend, CLI, and demo text.** User-facing strings the backend emits (notification test, update progress/errors), CLI help/startup output, and mock/demo content are in scope; internal identifiers, log keys, and comments are not.
- **Default theme.** Dark primary, light secondary; no removal of the theme switcher.
- **Internal identifiers.** Go module, binary, service, database, env vars, and package name stay `artex`.

### Scope beyond the flagships — resolved

**Builder call: (a), flagships first.** Dashboard, Tasks, Findings, LLM profiles, the shell, login, and setup got the full treatment. Every other route inherits the new tokens automatically and received targeted fixes only for obvious breakage (type floor, status contrast, table clipping, empty/error states). A full `PageHeader`/empty-state retrofit of all 24 routes was not part of this pass, consistent with the owner naming the four flagships and saying the direction was all they needed.

---

## 13. What is left to the builder

- Choose the exact SVG path geometry for the H and render the icon set; the constraints are in §3.1.
- Choose the final chart palette ordering from the tokens in §4.2.
- Decide which non-default task-detail tabs are lazy-loaded first (all of them is the goal).
- Choose the gzip implementation detail (standard `compress/gzip` vs `klauspost/compress`) — both are acceptable; the acceptance is transfer size and correctness.
- Decide whether the old logo files are archived or deleted once the new assets are in use.
- Pick the exact page-level skeleton breakpoints; the components and states are specified.
- Keep the current 5-second dashboard/findings cadence unless the owner asks otherwise; the fix is to stop polling when hidden, not to slow the live experience.
- Lazy-load the mock path rather than compiling it out, so `NEXT_PUBLIC_MOCK=1` still works for demos.
- Keep narrow-screen support functional at 375 px and 768 px (sidebar sheet, table scroll, filter sheet); the four flagships are desktop-first but must not break on a laptop or phone.
