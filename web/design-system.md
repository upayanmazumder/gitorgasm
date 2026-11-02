# Gitorgasm homepage design system

Owner: design direction by Opus 5.5. Implementation by Sonnet 5.5.
Scope: the homepage (`/`), the brand mark, and the favicon. Application pages keep their current look.
This file replaces the older "Signal Field" direction.

## 1. Direction

Name: **Sunbeam**. A marigold cat sits on the product, in a soft beam of light.

- One memorable element: the waving cat on top of the scoreboard. Everything else stays quiet.
- The git commit node (a ring on a line) is the only recurring graphic device. It appears in the mark, the cat's tail, and the "How it works" timeline.
- Marigold relates to GitHub stars without copying any GitHub asset. Navy-slate ink keeps the page serious.
- Readable first. No text below 14px. Body text is 16px or larger.
- No kicker labels, no all-caps, no mono data labels, no arrow glyphs in buttons, no identical card grids, no decorative textures.

## 2. Color tokens

Scope all tokens to `.home` in `src/app/globals.css`. Replace the current `--home-*` block entirely. Remove `--home-ink`, `--home-ember`, `--home-signal`, and `--home-on-ember`, and migrate every usage. Do not change the shadcn `:root`/`.dark` variables.

| Token                  | Light     | Dark      | Use                                            |
| ---------------------- | --------- | --------- | ---------------------------------------------- |
| `--home-bg`            | `#F5F7FA` | `#121A24` | Page background                                |
| `--home-surface`       | `#FFFFFF` | `#1A2431` | Panels                                         |
| `--home-surface-hover` | `#EBEFF4` | `#223042` | Secondary button hover, selected segment       |
| `--home-text`          | `#14202E` | `#E8EDF3` | Primary text                                   |
| `--home-muted`         | `#4A5868` | `#A7B3C2` | Secondary text                                 |
| `--home-line`          | `#D6DCE4` | `#2A3747` | Decorative dividers and panel borders          |
| `--home-line-strong`   | `#7A8797` | `#6B7B8F` | Control borders, timeline line (3:1 or better) |
| `--home-brand`         | `#F4B02A` | `#F4B02A` | Primary button fill, mark tile                 |
| `--home-brand-hover`   | `#E9A215` | `#F7BF4F` | Primary hover                                  |
| `--home-brand-press`   | `#DB9610` | `#E9A215` | Primary active                                 |
| `--home-brand-border`  | `#A87000` | `#F4B02A` | 1px primary button border                      |
| `--home-on-brand`      | `#14202E` | `#14202E` | Text on marigold                               |
| `--home-brand-text`    | `#8A5A00` | `#F4B02A` | Marigold-family text, if needed                |
| `--home-focus`         | `#1D5FD1` | `#7FB0FF` | Focus ring                                     |
| `--home-data`          | `#2C3E52` | `#C3CEDB` | Score bars, sparkline                          |
| `--home-data-track`    | `#E3E8EF` | `#2A3747` | Bar tracks                                     |

Mascot colors do not change with theme. Define them once on `.home`:

| Token            | Value     | Use                                   |
| ---------------- | --------- | ------------------------------------- |
| `--mascot-fur`   | `#F4B02A` | Body, head, legs, tail fill           |
| `--mascot-shade` | `#DB9610` | Inner ears                            |
| `--mascot-light` | `#FFE3A3` | Muzzle, belly                         |
| `--mascot-ink`   | `#14202E` | Outlines, eyes, nose, mouth, whiskers |
| `--mascot-paper` | `#FFFFFF` | Commit-node centers                   |

### Verified contrast pairs (WCAG 2.x relative luminance, computed)

| Pair                    | Light | Dark | Requirement |
| ----------------------- | ----- | ---- | ----------- |
| text on bg              | 15.3  | 14.9 | 4.5         |
| text on surface         | 16.5  | 13.3 | 4.5         |
| text on surface-hover   | 14.2  | 11.4 | 4.5         |
| muted on bg             | 6.8   | 8.2  | 4.5         |
| muted on surface        | 7.3   | 7.4  | 4.5         |
| on-brand on brand       | 8.7   | 8.7  | 4.5         |
| on-brand on brand-hover | 7.6   | 9.8  | 4.5         |
| on-brand on brand-press | 6.6   | 7.6  | 4.5         |
| brand-text on bg        | 5.5   | 9.3  | 4.5         |
| focus on bg             | 5.4   | 8.0  | 3.0         |
| line-strong on bg       | 3.4   | 4.1  | 3.0         |
| line-strong on surface  | 3.7   | 3.6  | 3.0         |
| brand-border on bg      | 3.9   | n/a  | 3.0         |
| data on data-track      | 8.9   | 7.6  | 3.0         |

`--home-line` is decorative only. Never use it as the only boundary of a control.
Marigold appears only in the mark, the cat, the primary button, and the light beam. Data stays neutral.

## 3. Typography

Load homepage fonts with `next/font/google` in `src/components/home/fonts.ts`. Apply the variables on the `.home` root only. Do not change `layout.tsx` fonts, because application pages use them.

- Display: **Bricolage Grotesque**, variable, weights 600 to 700. Its soft ink traps match the cat's round joins.
- Text: **Instrument Sans**, variable, weights 400 to 600.
- Remove Geist from homepage styles. Do not use a monospace face on the homepage.

| Role                     | Font       | Size                                      | Line height        | Weight           | Tracking |
| ------------------------ | ---------- | ----------------------------------------- | ------------------ | ---------------- | -------- |
| Display (h1)             | Bricolage  | `clamp(2.5rem, 1.6rem + 3.2vw, 4rem)`     | 1.04               | 650              | -0.03em  |
| Heading (h2)             | Bricolage  | `clamp(1.875rem, 1.4rem + 1.6vw, 2.5rem)` | 1.12               | 650              | -0.02em  |
| Title (h3, panel titles) | Bricolage  | 1.25rem                                   | 1.3                | 600              | -0.01em  |
| Wordmark                 | Bricolage  | 1.25rem nav, 1.125rem footer              | 1                  | 700              | -0.02em  |
| Lead                     | Instrument | 1.1875rem (1.125rem below 768px)          | 1.6                | 400              | 0        |
| Body, links, buttons     | Instrument | 1rem                                      | 1.6 (buttons 1.25) | 400, buttons 600 | 0        |
| Small (minimum)          | Instrument | 0.875rem                                  | 1.5                | 400 to 500       | 0        |
| Score figure             | Bricolage  | 1.5rem                                    | 1                  | 600              | -0.01em  |

- 14px is the floor for every visible string, including captions, legal text, rank numbers, and pills.
- Use `font-variant-numeric: tabular-nums` on scores and ranks. Check that the font supports `tnum`. If it does not, right alignment is enough. Do not add a mono font.
- Use sentence case everywhere. Use `text-wrap: balance` on h1 and h2 only.
- Measure: h1 max 15ch, lead max 52ch, body max 60ch.

## 4. Space, size, and layout

Spacing grid is 4px. Use only these steps: 4, 8, 12, 16, 24, 32, 48, 64, 96, 128.

| Item                  | Value                                                                         |
| --------------------- | ----------------------------------------------------------------------------- |
| Container max width   | 1200px                                                                        |
| Gutters               | 20px below 768px, 32px from 768px, 48px from 1024px                           |
| Breakpoints           | 768px and 1024px (keep the current `max-width: 767px` / `1023px` query style) |
| Review widths         | 390, 768, 1024, 1440                                                          |
| Hero padding block    | 96px desktop, 56px top and 64px bottom below 768px                            |
| Section padding block | 112px desktop, 72px below 768px                                               |
| Nav height            | 72px, not sticky                                                              |
| Touch targets         | 44px minimum height and width, 8px minimum gap                                |

### Radii

| Token                   | Value                 | Use                                  |
| ----------------------- | --------------------- | ------------------------------------ |
| `--home-radius-control` | 10px                  | Buttons, segments                    |
| `--home-radius-panel`   | 18px                  | Scoreboard, filter panel             |
| `--home-radius-pill`    | 999px                 | "Illustrative" pills, timeline nodes |
| Mark tile               | 7.5 / 32 of tile size | Brand tile only                      |

### Elevation and borders

- Level 0: page and timeline. No border, no shadow.
- Level 1: the two product panels only. Border `1px solid var(--home-line)` plus `--home-shadow-panel`.
  - Light: `0 1px 2px rgb(20 32 46 / 6%), 0 16px 40px -20px rgb(20 32 46 / 18%)`
  - Dark: `0 16px 40px -20px rgb(0 0 0 / 50%)`
- No other shadows. No glass or blur.

### Focus

- `outline: 2px solid var(--home-focus); outline-offset: 3px` on `a`, `button`, and the mascot button (offset 4px, radius 24px).
- Use `:focus-visible` only. Never remove outlines.
- `main[id]` and `section[id]` keep `scroll-margin-top: 24px` (the nav is not sticky).

## 5. Controls

All controls are links or buttons with a minimum height of 44px. Use `cursor: pointer` and `touch-action: manipulation`.

| Control        | Rest                                                                                                             | Hover                       | Active                                  | Focus |
| -------------- | ---------------------------------------------------------------------------------------------------------------- | --------------------------- | --------------------------------------- | ----- |
| Primary        | `--home-brand` fill, `--home-on-brand` text, 1px `--home-brand-border`, 48px high, 20px inline padding, 16px/600 | `--home-brand-hover`        | `--home-brand-press`, `translateY(1px)` | Ring  |
| Secondary      | Transparent, `--home-text`, 1px `--home-line-strong`, 48px high                                                  | `--home-surface-hover` fill | `translateY(1px)`                       | Ring  |
| Nav "Open app" | Secondary style, 44px high                                                                                       | Same                        | Same                                    | Ring  |
| Text link      | `--home-text`, underline 1px, offset 4px                                                                         | Underline 2px               | None                                    | Ring  |

- Hover changes color only. Transition `background-color, border-color` over `--motion-hover`.
- Do not append arrows or other glyphs to labels. External links take no icon. Their labels say "GitHub".
- No disabled states exist on the homepage.

## 6. Brand mark

### Concept

An original seated cat whose tail rises as a git line and ends in a commit ring. The mascot's tail forks into a branch with two commit rings.

Distinctness rules. They are required, because the mark must not suggest GitHub affiliation:

- Never put the cat in a circle. Never use a knockout cat silhouette in a disc.
- No tentacles, no octopus limbs, no Octocat proportions. Do not trace or adapt any GitHub artwork.
- Use a wide squircle head, round-tipped ears, vertical capsule eyes, a visible seated body, and the branch tail.
- The mascot's right ear (viewer's right) has a small notch.
- Remove the GitHub logo path in `features/workflow-illustrations.tsx` when you delete that file (section 9). Show "GitHub" as text where needed.

### Small mark (24 to 32px): `viewBox="0 0 32 32"`

Draw these shapes in this order. The cat is `--mascot-ink`. The tile is `--mascot-fur`.

| Part                     | Geometry                                                                                                                                        |
| ------------------------ | ----------------------------------------------------------------------------------------------------------------------------------------------- |
| Tile (tile variant only) | `<rect width="32" height="32" rx="7.5" fill="var(--mascot-fur)"/>`                                                                              |
| Left ear                 | `M6.2 14 L7.6 5.4 Q8 4 9.2 4.9 L14 9.5 Z`                                                                                                       |
| Right ear                | `M21.8 14 L20.4 5.4 Q20 4 18.8 4.9 L14 9.5 Z`                                                                                                   |
| Head with eye holes      | One path with `fill-rule="evenodd"`: a rounded rect x 5, y 8.5, w 18, h 13, r 6. Add two capsule holes: x 9.6 and 16, y 13, w 2.4, h 3.8, r 1.2 |
| Body                     | `M7.5 20 H20.5 V26 A2 2 0 0 1 18.5 28 H9.5 A2 2 0 0 1 7.5 26 Z`                                                                                 |
| Tail                     | `M20.5 26.8 C24.6 26.8 26 24.8 26 21.5 V12.6`, stroke 2.4, round caps, no fill                                                                  |
| Commit ring              | `<circle cx="26" cy="10" r="2.6" fill="none" stroke-width="2.2"/>`                                                                              |

- Eye holes use `evenodd`, so the mark works on any background without mask ids.
- The nav, footer, favicon, and apple icon all use the tile variant. This gives the same marigold recognition in both themes. Ink on marigold is 8.7:1.
- Clear space: 25% of the tile size on all sides. The wordmark sits 10px from the tile.
- Sizes: nav tile 28px, footer tile 24px, favicon 32px viewBox, apple icon 180px.
- Minimum size: 16px. Below 20px, keep the drawing the same. Do not add detail at larger sizes.

### Mascot (hero): `viewBox="-16 0 192 160"`

Render at 192 × 160 from 768px, and at 144 × 120 below 768px. Always set `width` and `height` attributes.
Unit rules: outlines are 3 units in `--mascot-ink` with `stroke-linejoin="round"`. Fills are flat. No gradients, no shadows.
The ground line is y = 150. The cat's base sits on the scoreboard's top border.

Draw bottom to top. Each named item is a separate `<g>` with a `data-part` attribute.

1. `tail`
   - Main: `M110 144 C134 146 142 136 142 118 V58`
   - Branch: `M142 116 C142 102 156 102 156 90 V86`
   - Stroke both paths in ink at width 13, then again in fur at width 8. Use round caps on both.
   - Rings: circles at (142, 50) and (156, 80), r 8, fur fill, 3-unit ink stroke. Each gets an inner circle r 3 in `--mascot-paper`.
2. `body`
   - `M38 150 V110 C38 91 54 80 76 80 C98 80 114 91 114 110 V150 Z` in fur with outline.
   - Belly: ellipse cx 76, cy 124, rx 20, ry 22, `--mascot-light`, no outline.
3. `leg-static` (viewer's right)
   - Line `M100 100 L92 142`: ink stroke width 22, then fur stroke width 16, round caps.
   - Paw: ellipse cx 92, cy 146, rx 11, ry 7, fur with outline. Two toe lines `M88 143 v4` and `M96 143 v4`, ink, width 2.
4. `head`
   - Left ear: `M36 52 L42 16 Q43.5 10 48.5 13.5 L74 38 Z`
   - Right ear with notch: `M116 52 L113.3 35.8 L109 32.5 L112.4 30.4 L110 16 Q108.5 10 103.5 13.5 L78 38 Z`
   - Ears are fur with outline. Draw them before the head shape, so the head covers the ear bases.
   - Inner ears: `M44 42 L47 22 L64 38 Z` and the mirror around x = 76, in `--mascot-shade`, no outline.
   - Head: rect x 30, y 34, w 92, h 62, rx 28, fur with outline.
   - Muzzle: ellipse cx 76, cy 80, rx 19, ry 12, `--mascot-light`.
   - `eyes-open`: rects x 56 and 87, y 54, w 9, h 14, rx 4.5, ink.
   - `eyes-happy`: `M54 63 Q60.5 55 67 63` and `M85 63 Q91.5 55 98 63`, ink stroke 4.5, round caps, no fill. Opacity 0 at rest.
   - Nose: `M71.5 73 h9 a1.5 1.5 0 0 1 1.2 2.4 l-4.5 4.6 a1.6 1.6 0 0 1 -2.4 0 l-4.5 -4.6 a1.5 1.5 0 0 1 1.2 -2.4 Z`, ink.
   - Mouth: `M68 82 Q72 87 76 82 Q80 87 84 82`, ink stroke 3, round caps.
   - Whiskers: `M50 80 L32 77`, `M50 86 L33 89`, `M102 80 L120 77`, `M102 86 L119 89`, ink stroke 2.5, round caps.
5. `arm-wave` (viewer's left, drawn last)
   - Line `M52 100 L60 142`: ink width 22, then fur width 16, round caps.
   - Paw: ellipse cx 60, cy 146, rx 11, ry 7, with toe lines `M56 143 v4` and `M64 143 v4`.
   - Pivot: `transform-origin: 52px 100px; transform-box: view-box`.
   - Rest pose (SSR and no-JS) is `rotate(0deg)`. The paw is down and matches the other leg.

Geometry check: at 90°, 132°, 150°, and 165°, the paw stays inside x ≥ -16 and y ≥ 0. Adjust coordinates visually if needed. Keep the proportions: the head is 1.2× the body width, the ears are 0.4× the head height, and the eyes sit on the head's upper half.

### Illustration style (all homepage graphics)

- Flat vector, round caps and joins, one stroke weight per graphic.
- Commit node: a ring on a line. It is the only motif. Do not add other git glyphs.
- No emoji. Remove the text glyphs `↗ → ⌄ ✓ ↻`. Use small inline SVG only where the shape carries meaning.

## 7. Motion

Add these tokens to `.home`. Keep `src/lib/motion.ts` unchanged. The mascot uses CSS keyframes, because they run from server HTML before hydration.

| Token              | Value                              | Use                           |
| ------------------ | ---------------------------------- | ----------------------------- |
| `--motion-press`   | 90ms                               | Active translate              |
| `--motion-hover`   | 160ms                              | Color changes                 |
| `--motion-state`   | 240ms                              | Panel and segment state       |
| `--motion-wave`    | 1600ms                             | One full wave                 |
| `--ease-out`       | `cubic-bezier(0.16, 1, 0.3, 1)`    | Arrivals (same as `EASE.out`) |
| `--ease-in-out`    | `cubic-bezier(0.65, 0, 0.35, 1)`   | Oscillation and return        |
| `--ease-overshoot` | `cubic-bezier(0.34, 1.4, 0.64, 1)` | Paw raise                     |

### Wave choreography (`arm-wave`, one run, `animation-fill-mode: both`)

| Keyframe | Arm rotation | Timing function into next |
| -------- | ------------ | ------------------------- |
| 0%       | 0deg         | `--ease-overshoot`        |
| 20%      | 150deg       | `--ease-in-out`           |
| 32%      | 132deg       | `--ease-in-out`           |
| 44%      | 165deg       | `--ease-in-out`           |
| 56%      | 132deg       | `--ease-in-out`           |
| 68%      | 160deg       | `--ease-in-out`           |
| 80%      | 150deg       | `--ease-in-out`           |
| 100%     | 0deg         | end                       |

- `head`: rotate -4deg around (76px, 96px) from 20% to 80%. Ease in and out over the same 1600ms.
- `eyes-happy` opacity 1 and `eyes-open` opacity 0 from 22% to 78%. Use `step-end` so the eyes swap without a fade.
- Animate `transform` and `opacity` only. Nothing outside the SVG moves. The SVG box never changes size.
- No idle loop, no blink loop, no hover motion, no tail motion.

### Triggers

- First appearance: run once when the mascot is 60% visible (`IntersectionObserver`, then disconnect). This covers mobile, where the mascot can start below the fold.
- Replay: the mascot is a `<button type="button">`. Click, tap, Enter, or Space restarts the wave from 0%. Restart it by incrementing a React `key` on the SVG. Never depend on `animationend` for state.
- A press during a wave restarts the wave. Input is never blocked.

### Reduced motion

- `@media (prefers-reduced-motion: reduce)`: no animation. Show the static greeting pose: `arm-wave` at `rotate(150deg)`, `eyes-happy` visible, `eyes-open` hidden, head upright.
- With reduced motion, render the art in `<div role="img" aria-label="The Gitorgasm cat waving hello">`, not in a button. A control that does nothing is not allowed.
- Detect the preference with a `useSyncExternalStore` media hook, the same pattern as `effects/use-field-effects.ts`. The server snapshot returns `false`.

### Page motion budget

There are at most two animated elements per view: the mascot (once) and LightRays (ambient, desktop only).

- No section entrance animations, no stagger, no scroll reveals, no card hover lifts.

### React Bits decisions

- **LightRays: keep.** It is the sunbeam that the cat sits in. Set `raysColor` from `--home-brand`, origin `top-right`, `raysSpeed` 0.08, `opacity` 0.22 light and 0.3 dark. Keep `followMouse`, `cursorShadow`, `clickPulse`, and `intro` off. Keep the current gating and CSS radial fallback. Recolor the fallback to `--home-brand` at 10%.
- **SpotlightCard: keep on the scoreboard panel only.** Set `spotlightColor` to `var(--home-brand)`, intensity 0.06, and keep the current flags.
- **DotGrid: remove.** It competes with the mascot. Delete `react-bits/dot-grid.tsx`, its barrel export, the `.dots` styles, and its row and prose in `web/README.md`.
- Do not install other React Bits or packages.

## 8. Page structure and final copy

```
Desktop ≥1024px
┌──────────────────────────────────────────────────────────────┐
│ [tile] Gitorgasm                  How it works   [Open app]   │
├──────────────────────────────────────────────────────────────┤
│ H1 Your GitHub organization’s        ░ sunbeam ░    (cat)    │
│    activity, scored daily.                         /|  tail  │
│ Lead (52ch)                          ┌──────────────────────┐ │
│ [Connect an organization] [How it    │ scoreboard panel     │ │
│ works]                               │                      │ │
│ Sign in with GitHub to start.        └──────────────────────┘ │
├──────────────────────────────────────────────────────────────┤
│ How it works                                                  │
│ (1)────────────────(2)────────────────(3)                     │
│ Connect…           Choose…            Come back…              │
├──────────────────────────────────────────────────────────────┤
│ Decide what each score includes.     ┌ filter panel ────────┐ │
│ Text, boundary note                  │                      │ │
│ [Connect an organization]            └──────────────────────┘ │
├──────────────────────────────────────────────────────────────┤
│ [tile] Gitorgasm  description        links                    │
│ Not affiliated with GitHub.  © 2026 Gitorgasm                 │
└──────────────────────────────────────────────────────────────┘
Below 1024px: one column in this order: copy, actions, cat + scoreboard, timeline (vertical), scope copy, filter panel, footer.
```

Alignment: left-aligned everywhere, also on mobile. Hero columns are `minmax(0, 1fr) minmax(0, 1fr)` with a 64px gap. The copy column is vertically centered.

### Navigation (`home-navigation.tsx`)

- Left: a link to `/` with the 28px tile and the text "Gitorgasm". The tile is `aria-hidden`.
- Right: text link "How it works" (`#how-it-works`). Hide it below 480px, because the hero has the same link. Then the secondary button "Open app" (`/organizations`).
- Remove the duplicate "Organizations" link and the `↗` glyph.

### Hero (`hero/home-hero.tsx`)

- Remove the `// signal over noise` kicker.
- H1: **Your GitHub organization’s activity, scored daily.**
- Lead: **Connect an organization to see activity scores for its contributors and repositories. Choose which activity counts, pick a UTC date period, and save the views you check often.**
- Primary: **Connect an organization** → `/organizations`
- Secondary: **How it works** → `#how-it-works`
- Note (14px, muted): **Sign in with GitHub to start.**
- Product column: `<WavingMascot />`, then `<ContributionScoreboard />`. The column has `padding-top` equal to the mascot height above the ground line: 150px desktop, 113px mobile. The mascot is absolutely positioned, right inset 32px (20px mobile). Its ground line overlaps the panel's top border, and the paws overlap the border by about 5px. The mascot sits above the panel in z-order. In DOM order the mascot comes before the figure.

### Scoreboard (`scoreboard/*`)

- Toolbar: **example-org** and the pill **Illustrative data**. Remove the `g/` mark.
- Title: **Contributor scores**. Meta: **Last 30 days, UTC**. Do not use middle dots.
- Rows (illustrative): 1 `ana-k` 92, 2 `jdoe` 71, 3 `mori` 58, 4 `kline` 44. Remove `octo-a`. Show ranks as plain numbers, not zero-padded.
- Row height 56px. Names 16px. Ranks 14px muted. Scores use the score figure style. Bars are 6px high in `--home-data` on `--home-data-track`, radius pill.
- Repository block: **Repository activity**, **example-org/core**, sparkline in `--home-data`.
- Panel footer: **Scores refresh daily.** Remove the status dot.
- Figcaption: **Sample figures for illustration. Not live data.**
- Panel padding 24px (16px below 768px). Remove the column header row, because it is `aria-hidden` and adds nothing.

### How it works (`features/home-features.tsx`)

Replace the three cards and illustrations with a commit-line timeline. The content is a real sequence, so numbered nodes are correct.

- H2: **How it works**
- Intro: **Three steps from a GitHub organization to scores you can return to.**
- Markup: `<ol>`. Each `<li>` has a 32px node (a ring with 2px `--home-line-strong` border, `--home-surface` fill, and the step number at 14px/600), an h3, and a paragraph.
- The line is 2px `--home-line-strong`, behind the nodes. It is horizontal from 1024px with three equal columns. Below 1024px it is vertical on the left, with the text 48px from the line.
  1. **Connect an organization**: **Sign in with GitHub and install the GitHub App on your organization. Gitorgasm shows the contributors and repositories that the app can access.**
  2. **Choose what counts**: **Pick an activity type and a UTC date period. Hide bots if you want, then save the combinations you check often as presets.**
  3. **Come back to fresh scores**: **Scores update daily, so each visit reflects recent activity in your organization.**
- Delete `features/workflow-illustrations.tsx` and its styles. No panels, no shadows in this section.

### Scope (`scope/home-scope.tsx`)

- H2: **Decide what each score includes.**
- Paragraph: **Filter by code, reviews, or issues. Set an exact UTC window, or use a preset period such as the last 7 or 30 days.**
- Boundary note (16px, muted): **Scores summarize recorded GitHub activity. They do not measure productivity or the value of anyone’s work.**
- Action: primary button **Connect an organization** → `/organizations`. The action keeps the same name as in the hero.
- Filter panel (level 1, static picture, `aria-hidden` on the decoration):
  - Header: **Activity filters** and the pill **Illustrative view**
  - **Activity**: a segmented row **All, Code, Reviews, Issues** with **Reviews** selected (`--home-surface-hover` fill, 1px `--home-line-strong`, 600 weight). The real control is single-select, so exactly one segment is selected.
  - **Time period (UTC)**: **Last 30 days**, with an inline SVG chevron
  - **Hide bots**: an SVG checkbox in the checked state
  - **Saved preset**: **Monthly review**, with the existing bookmark SVG
  - Figcaption: **Example filter settings, not a working form.**
- All text in the panel is 14 to 16px.

### Footer (`footer/home-footer.tsx`)

- 24px tile and **Gitorgasm** (link to `/`).
- **Activity scores for GitHub organizations.**
- Links: **How it works** (`#how-it-works`), **Organizations** (`/organizations`), **GitHub** (`https://github.com/upayanmazumder/gitorgasm`). Remove `↗`.
- Bottom row (14px, muted): **Gitorgasm is not affiliated with GitHub.** and **© 2026 Gitorgasm**.
- No mascot, no motion in the footer.

### Metadata

- In `src/app/page.tsx`, export page metadata. Description: **Activity scores for the contributors and repositories in your GitHub organization.** Do not change the root layout metadata.

## 9. Files for Sonnet

| File                                                                                                                           | Action                                                                                                                                               |
| ------------------------------------------------------------------------------------------------------------------------------ | ---------------------------------------------------------------------------------------------------------------------------------------------------- |
| `src/components/brand/brand-mark.tsx`                                                                                          | New server component `BrandMark` with props `size` (number) and `className`. Renders the tile variant from section 6. Uses `aria-hidden` by default. |
| `src/components/brand/index.ts`                                                                                                | Barrel                                                                                                                                               |
| `src/components/home/mascot/mascot-head.tsx`                                                                                   | Server: `head` group                                                                                                                                 |
| `src/components/home/mascot/mascot-body.tsx`                                                                                   | Server: `tail`, `body`, `leg-static` groups                                                                                                          |
| `src/components/home/mascot/mascot-art.tsx`                                                                                    | Server: `<svg>` that composes the parts plus `arm-wave`, with props `playing` and `className`                                                        |
| `src/components/home/mascot/waving-mascot.tsx`                                                                                 | `'use client'` leaf: observer, replay key, reduced-motion branch, button                                                                             |
| `src/components/home/mascot/use-reduced-motion-preference.ts`                                                                  | `useSyncExternalStore` media hook                                                                                                                    |
| `src/components/home/mascot/mascot.module.css`                                                                                 | Keyframes, pose rules, button and focus styles                                                                                                       |
| `src/components/home/mascot/index.ts`                                                                                          | Barrel exporting `WavingMascot`                                                                                                                      |
| `src/components/home/fonts.ts`                                                                                                 | `next/font/google` for Bricolage Grotesque and Instrument Sans with CSS variables                                                                    |
| `src/app/icon.svg`                                                                                                             | Tile mark with literal hex colors (the file is standalone)                                                                                           |
| `src/app/apple-icon.tsx`                                                                                                       | 180px PNG of the tile mark through the file convention                                                                                               |
| `src/app/favicon.ico`                                                                                                          | Real 16, 32, and 48 pixel frames rendered from `icon.svg`. Do not keep the default Next.js icon.                                                     |
| `src/app/globals.css`                                                                                                          | New `.home` tokens (sections 2, 4, 7). Update the focus rule to `--home-focus`.                                                                      |
| `home.module.css`, `hero.module.css`, `scoreboard.module.css`, `workflow.module.css`, `scope.module.css`, `effects.module.css` | Migrate to the new tokens and type scale. Remove all sizes below 14px.                                                                               |
| `effects/contribution-field.tsx`                                                                                               | LightRays only, brand color. Remove DotGrid.                                                                                                         |
| `effects/signal-surface.tsx`                                                                                                   | Brand spotlight. Scoreboard is the only consumer.                                                                                                    |
| `react-bits/dot-grid.tsx`, `react-bits/index.ts`, `web/README.md`                                                              | Remove DotGrid. Update the homepage effects prose and provenance table.                                                                              |

Implementation notes:

- `WavingMascot` state: `run` (number, starts 0) and `playing` (boolean). The observer sets `playing` once. A click sets `playing = true` and increments `run`. `<MascotArt key={run} playing={playing} />`. The CSS animation applies only under `[data-playing='true']`.
- Button: `aria-label="Make the Gitorgasm cat wave"`. The SVG inside is `aria-hidden="true"` and `focusable="false"`. Transparent background, no border, `border-radius: 24px`. The button box equals the SVG box, which is larger than 44px.
- Use `cursor: pointer`. A press gives no scale feedback, because the wave is the feedback.
- Read `node_modules/next/dist/docs/` for `next/font` and the `icon`/`apple-icon` metadata file conventions before you write those files. Next.js 16 can differ from older versions.
- Keep files near 80 lines. Use named exports and barrels. Comment only the reason for a non-obvious choice.
- Do not touch `src/components/organizations/**`, `src/components/ui/**`, the shadcn tokens, or root layout fonts.

## 10. Acceptance checklist

Brand

- [ ] The mark reads as a cat with a commit-ring tail at 16, 24, and 28px, on light and dark backgrounds.
- [ ] No circle container, no tentacles, no GitHub artwork anywhere. The Invertocat path is gone from the repository's homepage code.
- [ ] The favicon and apple icon show the marigold tile mark.

Mascot

- [ ] The SSR HTML contains the mascot in the rest pose with fixed `width`/`height`. CLS from the mascot is 0.
- [ ] The wave runs once when the mascot first becomes 60% visible, then stops in the rest pose.
- [ ] Click, tap, Enter, and Space replay the wave. Repeated presses restart it cleanly.
- [ ] With reduced motion: static greeting pose, `role="img"` with a label, no button, no animation.
- [ ] Only `transform` and `opacity` animate. No element outside the SVG moves.

Accessibility and layout

- [ ] Every visible string is 14px or larger. Body and lead text are 16px or larger.
- [ ] All text pairs use the tokens from section 2. No raw hex in components except `icon.svg` and `apple-icon.tsx`.
- [ ] Every link and button is at least 44px high, with 8px or more between targets.
- [ ] The focus ring is visible on every control in both themes, and no sticky element hides it.
- [ ] The skip link works. Heading order is h1, then h2, then h3.
- [ ] No horizontal scroll at 390px. Check 390, 768, 1024, and 1440 in both themes.
- [ ] Content is readable with JavaScript disabled and effects off.

Content and restraint

- [ ] The copy matches section 8 exactly. Both mock panels say they are illustrative.
- [ ] No testimonials, logos, pricing, customer claims, real metrics, or team-management features.
- [ ] No kicker labels, all-caps labels, mono labels, middle-dot meta strings, arrow glyphs, or emoji.
- [ ] Marigold appears only in the mark, the mascot, primary buttons, and the light beam.
- [ ] DotGrid is gone from the code and the README. LightRays and SpotlightCard follow section 7.
- [ ] Application pages look the same as before.

Verification (Sonnet, in `web`): `pnpm lint && pnpm typecheck && pnpm test && pnpm build`. Add one component test for `WavingMascot`. With the default preference, a button named "Make the Gitorgasm cat wave" exists. With reduced motion, an image with the greeting label exists and no button exists.

## 11. Handoff to Sonnet 5.5

1. Read this file, `web/AGENTS.md`, and the Next.js 16 docs for fonts and metadata files.
2. Build the tokens and fonts first (sections 2 to 4). Then build `BrandMark`, `icon.svg`, and `apple-icon.tsx`.
3. Build the mascot from the section 6 coordinates. Check the drawing at both sizes and in both themes. Adjust coordinates only to fix visual balance. Keep the parts, the pivot, and the proportions.
4. Add the wave (section 7) and the reduced-motion branch. Then place the mascot on the scoreboard.
5. Rewrite the nav, hero, timeline, scope, and footer with the section 8 copy. Remove DotGrid.
6. Run the checklist. Report any coordinate change and every item that you could not verify.
7. Ask Opus before you add a color, a font, an animation, or a section.

## 12. Implementation notes (Sonnet 5.5)

These notes record how the code maps to this file. They do not change any decision above.

- Mascot coordinates and keyframes follow section 6 and 7 without change. A rest, 132, 150, and 165 degree render check showed the paw inside the view box.
- Pivot change: `arm-wave` and `head` use a translate, animate, translate-back wrapper (`mascot-pivot.tsx`) instead of `transform-origin: 52px 100px; transform-box: view-box`. The origin stays in local user space, so the result does not depend on how a browser places the reference box for a viewBox that starts at x = -16.
- `BrandMark` takes an optional `title`. Without it the mark is `aria-hidden`. Shared geometry lives in `brand/mark-paths.ts`, and `icon.svg` repeats it with literal colors.
- The glyph groups (not the tile rectangle) sit inside `translate(-1.4 0)` in `BrandMark`, `icon.svg`, and `apple-icon.tsx`, because the geometry spans x 5 to 29.7. `apple-icon.tsx` uses a plain `0 0 32 32` viewBox with the same transform.
- `apple-icon.tsx` uses a full-bleed marigold square, not a rounded tile, because iOS applies its own corner mask.
- Renames: `ContributionField` is `SunbeamField`, and `SignalSurface` is `SpotlightPanel`. The old names belonged to the removed direction.
- New shared files: `home/button-link.tsx` (primary, secondary, and text links) and `home/home-brand.tsx` (tile and wordmark).
- Mascot parts are split into `mascot-body`, `mascot-head`, `mascot-limb`, and `mascot-art`. `MascotLimb` draws both the static leg and the waving arm.
- The wave animation rules sit inside `prefers-reduced-motion: no-preference`. The static greeting pose sits inside `prefers-reduced-motion: reduce`. The React hook only decides the semantics (button or image).
- The scoreboard title is an `h2`, because it follows the page `h1` directly. The filter panel title is a `p`, because the panel is `aria-hidden` decoration.
- The checked "Hide bots" box uses `--home-data`, so marigold stays limited to the places in section 2.
- `SunbeamField` is the first child of the page root in `HomeView`, not of the hero. The page root is `position: relative`, `isolation: isolate`, and `overflow-x: clip`. The field is full bleed (`top`, `right`, and `left` at 0, no `100vw`) with a fixed height of 680px (780px from 768px wide), so it spans the navigation and the hero. A bottom mask and a left mask fade the beam, so no side shows a clipping edge. The field has `pointer-events: none` and `z-index: -1`. The header sits above it, so the beam never tints nav text.
- `WavingMascot` starts the first wave only when `entry?.isIntersecting && entry.intersectionRatio >= VISIBLE_SHARE` (0.6). The guard makes the intended condition explicit. A first-paint check at 27% visibility and a scroll check at 25% did not start the wave early, so this is not a reproduced bug.
- Headless browsers report `hover: none` and `pointer: coarse`, so they use the CSS radial fallback. An emulated fine-pointer desktop rendered a real 404 by 483 LightRays canvas with no browser errors. Do not raise the ray opacity based on fallback screenshots.
- The timeline text sits 64px from the node's left edge, which is 48px from the line center.

### Opus 5.5 visual review

Opus 5.5 reviewed screenshots of the implementation and approved the design after four fixes: a full-bleed beam without a hard edge at the content boundary, an explicit 60% visibility guard, a real `favicon.ico` from the new mark, and a centered tile glyph. Opus asked to keep the cat and its wave, the headline wrapping, the type and palette, the score and rank positions, the sparkline weight, the section rhythm, the card-free timeline, the footer affiliation line, the static reduced-motion image, the loading fallback, and the React Bits provenance. Opus asked for no other polish.
