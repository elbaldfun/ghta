# Handoff: GitHub 高星仓库排名 / 搜索网站 (High-Star Repo Ranking & Search)

## Overview
A website for browsing, filtering, and searching high-star GitHub repositories. Users browse a card grid of repos, filter by a two-level category taxonomy (sidebar tree), refine by language / license / topic, sort by stars or recency, and open a detail page for any repo. The detail page shows stats, a star-growth chart, package/artifact install info, a multi-language README (collapsible), and horizontally-scrollable carousels of related repos and related articles. A blog/article reading view is also included.

The selected/approved direction is **2a — Tree Nav + Card Grid** (see the badge navigation in the file). Earlier explorations (1a terminal, 1b dev-tool, 1c dashboard) are kept in the same file for reference but should NOT be built.

## About the Design Files
The file in this bundle (`GitHub高星仓库排名网站.dc.html`) is a **design reference created in HTML** — a prototype showing intended look and behavior, not production code to copy directly. It is authored as a "Design Component" with a small custom template runtime (`<x-dc>`, `{{ }}` holes, `<sc-for>`/`<sc-if>`, a `class Component`), which is **specific to the prototyping tool and should not be reproduced**.

The task is to **recreate the 2a design in your target codebase** (React, Vue, Svelte, etc.) using its established patterns, component library, and data layer. If no frontend exists yet, pick the most appropriate framework and implement there. Wire the UI to a real GitHub data source (GitHub REST/GraphQL API or your own indexed dataset) — the prototype uses a hardcoded in-file `REPOS` array as placeholder data.

## Fidelity
**High-fidelity (hifi).** Colors, typography, spacing, and interactions are final. Recreate the 2a UI pixel-accurately using your codebase's libraries. Exact tokens are listed below.

## Design Tokens

### Fonts (Google Fonts)
- **Sora** — headings / display (weights 400–800)
- **Manrope** — UI and body text (weights 400–800); this is the default `font-family`
- **IBM Plex Mono** — code, install commands, monospace values (weights 400–700)

### Theme system
The prototype ships **4 palettes** (`a`,`b`,`c`,`d`), each with a **light** and **dark** variant, toggled at runtime. Ship at minimum palette **a** (GitHub-contributions green) in light + dark. Each theme is a set of 9 role tokens: `bg, panel, panel2, border, text, muted, accent, accent2, contrastOn`.

Palette A (recommended default):
- dark:  bg `#0a0e0c` · panel `#101512` · panel2 `#141a16` · border `#1f2b23` · text `#d6e8d9` · muted `#7c8f80` · accent `#39d353` · accent2 `#ffb454` · contrastOn `#04140a`
- light: bg `#f4f7f4` · panel `#ffffff` · panel2 `#eef3ee` · border `#d3e0d6` · text `#132016` · muted `#5c6d60` · accent `#1e8e3e` · accent2 `#a35c00` · contrastOn `#ffffff`

Palette B (indigo): dark accent `#a89bff`/`#6d5efc`; light accent `#4f46e5`/`#6d5efc`.
Palette C (cyan/pink): dark accent `#22d3ee`/`#f472b6`; light accent `#0891b2`/`#db2777`.
Palette D (blue/green): dark accent `#6d8bff`/`#34d399`; light accent `#4f5bd5`/`#0d9f6e`.
(Full 9-token sets for B/C/D are in the file's `THEMES` object — copy verbatim if implementing the theme switcher.)

Token roles:
- `bg` — page background · `panel` — card/surface background · `panel2` — subtle inset (search bar, chips, table headers)
- `border` — 1px borders (`1px solid border`) · `text` — primary text · `muted` — secondary text, labels, icons
- `accent` — primary brand (active states, stars, links, primary numbers) · `accent2` — secondary highlight (delta "▲", accents)
- `contrastOn` — text/icon color placed on an `accent` fill

### Language dot colors
Repo language chips use canonical GitHub language colors (`LANG_COLORS` map in file): JavaScript `#f1c40f`, TypeScript `#3178c6`, Python `#3572A5`, Go `#00ADD8`, Rust `#dea584`, etc.

### Radii / shadows / spacing
- Radius: cards `12px`, panels/stat boxes `12px`, section container `16px`, small buttons `8px`, chips/pills `999px`, README lang buttons `6px`.
- Card hover: `box-shadow: 0 8px 22px rgba(20,20,40,0.12)` + `border-color: accent`.
- Section outer shadow: `0 30px 70px rgba(20,20,40,0.18)`.
- Floating carousel arrow shadow: `0 4px 14px rgba(20,20,40,0.16)`.
- Grid gap `14px`; card padding `16px`; section header horizontal padding `~26–28px`.

## Screens / Views

### 1. Header (persistent, all views)
- **Layout**: flex row, space-between, padding `20px 28px`, bottom `1px solid border`.
- **Left**: logo/brand + primary nav buttons (`background` active = accent, radius `8px`, padding `7px 13px`, 12.5px/700).
- **Center**: search input in a pill — `panel2` bg, `border`, radius `999px`, padding `9px 16px`, max-width `460px`, search icon (muted stroke) + transparent-bg input. Placeholder text per UI language.
- **Right**: UI-language pills (zh / en / ja — pill buttons, active = accent fill) + theme toggle button (`panel2` circle, sun/moon icon).

### 2. Home — Tree Nav + Card Grid (view `home`)
- **Layout**: two columns — left sidebar taxonomy tree + right main column.
- **Sidebar tree**: two-level taxonomy (一级大类 → 二级子类). Each top category row is expandable (chevron), shows a repo count, highlights when active (accent). Sub-rows indent and show their own counts. "All" node at top. Categories: Frontend / Backend / AI & ML / Systems & Infra / Dev Tools / Languages (labels localized via `CAT_LABELS`).
- **Filter bar** (above grid): three `select` dropdowns — fine-grained filter, license, and sort-by — each `panel` bg, `border`, radius `8px`, padding `6px 30px 6px 11px`, with a label in muted 11px/600. Sort options include Total Stars / Today / This Week / This Month / Recently Updated.
- **Card grid**: `display:grid; grid-template-columns: repeat(auto-fill, minmax(288px,1fr)); gap:14px`.
- **Repo card** (the core reusable component — one `repoCard` builder feeds home grid, related-repos carousels, and article-page related repos, so build ONE component):
  - Column flex, gap `10px`, padding `16px`, `panel` bg, `1px solid border`, radius `12px`, pointer cursor. Hover = shadow + accent border.
  - Row 1: language color dot (9px circle) + `owner/name` (14px/700, ellipsis) on the left; `▲ +delta` on the right (accent2, 11px/700).
  - Description: 2-line clamp (`-webkit-line-clamp:2`), 12.5px/1.5 muted, min-height ~37px.
  - Chip row (wrap, gap 6px): language chip (filled with language color, white text, 10px/600), optional artifact/registry chip (accent border + box icon), license chip (shield icon, muted, border), then up to a couple topic chips (panel2).
  - Optional homepage link row: globe icon + hostname, accent, ellipsis.
  - Footer (top `1px solid border`, padding-top 10px): stars (filled star icon in accent2, 12px/700) + forks (fork icon, muted); on the right either an "updated X ago" pill (when sorting by recency) or a mini star-history **sparkline** (76×26 svg: filled area at 0.16 opacity + polyline, both accent).
- **Pagination**: prev/next buttons + numbered page buttons (active = accent fill, radius 8px, 32px min-width), range text ("showing X–Y of Z").

### 3. Repo Detail (view `detail`)
- Back button (arrow + label, muted, text button).
- Title `owner/name`, description (muted, max-width 640px).
- **Stat grid**: `grid-template-columns: repeat(4,1fr); gap:12px` — Stars (value in accent), Forks, Watchers, Issues. Each: `panel` box, border, radius 12px, padding 14px; label 11px muted, value 19px/800.
- **Growth chart**: section label ("Growth · Created in YYYY"), then a 540×170 svg on a `panel` card — filled area (accent 0.14) + polyline (accent, 2.5 stroke).
- **Artifacts row** (if the repo publishes a package): accent-bordered panel, box icon + "Artifacts" label (accent), registry pill, and a monospace install command (IBM Plex Mono, muted, ellipsis).
- **README block**: bordered panel; header bar (`panel2`) with title + a **collapse/expand toggle** (shown only when content exceeds height) + README language buttons (zh/en/…). Body is capped to ~440px with a bottom fade-out gradient when collapsed; "展开全部 / 收起" toggles full height.
- **Related Repos carousel**:
  - Uppercase muted section label.
  - A `position:relative` wrapper. Inside: a horizontal flex scroller (`overflow-x:auto`, `gap:14px`, `scroll-behavior:smooth`, scrollbars hidden) of repo cards (each `width:290px; flex-shrink:0`).
  - **Prev/Next arrow buttons are absolutely positioned OVER the card layer** — `position:absolute; top:50%; transform:translateY(-50%); z-index:5`, left at `-6px` / right at `-6px`, 38×38 circle, `panel` bg + `border`, `box-shadow:0 4px 14px rgba(20,20,40,0.16)`, chevron icon. Clicking scrolls by ~85% of the visible width (`scrollBy({left, behavior:'smooth'})`).
- **Related Articles carousel**: same absolute-arrow-over-cards pattern; article cards are `width:280px; flex-shrink:0`, `panel` bg, radius 10px — category pill (colored per `BLOG_CAT_COLOR`), 2-line title clamp, read-time meta.

### 4. Blog / Article view (view `blog` / article)
- Blog list of article cards, and an article reading view (title, category pill, date + read-time, lead paragraph + body paragraphs). Article view also renders a **grid** of related repo cards (reusing the same card component; grid `minmax(288px,1fr)`).

## Interactions & Behavior
- **Navigation**: clicking a repo card → detail view; back button → home; nav buttons switch between home/blog; article cards → article view. All client-side view switches (no full reload).
- **Search**: controlled input filters the repo list live.
- **Filters/sort**: changing any `select` re-filters/re-sorts the grid; changing category/sub in the tree filters and resets to page 1.
- **Pagination**: prev/next disabled at bounds; clicking a page number jumps.
- **Theme toggle**: swaps light/dark within the current palette. Palette (a/b/c/d) is per-instance.
- **UI language (zh/en/ja)**: swaps all UI strings via the `T` dictionary; also resets README language override.
- **README collapse**: toggle expands/collapses; fade gradient only while collapsed; toggle only present when content overflows the cap.
- **Carousels**: prev/next scroll the row by ~85% of its client width, smooth behavior. Scrollbar hidden; arrows are the only affordance (consider adding keyboard/swipe as an enhancement). Related repos pull up to 12; related articles up to 8.
- **Card hover**: shadow lift + accent border (repo cards); accent-colored border (article cards).

## State Management
Per-view state needed: `view` (home/detail/blog/article), `search` string, `cat`/`sub` (active taxonomy selection), `expanded` (which tree categories are open), `page`, `sort`, `license` + fine filter values, `lang` (UI language), `theme` (light/dark), `palette` (a–d), `selectedRepo`, `readmeLang` override, `readmeExpanded`. Data fetching: replace the in-file `REPOS`/`BLOG` arrays with real API calls; derived values (formatted numbers, sparkline paths, related-repo/related-article matching) are computed from the base records — see `repoCard`, `relatedReposForArticle`, and the sparkline builder in the file for the exact logic.

## Assets
No binary assets. All icons are **inline SVG** (stroke-based, 24-viewBox: search, chevrons, star, fork, shield, globe, box/package, clock, sun/moon, back arrow). Fonts load from Google Fonts. Reproduce icons with your codebase's icon library (e.g. lucide/heroicons — the shapes match Feather-style icons) or copy the inline SVGs from the file.

## Files
- `GitHub高星仓库排名网站.dc.html` — the full prototype. Focus on the `#2a` section (Turn 2) for the approved design; the `THEMES`, `LANG_COLORS`, `CAT_MAP`/`CAT_LABELS`/`TAXONOMY`, `T` (i18n), `REPOS`, `BLOG` data objects; and the `Component` class methods `repoCard`, home-view builder, detail-view builder, and `scrollRow` (carousel logic).
