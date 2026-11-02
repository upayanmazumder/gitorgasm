# Gitorgasm design system (Sunbeam)

Owner: design direction by Opus 5.5. Implementation by Sonnet 5.5.
Scope: every route (`/`, `/organizations`, `/organizations/[login]`, `/motion`, and the unmatched 404), the brand mark, and the favicon. One theme applies to all routes.
The homepage keeps its approved composition (sections 9 and 10). Application screens use the same tokens, type, controls, panels, and focus rules.
This file replaces the older "Signal Field" direction and the earlier homepage-only scope.

Sources: the approved Sunbeam palette and type are the source of truth. Local UI/UX guidance supplied two rules: text contrast is 4.5:1 or more, and color is never the only signal. No palette or architecture database match exists, so this file does not cite one.

## 1. Direction

Name: **Sunbeam**. A marigold cat sits on the product, in a soft beam of light.

- One memorable element: the waving cat on top of the homepage scoreboard. Everything else stays quiet.
- Application screens are the product itself. They share the palette, type, controls, panels, and focus. They get no cat, no beam, and no decorative motion.
- The git commit node (a ring on a line) is the only recurring graphic device. It appears in the mark, the cat's tail, the "How it works" timeline, and the neutral status icon.
- Marigold relates to GitHub stars without copying any GitHub asset. Navy-slate ink keeps the product serious.
- Readable first. No text below 14px. Body text is 16px or larger.
- No kicker labels, no all-caps, no mono data labels, no arrow glyphs in controls, no identical card grids, no decorative textures.

## 2. Theme architecture

`src/app/globals.css` is an entry file. It contains imports only, in this order:

```css
@import 'tailwindcss';
@import 'tw-animate-css';
@import 'shadcn/tailwind.css';
@import '../styles/tokens.css';
@import '../styles/theme.css';
@import '../styles/typography.css';
@import '../styles/motion.css';
@import '../styles/base.css';
```

Theme modules in `src/styles`:

- `tokens.css`: raw color tokens on `:root` and `.dark`, `color-scheme`, mascot colors, radii, and the panel shadow.
- `theme.css`: the Tailwind bridge. It holds `@custom-variant dark` and the `@theme inline` mapping. It defines no values.
- `typography.css`: typeface stacks on `:root` and base type roles in `@layer base`.
- `motion.css`: motion tokens, the reduced-motion override, and the `transition-control` utility.
- `base.css`: page colors, the focus ring, scroll margin, and native control rules in `@layer base`.
- `fonts.ts`: the only `next/font` load in the app.
- `index.ts`: the barrel. It exports `fontVariables`.

Rules:

- One theme exists. It sits on `:root` and `.dark`. No route, layout, or component defines theme tokens.
- No `.home` theme scope, no `--home-*` names, and no aliases from old names. Migrate every consumer to the new names.
- Raw tokens use hex literals. `SunbeamField` reads the computed `--primary` value and passes it to WebGL, so it must stay a literal color.
- A mapped name never references itself. Write `--font-sans: var(--typeface-text)`. Never write `--font-sans: var(--font-sans)`. The current `globals.css` has this recursion. Remove it.
- Raw token names stay outside the Tailwind theme namespaces (`--color-*`, `--font-*`, `--text-*`, `--radius-*`, `--shadow-*`, `--ease-*`). The shadcn color names (`--background`, `--primary`, and so on) are not namespaces, so they stay.
- CSS modules use raw tokens only. Tailwind does not process modules, and it does not emit unused theme variables. Tailwind code uses mapped utilities only.
- No raw hex in components. Exceptions: `src/app/icon.svg` and `src/app/apple-icon.tsx`, because they are standalone.
- Keep the existing `ThemeProvider` and its props (`attribute="class"`, `defaultTheme="system"`, `enableSystem`). Add `disableTransitionOnChange`, so a theme change does not animate control colors. `next-themes` sets `.dark` on `<html>`. Do not change `transition-control` or the global motion rules.
- No `forms.css` file. The `ui` form primitives own form styling (section 6). `base.css` holds the native rules that no primitive can own.

## 3. Color

### Raw tokens (`tokens.css`)

`:root` sets `color-scheme: light`. `.dark` sets `color-scheme: dark`, so native controls, scrollbars, and date pickers follow the theme.

| Token                  | Light     | Dark      | Use                                            | Was                    |
| ---------------------- | --------- | --------- | ---------------------------------------------- | ---------------------- |
| `--background`         | `#F5F7FA` | `#121A24` | Page background                                | `--home-bg`            |
| `--foreground`         | `#14202E` | `#E8EDF3` | Primary text, checked native controls          | `--home-text`          |
| `--card`               | `#FFFFFF` | `#1A2431` | Panels, control fills, popovers                | `--home-surface`       |
| `--muted`              | `#EBEFF4` | `#223042` | Hover fill, selected segment, pills            | `--home-surface-hover` |
| `--muted-foreground`   | `#4A5868` | `#A7B3C2` | Secondary text, placeholders                   | `--home-muted`         |
| `--border`             | `#D6DCE4` | `#2A3747` | Decorative dividers and panel borders          | `--home-line`          |
| `--border-strong`      | `#7A8797` | `#6B7B8F` | Control borders, timeline line (3:1 or better) | `--home-line-strong`   |
| `--ring`               | `#1D5FD1` | `#7FB0FF` | Focus ring                                     | `--home-focus`         |
| `--primary`            | `#F4B02A` | `#F4B02A` | Primary button fill, mark tile, beam           | `--home-brand`         |
| `--primary-hover`      | `#E9A215` | `#F7BF4F` | Primary hover                                  | `--home-brand-hover`   |
| `--primary-press`      | `#DB9610` | `#E9A215` | Primary active                                 | `--home-brand-press`   |
| `--primary-border`     | `#A87000` | `#F4B02A` | 1px primary button border                      | `--home-brand-border`  |
| `--primary-foreground` | `#14202E` | `#14202E` | Text on marigold                               | `--home-on-brand`      |
| `--destructive`        | `#B42318` | `#FF8A80` | Error text and icon, invalid border            | New                    |
| `--success`            | `#19703A` | `#7DD69A` | Success text and icon                          | New                    |
| `--chart-1`            | `#2C3E52` | `#C3CEDB` | Score bars, sparkline, illustrative checkbox   | `--home-data`          |
| `--chart-track`        | `#E3E8EF` | `#2A3747` | Bar tracks                                     | `--home-data-track`    |

- `--panel-shadow` (was `--home-shadow-panel`). Light: `0 1px 2px rgb(20 32 46 / 6%), 0 16px 40px -20px rgb(20 32 46 / 18%)`. Dark: `0 16px 40px -20px rgb(0 0 0 / 50%)`.
- Remove `--home-brand-text`. It has no consumer.
- Remove the gray `oklch` shadcn values. Remove the raw `--sidebar-*`, `--popover*`, `--secondary*`, `--accent*`, `--card-foreground`, and `--chart-2` to `--chart-5` tokens. The mapping below covers their utilities.

Mascot colors are brand tokens. They do not change with theme. Define them once on `:root`, so `BrandMark` works on every route.

| Token            | Value     | Use                                   |
| ---------------- | --------- | ------------------------------------- |
| `--mascot-fur`   | `#F4B02A` | Body, head, legs, tail fill           |
| `--mascot-shade` | `#DB9610` | Inner ears                            |
| `--mascot-light` | `#FFE3A3` | Muzzle, belly                         |
| `--mascot-ink`   | `#14202E` | Outlines, eyes, nose, mouth, whiskers |
| `--mascot-paper` | `#FFFFFF` | Commit-node centers                   |

### Tailwind mapping (`theme.css`)

Use this block exactly. Sections 4 and 5 explain the font, radius, and shadow lines.

```css
@custom-variant dark (&:is(.dark *));

@theme inline {
  --color-background: var(--background);
  --color-foreground: var(--foreground);
  --color-card: var(--card);
  --color-card-foreground: var(--foreground);
  --color-popover: var(--card);
  --color-popover-foreground: var(--foreground);
  --color-primary: var(--primary);
  --color-primary-foreground: var(--primary-foreground);
  --color-primary-hover: var(--primary-hover);
  --color-primary-press: var(--primary-press);
  --color-primary-border: var(--primary-border);
  --color-secondary: var(--muted);
  --color-secondary-foreground: var(--foreground);
  --color-muted: var(--muted);
  --color-muted-foreground: var(--muted-foreground);
  --color-accent: var(--muted);
  --color-accent-foreground: var(--foreground);
  --color-destructive: var(--destructive);
  --color-success: var(--success);
  --color-border: var(--border);
  --color-input: var(--border-strong);
  --color-ring: var(--ring);
  --color-chart-1: var(--chart-1);
  --color-chart-2: var(--muted-foreground);
  --color-chart-3: var(--border-strong);
  --color-chart-track: var(--chart-track);
  --color-sidebar: var(--card);
  --color-sidebar-foreground: var(--foreground);
  --color-sidebar-primary: var(--primary);
  --color-sidebar-primary-foreground: var(--primary-foreground);
  --color-sidebar-accent: var(--muted);
  --color-sidebar-accent-foreground: var(--foreground);
  --color-sidebar-border: var(--border);
  --color-sidebar-ring: var(--ring);
  --font-sans: var(--typeface-text);
  --font-heading: var(--typeface-display);
  --font-mono: var(--typeface-code);
  --radius-control: var(--control-radius);
  --radius-panel: var(--panel-radius);
  --shadow-panel: var(--panel-shadow);
}
```

### Color rules

- Marigold appears only as a fill: primary buttons, the brand tile, the mascot, and the light beam. Data stays neutral.
- Never use `text-primary` or `decoration-primary`. Marigold text on a light surface is about 1.9:1. Text on marigold is always `primary-foreground`.
- Text colors are `foreground`, `muted-foreground`, `destructive`, `success`, and `primary-foreground` on marigold. Use no other text color.
- On a `muted` fill, use `foreground` text.
- `--border` is decorative. Never use it as the only boundary of a control. Controls use `border-input`, which is `--border-strong`.
- A status never depends on color alone. Each status has an icon shape and words (section 6).
- `chart-1` to `chart-3` are the only data colors. They are all neutral navy-slate.

### Verified contrast pairs (WCAG 2.x relative luminance, computed)

| Pair                                | Light | Dark | Requirement |
| ----------------------------------- | ----- | ---- | ----------- |
| foreground on background            | 15.3  | 14.9 | 4.5         |
| foreground on card                  | 16.5  | 13.3 | 4.5         |
| foreground on muted                 | 14.2  | 11.4 | 4.5         |
| muted-foreground on background      | 6.8   | 8.2  | 4.5         |
| muted-foreground on card            | 7.3   | 7.4  | 4.5         |
| primary-foreground on primary       | 8.7   | 8.7  | 4.5         |
| primary-foreground on primary-hover | 7.6   | 9.8  | 4.5         |
| primary-foreground on primary-press | 6.6   | 7.6  | 4.5         |
| destructive on background           | 6.1   | 7.7  | 4.5         |
| destructive on card                 | 6.6   | 6.9  | 4.5         |
| success on background               | 5.7   | 10.0 | 4.5         |
| success on card                     | 6.1   | 8.9  | 4.5         |
| ring on background                  | 5.4   | 8.0  | 3.0         |
| ring on card                        | 5.8   | 7.2  | 3.0         |
| ring on muted                       | 5.0   | 6.1  | 3.0         |
| border-strong on background         | 3.4   | 4.1  | 3.0         |
| border-strong on card               | 3.7   | 3.6  | 3.0         |
| primary-border on background        | 3.9   | n/a  | 3.0         |
| chart-1 on chart-track              | 8.9   | 7.6  | 3.0         |

## 4. Typography

### Font loading

Load fonts once, in `src/styles/fonts.ts`, with `next/font/google`. Read the Next.js 16 `next/font` docs in `node_modules/next/dist/docs/` first.

- Display: **Bricolage Grotesque**, variable, `axes: ['opsz']`, `variable: '--font-bricolage-grotesque'`, `display: 'swap'`. Use weights 600 to 700.
- Text: **Instrument Sans**, variable, `variable: '--font-instrument-sans'`, `display: 'swap'`. Use weights 400 to 600.
- Export `fontVariables`: the two `.variable` class names, joined with a space.
- `RootLayout` sets `className={cn('h-full', 'antialiased', fontVariables)}` on `<html>`. The loader variables must sit on `<html>`, because `typography.css` resolves them on `:root`.
- Remove `Inter`, `Geist`, `Geist_Mono`, the `--font-geist-*` variables, and the `font-sans` class from `layout.tsx`. Delete `src/components/home/fonts.ts`.
- The code role uses a system monospace stack, not a webfont. Its consumers are the JSON rules textarea and native `code`, `kbd`, `samp`, and `pre`.

### Base roles (`typography.css`)

```css
:root {
  --typeface-display: var(--font-bricolage-grotesque), 'Trebuchet MS', system-ui, sans-serif;
  --typeface-text: var(--font-instrument-sans), system-ui, sans-serif;
  --typeface-code: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
}

@layer base {
  html {
    font-family: var(--typeface-text);
  }
  body {
    font-size: 1rem;
    line-height: 1.6;
  }
  h1,
  h2,
  h3,
  h4 {
    font-family: var(--typeface-display);
  }
  h1 {
    font-size: clamp(1.875rem, 1.4rem + 1.6vw, 2.5rem);
    font-weight: 650;
    line-height: 1.12;
    letter-spacing: -0.02em;
    text-wrap: balance;
  }
  h2 {
    font-size: 1.25rem;
    font-weight: 600;
    line-height: 1.3;
    letter-spacing: -0.01em;
    text-wrap: balance;
  }
  h3,
  h4 {
    font-size: 1rem;
    font-weight: 600;
    line-height: 1.4;
  }
  code,
  kbd,
  samp,
  pre {
    font-family: var(--typeface-code);
  }
}
```

The base roles give every screen the same heading family and scale. Homepage CSS modules are unlayered, so their sizes below override the base roles. Application code removes its own heading size and weight utilities (`text-3xl`, `text-2xl`, `text-4xl`, `text-xl`, `font-semibold`, `tracking-tight`) and uses the base roles.

### Type roles

- Homepage display (hero h1): Bricolage, `clamp(2.5rem, 1.6rem + 3.2vw, 4rem)`, line height 1.04, weight 650, tracking -0.03em.
- Page title (application h1) and homepage h2: Bricolage, `clamp(1.875rem, 1.4rem + 1.6vw, 2.5rem)`, 1.12, 650, -0.02em.
- Title (application h2, panel titles): Bricolage, 1.25rem, 1.3, 600, -0.01em.
- Subtitle (application h3): Bricolage, 1rem, 1.4, 600.
- Wordmark: Bricolage, 1.25rem next to a 28px tile and 1.125rem next to a 24px tile, 1, 700, -0.02em.
- Lead (homepage): Instrument, 1.1875rem (1.125rem below 768px), 1.6, 400.
- Body, links, buttons, and inputs: Instrument, 1rem, 1.6 (buttons 1.25), 400 (buttons 600).
- Field label: Instrument, 0.875rem, 1.5, 500.
- Small (minimum): Instrument, 0.875rem, 1.5, 400 to 500.
- Score figure (homepage): Bricolage, 1.5rem, 1, 600, -0.01em.
- Code (JSON rules): system monospace, 0.875rem, 1.5, 400.

Type rules:

- 14px is the floor for every visible string, including captions, legal text, ranks, pills, and labels. Do not use `text-xs` or any size below `0.875rem`.
- Use `tabular-nums` on scores, ranks, and counts. Do not add a mono font for numbers.
- Tables and lists use the text family. Only headings and the wordmark use the display family. Code and the JSON editor keep the code family.
- Use sentence case everywhere. Balance h1 and h2 only.
- Measure: homepage h1 max 15ch, lead max 52ch, body max 60ch.

## 5. Space, layout, elevation, and focus

### Space and layout

- The spacing grid is 4px. Use only these steps: 4, 8, 12, 16, 24, 32, 48, 64, 96, 128. In Tailwind these are `1`, `2`, `3`, `4`, `6`, `8`, `12`, `16`, `24`, `32`. The 20px mobile gutter is the only exception.
- Breakpoints are 768px and 1024px. In Tailwind, use `md:` and `lg:` only. Do not use `sm:`, `xl:`, or `2xl:`.
- Gutters are 20px below 768px, 32px from 768px, and 48px from 1024px.
- Application pages are narrow (`max-w-3xl`) for `/organizations` and `/motion`. They are wide (`max-w-5xl`) for `/organizations/[login]`.
- Application page block padding is 48px below 768px and 64px from 768px. Page sections are 48px apart. Space inside a section is 16px to 24px.
- Review widths: 390, 768, 1024, and 1440.
- Touch targets: 44px minimum height and width, 8px minimum gap.
- Homepage layout values are in section 9.

### Radii

- `--control-radius`: 10px. Tailwind `rounded-control`. Buttons, inputs, segments, and list rows.
- `--panel-radius`: 18px. Tailwind `rounded-panel`. Panels.
- `--pill-radius`: 999px. Tailwind `rounded-full`. Pills, timeline nodes, and bars.
- Mark tile: 7.5 / 32 of the tile size. Brand tile only.
- Remove `--radius` and the `--radius-sm` to `--radius-4xl` scale. Use no other radius utility. A shadcn component that you add later must use these names.

### Elevation and borders

- Level 0: the page, headers, the timeline, and the account strip. No border and no shadow.
- Level 1: panels. Border `1px solid var(--border)`, `--card` fill, and `--panel-shadow`. In Tailwind, use `panelVariants()` (section 6).
- Empty states use a dashed `border-input` border, no fill, and no shadow.
- No other shadows. No glass and no blur.

### Focus and native rules (`base.css`)

```css
@layer base {
  *,
  ::before,
  ::after {
    border-color: var(--border);
  }
  body {
    background-color: var(--background);
    color: var(--foreground);
  }
  :focus-visible {
    outline: 2px solid var(--ring);
    outline-offset: 3px;
  }
  :is(main, section)[id] {
    scroll-margin-top: 24px;
  }
  input:is([type='checkbox'], [type='radio']) {
    accent-color: var(--foreground);
  }
  summary {
    cursor: pointer;
    padding-block: 10px;
    font-weight: 600;
  }
}
```

- This is the only focus style. Remove `outline-ring/50` from the old base rule. Remove `outline-none`, `focus-visible:ring-*`, and `focus-visible:outline-*` utilities from components.
- Use `:focus-visible` only. Never remove an outline.
- The mascot button keeps its 4px offset and 24px radius.
- Rows inside a clipped panel use `focus-visible:-outline-offset-4`, so the panel does not clip the ring.
- No element is sticky, so nothing hides the focus ring.

## 6. Controls and shared primitives

All primitives live in `src/components/ui` and export through its barrel. App code uses `Button` or `LinkButton`. App code never applies `buttonVariants` to a raw element. `buttonVariants` leaves the barrel.

### Button (`button.tsx`)

Refine the existing `cva`. Keep `@base-ui/react/button`.

- Base: `inline-flex shrink-0 cursor-pointer touch-manipulation items-center justify-center gap-2 rounded-control border border-transparent text-base leading-tight font-semibold whitespace-nowrap select-none transition-control motion-safe:active:translate-y-px disabled:pointer-events-none disabled:opacity-50 [&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-5`
- Variant `default` (primary): `border-primary-border bg-primary text-primary-foreground hover:bg-primary-hover active:bg-primary-press`
- Variant `outline` (secondary): `border-input bg-transparent text-foreground hover:bg-muted`
- Variant `link` (text link): `border-0 font-normal text-foreground underline decoration-1 underline-offset-4 hover:decoration-2 motion-safe:active:translate-y-0`
- Size `default`: `h-11 px-4` (44px). Size `lg`: `h-12 px-5` (48px).
- Compound variant: `{ variant: 'link', class: 'px-0' }`. The text link has no inline padding.
- Remove the `secondary`, `ghost`, and `destructive` variants. Remove the `xs`, `sm`, `icon`, `icon-xs`, `icon-sm`, and `icon-lg` sizes. Remove the old focus, `aria-expanded`, and `aria-invalid` classes.
- Every button has a visible boundary or an underline at rest.
- Use one primary button per region. Do not append arrows or other glyphs to labels.
- Hover changes color only. Disabled controls use 50% opacity and keep their label.

### LinkButton (`link-button.tsx`)

- Props: anchor props without `href`, `VariantProps<typeof buttonVariants>`, `href: string`, and `native?: boolean`.
- It renders `next/link` by default. With `native`, it renders a plain `<a>`. Use `native` for `/api/v1/auth/*`, because those routes must leave the app with a full navigation.
- Classes: `cn(buttonVariants({ variant, size }), className)`.
- It replaces `home/button-link.tsx`. The old `primary` is `size="lg"`. The old `secondary` is `variant="outline" size="lg"`. The old compact `secondary` is `variant="outline"`. The old `text` is `variant="link"`.

### Form primitives

- `control.ts` exports `controlClassName`: `min-h-11 w-full rounded-control border border-input bg-card px-3 py-2 text-base leading-normal text-foreground placeholder:text-muted-foreground transition-control disabled:cursor-not-allowed disabled:opacity-50 aria-invalid:border-destructive`.
- `input.tsx` exports `Input`. It renders a native `<input>`, takes `ComponentProps<'input'>`, and merges `controlClassName` with `className`.
- `textarea.tsx` exports `Textarea`. It renders a native `<textarea>` with `controlClassName`. Callers add `font-mono text-sm` for code.
- `native-select.tsx` exports `NativeSelect`. It renders `<span className="relative block">`, a native `<select>` with `controlClassName` plus `appearance-none pr-10`, and a 20px chevron SVG. The chevron is `aria-hidden`, path `m6 8 4 4 4-4`, stroke 2, round caps, `currentColor`, `pointer-events-none`, 12px from the right, and vertically centered.
- `field.tsx` exports `Field`. Props: `label`, `className`, and `children`. It renders `<label className="grid gap-2">`, then `<span className="text-sm font-medium">` with the label, then the control.
- `checkbox-field.tsx` exports `CheckboxField`. Props: `label` and input props without `type`. It renders `<label className="flex min-h-11 cursor-pointer items-center gap-3">`, a 20px native checkbox, then the label text.
- Labels wrap their controls, as today. Visible label text and order do not change.
- Native controls keep native behavior and keyboard support. `color-scheme` styles native pickers and menus in both themes.
- An invalid control sets `aria-invalid` and gets a `destructive` border. Its message uses `StatusMessage` with tone `error`.

### StatusMessage (`status-message.tsx` and `status-icon.tsx`)

- Props: `role` (`'status'` or `'alert'`, required), `tone` (`'neutral'`, `'success'`, or `'error'`, default `'neutral'`), `className`, and `children`.
- It renders `<p role={role} className="flex items-start gap-2">`, a 20px `StatusIcon` with `mt-0.5 shrink-0`, and the text in a `<span>`. The text content does not change.
- The caller sets `role`. The role never comes from the tone, so every existing `status` and `alert` stays as it is.
- Tone `neutral`: text `text-foreground`, icon `text-muted-foreground`. The icon is the commit node: paths `M2 10h4.5` and `M13.5 10H18`, and a circle at (10, 10) with r 3.5.
- Tone `success`: text and icon `text-success`. The icon is a circle with r 8 and the check `m6.5 10.2 2.4 2.4 4.6-5`.
- Tone `error`: text and icon `text-destructive`. The icon is a circle with r 8, the line `M10 6v4.5`, and the dot `M10 13.6v.01`.
- Icons use `viewBox="0 0 20 20"`, stroke 2, round caps, no fill, `aria-hidden="true"`, and `focusable="false"`.
- No spinner and no animation in status messages.

### Panels (`panel.ts`)

- `panelVariants` is a `cva` with the base `rounded-panel border`.
- Variant `tone`: `raised` (default) is `border-border bg-card shadow-panel`. `dashed` is `border-dashed border-input`.
- Variant `padding`: `default` is `p-4 md:p-6`. `none` is `overflow-hidden`.
- Apply `panelVariants` to the semantic element (`section`, `form`, `ul`, or `div`). Do not add a `Panel` wrapper component.

### Toaster (`sonner.tsx`)

Keep the `next-themes` wiring. Pass `style` with `--normal-bg: var(--card)`, `--normal-text: var(--foreground)`, `--normal-border: var(--border-strong)`, `--border-radius: var(--control-radius)`, and `fontFamily: 'var(--typeface-text)'`. No toast consumer exists today.

### Page layout (`src/components/layout`)

- `page-shell.tsx` exports `PageShell`. Props: `width` (`'narrow'` or `'wide'`, default `'narrow'`), `className`, and `children`. It renders `<main>` with `mx-auto w-full space-y-12 px-5 py-12 md:px-8 md:py-16 lg:px-12`, plus `max-w-3xl` or `max-w-5xl`.
- `page-header.tsx` exports `PageHeader`. Props: `lead` (the link above the title), `title`, and an optional `description`. It renders `<header className="space-y-4">`, the lead, the `h1`, and a `max-w-[60ch] text-muted-foreground` paragraph.
- `index.ts` is the barrel.
- Application pages get no skip link, because no navigation comes before `<main>`.

## 7. Brand mark

### Concept

An original seated cat whose tail rises as a git line and ends in a commit ring. The mascot's tail forks into a branch with two commit rings.

Distinctness rules. They are required, because the mark must not suggest GitHub affiliation:

- Never put the cat in a circle. Never use a knockout cat silhouette in a disc.
- No tentacles, no octopus limbs, no Octocat proportions. Do not trace or adapt any GitHub artwork.
- Use a wide squircle head, round-tipped ears, vertical capsule eyes, a visible seated body, and the branch tail.
- The mascot's right ear (viewer's right) has a small notch.
- No GitHub logo path exists in the code. Show "GitHub" as text where needed.

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
- The homepage nav, the footer, the favicon, the apple icon, and the `/organizations` header use the tile variant. This gives the same marigold recognition in both themes. Ink on marigold is 8.7:1.
- Clear space: 25% of the tile size on all sides. The wordmark sits 10px from the tile.
- Sizes: homepage nav and `/organizations` header tile 28px, footer tile 24px, favicon 32px viewBox, apple icon 180px.
- Minimum size: 16px. Below 20px, keep the drawing the same. Do not add detail at larger sizes.

### BrandLink (`src/components/brand/brand-link.tsx`)

- It replaces `home/home-brand.tsx`. Props: `size` (`24` or `28`).
- It renders a `next/link` to `/` with `BrandMark` (hidden from assistive technology) and the text "Gitorgasm". The accessible name is "Gitorgasm".
- Move `.brandLarge` and `.brandSmall` from `home.module.css` to `brand/brand-link.module.css`. Use `--typeface-display` and the wordmark role. Minimum height 44px.
- Consumers: the homepage navigation (28), the homepage footer (24), and the `/organizations` header (28).

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

### Illustration style (all graphics)

- Flat vector, round caps and joins, one stroke weight per graphic.
- Commit node: a ring on a line. It is the only motif. Do not add other git glyphs.
- No emoji. No text glyphs such as `↗ → ← ⌄ ✓ ↻`. Use small inline SVG only where the shape carries meaning.

## 8. Motion

Define these tokens on `:root` in `src/styles/motion.css`. Keep `src/lib/motion.ts` unchanged. The mascot uses CSS keyframes, because they run from server HTML before hydration.

| Token                     | Value                              | Use                           |
| ------------------------- | ---------------------------------- | ----------------------------- |
| `--motion-press`          | 90ms                               | Active translate              |
| `--motion-hover`          | 160ms                              | Color changes                 |
| `--motion-state`          | 240ms                              | Panel and segment state       |
| `--motion-wave`           | 1600ms                             | One full wave                 |
| `--motion-ease-out`       | `cubic-bezier(0.16, 1, 0.3, 1)`    | Arrivals (same as `EASE.out`) |
| `--motion-ease-in-out`    | `cubic-bezier(0.65, 0, 0.35, 1)`   | Oscillation and return        |
| `--motion-ease-overshoot` | `cubic-bezier(0.34, 1.4, 0.64, 1)` | Paw raise                     |

The old `--ease-*` names collide with the Tailwind `--ease-*` namespace on `:root`. Rename every consumer, including `mascot.module.css`.

### Shared motion rules

```css
@media (prefers-reduced-motion: reduce) {
  :root {
    --motion-press: 0ms;
    --motion-hover: 0ms;
    --motion-state: 0ms;
  }
}

@utility transition-control {
  transition:
    color var(--motion-hover),
    background-color var(--motion-hover),
    border-color var(--motion-hover),
    text-decoration-color var(--motion-hover),
    translate var(--motion-press);
}
```

- Every control uses `transition-control` in Tailwind code, or the same token-based transition in CSS modules. Reduced motion then removes control transitions everywhere.
- The press translate uses `motion-safe:active:translate-y-px`, so reduced motion has no translate.
- Framer Motion keeps `MotionConfig reducedMotion="user"` in `MotionProvider`.

### Wave choreography (`arm-wave`, one run, `animation-fill-mode: both`)

| Keyframe | Arm rotation | Timing function into next |
| -------- | ------------ | ------------------------- |
| 0%       | 0deg         | `--motion-ease-overshoot` |
| 20%      | 150deg       | `--motion-ease-in-out`    |
| 32%      | 132deg       | `--motion-ease-in-out`    |
| 44%      | 165deg       | `--motion-ease-in-out`    |
| 56%      | 132deg       | `--motion-ease-in-out`    |
| 68%      | 160deg       | `--motion-ease-in-out`    |
| 80%      | 150deg       | `--motion-ease-in-out`    |
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

### Motion budget

- Homepage: at most two animated elements per view. They are the mascot (once) and LightRays (ambient, desktop only).
- Application screens: no ambient or entrance motion. Only control color transitions and the press translate move.
- `/motion` demonstrates the Framer Motion primitives. Its examples keep their motion. It gets no mascot and no beam.
- Outside `/motion`: no section entrance animations, no stagger, no scroll reveals, and no card hover lifts.

### React Bits decisions

- **LightRays: keep, homepage only.** It is the sunbeam that the cat sits in. Set `raysColor` from the computed `--primary`, origin `top-right`, `raysSpeed` 0.08, `opacity` 0.22 light and 0.3 dark. Keep `followMouse`, `cursorShadow`, `clickPulse`, and `intro` off. Keep the current gating and CSS radial fallback. The fallback uses `--primary` at 10%.
- **SpotlightCard: keep on the homepage scoreboard panel only.** Set `spotlightColor` to `var(--primary)`, intensity 0.06, and keep the current flags. Its palette uses `--card` and `--border`.
- **DotGrid: removed.** Do not add it back.
- Do not install other React Bits or packages.

## 9. Homepage structure and final copy

The homepage composition is homepage-only. Application screens never show the beam, the mascot, the timeline, or the illustrative panels.

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

Homepage layout: container max width 1200px. Hero padding block is 96px on desktop, and 56px top and 64px bottom below 768px. Section padding block is 112px on desktop and 72px below 768px. The nav is 72px high and not sticky. Homepage modules keep the current `max-width: 767px` and `max-width: 1023px` query style.

### Navigation (`home-navigation.tsx`)

- Left: `BrandLink` at 28px.
- Right: `LinkButton variant="link"` "How it works" (`#how-it-works`). Hide it below 480px, because the hero has the same link. Then `LinkButton variant="outline"` "Open app" (`/organizations`), 44px high.

### Hero (`hero/home-hero.tsx`)

- H1: **Your GitHub organization’s activity, scored daily.**
- Lead: **Connect an organization to see activity scores for its contributors and repositories. Choose which activity counts, pick a UTC date period, and save the views you check often.**
- Primary (`LinkButton size="lg"`): **Connect an organization** → `/organizations`
- Secondary (`LinkButton variant="outline" size="lg"`): **How it works** → `#how-it-works`
- Note (14px, muted): **Sign in with GitHub to start.**
- Product column: `<WavingMascot />`, then `<ContributionScoreboard />`. The column has `padding-top` equal to the mascot height above the ground line: 150px desktop, 113px mobile. The mascot is absolutely positioned, right inset 32px (20px mobile). Its ground line overlaps the panel's top border, and the paws overlap the border by about 5px. The mascot sits above the panel in z-order. In DOM order the mascot comes before the figure.

### Scoreboard (`scoreboard/*`)

- Toolbar: **example-org** and the pill **Illustrative data**.
- Title: **Contributor scores**. Meta: **Last 30 days, UTC**. Do not use middle dots.
- Rows (illustrative): 1 `ana-k` 92, 2 `jdoe` 71, 3 `mori` 58, 4 `kline` 44. Show ranks as plain numbers, not zero-padded.
- Row height 56px. Names 16px. Ranks 14px muted. Scores use the score figure style. Bars are 6px high in `--chart-1` on `--chart-track`, radius pill.
- Repository block: **Repository activity**, **example-org/core**, sparkline in `--chart-1`.
- Panel footer: **Scores refresh daily.**
- Figcaption: **Sample figures for illustration. Not live data.**
- Panel padding 24px (16px below 768px). No column header row.

### How it works (`features/home-features.tsx`)

- H2: **How it works**
- Intro: **Three steps from a GitHub organization to scores you can return to.**
- Markup: `<ol>`. Each `<li>` has a 32px node (a ring with 2px `--border-strong` border, `--card` fill, and the step number at 14px/600), an h3, and a paragraph.
- The line is 2px `--border-strong`, behind the nodes. It is horizontal from 1024px with three equal columns. Below 1024px it is vertical on the left, with the text 48px from the line.
  1. **Connect an organization**: **Sign in with GitHub and install the GitHub App on your organization. Gitorgasm shows the contributors and repositories that the app can access.**
  2. **Choose what counts**: **Pick an activity type and a UTC date period. Hide bots if you want, then save the combinations you check often as presets.**
  3. **Come back to fresh scores**: **Scores update daily, so each visit reflects recent activity in your organization.**
- No panels and no shadows in this section.

### Scope (`scope/home-scope.tsx`)

- H2: **Decide what each score includes.**
- Paragraph: **Filter by code, reviews, or issues. Set an exact UTC window, or use a preset period such as the last 7 or 30 days.**
- Boundary note (16px, muted): **Scores summarize recorded GitHub activity. They do not measure productivity or the value of anyone’s work.**
- Action: `LinkButton size="lg"` **Connect an organization** → `/organizations`. The action keeps the same name as in the hero.
- Filter panel (level 1, static picture, `aria-hidden` on the decoration):
  - Header: **Activity filters** and the pill **Illustrative view**
  - **Activity**: a segmented row **All, Code, Reviews, Issues** with **Reviews** selected (`--muted` fill, 1px `--border-strong`, 600 weight). The real control is single-select, so exactly one segment is selected.
  - **Time period (UTC)**: **Last 30 days**, with an inline SVG chevron
  - **Hide bots**: an SVG checkbox in the checked state (`--chart-1` fill, `--card` check)
  - **Saved preset**: **Monthly review**, with the existing bookmark SVG
  - Figcaption: **Example filter settings, not a working form.**
- All text in the panel is 14 to 16px.

### Footer (`footer/home-footer.tsx`)

- `BrandLink` at 24px.
- **Activity scores for GitHub organizations.**
- Links (`LinkButton variant="link"`): **How it works** (`#how-it-works`), **Organizations** (`/organizations`), **GitHub** (`https://github.com/upayanmazumder/gitorgasm`).
- Bottom row (14px, muted): **Gitorgasm is not affiliated with GitHub.** and **© 2026 Gitorgasm**.
- No mascot, no motion in the footer.

### Metadata

- `src/app/page.tsx` exports page metadata. Description: **Activity scores for the contributors and repositories in your GitHub organization.** Do not change the root layout metadata.

## 10. Homepage implementation notes

These notes record how the approved homepage maps to this file. The global migration changes only token names, font loading, and the shared link and brand components.

- Mascot coordinates and keyframes follow sections 7 and 8 without change. A rest, 132, 150, and 165 degree render check showed the paw inside the view box.
- Pivot change: `arm-wave` and `head` use a translate, animate, translate-back wrapper (`mascot-pivot.tsx`) instead of `transform-origin: 52px 100px; transform-box: view-box`. The origin stays in local user space, so the result does not depend on how a browser places the reference box for a viewBox that starts at x = -16.
- `BrandMark` takes an optional `title`. Without it the mark is `aria-hidden`. Shared geometry lives in `brand/mark-paths.ts`, and `icon.svg` repeats it with literal colors.
- The glyph groups (not the tile rectangle) sit inside `translate(-1.4 0)` in `BrandMark`, `icon.svg`, and `apple-icon.tsx`, because the geometry spans x 5 to 29.7. `apple-icon.tsx` uses a plain `0 0 32 32` viewBox with the same transform.
- `apple-icon.tsx` uses a full-bleed marigold square, not a rounded tile, because iOS applies its own corner mask.
- Renames: `ContributionField` is `SunbeamField`, and `SignalSurface` is `SpotlightPanel`. The old names belonged to the removed direction.
- `home/button-link.tsx` and `home/home-brand.tsx` were the first shared homepage helpers. The global migration replaces them with `LinkButton` in `ui` and `BrandLink` in `brand`.
- Mascot parts are split into `mascot-body`, `mascot-head`, `mascot-limb`, and `mascot-art`. `MascotLimb` draws both the static leg and the waving arm.
- The wave animation rules sit inside `prefers-reduced-motion: no-preference`. The static greeting pose sits inside `prefers-reduced-motion: reduce`. The React hook only decides the semantics (button or image).
- The scoreboard title is an `h2`, because it follows the page `h1` directly. The filter panel title is a `p`, because the panel is `aria-hidden` decoration.
- The checked "Hide bots" box uses `--chart-1`, so marigold stays limited to the places in section 3.
- `SunbeamField` is the first child of the page root in `HomeView`, not of the hero. The page root is `position: relative`, `isolation: isolate`, and `overflow-x: clip`. The field is full bleed (`top`, `right`, and `left` at 0, no `100vw`) with a fixed height of 680px (780px from 768px wide), so it spans the navigation and the hero. A bottom mask and a left mask fade the beam, so no side shows a clipping edge. The field has `pointer-events: none` and `z-index: -1`. The header sits above it, so the beam never tints nav text.
- `WavingMascot` starts the first wave only when `entry?.isIntersecting && entry.intersectionRatio >= VISIBLE_SHARE` (0.6). The guard makes the intended condition explicit. A first-paint check at 27% visibility and a scroll check at 25% did not start the wave early, so this is not a reproduced bug.
- Headless browsers report `hover: none` and `pointer: coarse`, so they use the CSS radial fallback. An emulated fine-pointer desktop rendered a real 404 by 483 LightRays canvas with no browser errors. Do not raise the ray opacity based on fallback screenshots.
- The timeline text sits 64px from the node's left edge, which is 48px from the line center.

### Opus 5.5 visual review

Opus 5.5 reviewed screenshots of the implementation and approved the design after four fixes: a full-bleed beam without a hard edge at the content boundary, an explicit 60% visibility guard, a real `favicon.ico` from the new mark, and a centered tile glyph. Opus asked to keep the cat and its wave, the headline wrapping, the type and palette, the score and rank positions, the sparkline weight, the section rhythm, the card-free timeline, the footer affiliation line, the static reduced-motion image, the loading fallback, and the React Bits provenance. Opus asked for no other polish. The global migration must keep all of these.

## 11. Application screens

Keep every label, role, heading level, element order, route path, and handler. Keep feature state and hooks in their feature folders. Change presentation only.

### `/organizations`

- `organizations-view.tsx`: `PageShell` (narrow) replaces the raw `<main>`.
- `organization-header.tsx`: `PageHeader` with `lead={<BrandLink size={28} />}`, the title "Your organizations", and the existing description. The "Gitorgasm" link keeps its name, target, and position.
- `authorization/organization-authorization.tsx`: "Checking your session…" is a neutral `StatusMessage role="status"`.
- `authorization/session-error.tsx`: the message is `StatusMessage role="alert" tone="error"`. "Try again" stays `Button variant="outline"`.
- `authorization/sign-in-section.tsx`: the `section` uses `panelVariants()` and `space-y-4`. The h2 uses the base role. "Sign in with GitHub" is a primary `LinkButton native` to `/api/v1/auth/login`.
- `account/account-section.tsx`: a level 0 strip with `space-y-6 border-y border-border py-6`. "Sign out" stays `Button variant="outline"`. "Signing out…" is a neutral `StatusMessage role="status"`. The sign-out failure is `StatusMessage role="alert" tone="error"`.
- `account/access-actions.tsx`: "Install GitHub App" is a primary `LinkButton native`. "Refresh GitHub access" is `LinkButton native variant="outline"`. Keep the muted help text at 14px.
- `directory/organizations-section.tsx`: "Check organizations" becomes `Button variant="outline"`. Loading is a neutral `StatusMessage role="status"`. The load failure is `StatusMessage role="alert" tone="error"`.
- `directory/organization-list.tsx`: the `ul` uses `panelVariants({ padding: 'none' })` and `divide-y divide-border`. Each row link uses `block space-y-1 p-4 md:p-6 transition-control hover:bg-muted focus-visible:-outline-offset-4`. The login is `font-semibold`. The meta line is `text-sm text-muted-foreground` and keeps its text.
- `directory/empty-organizations.tsx`: `panelVariants({ tone: 'dashed' })` with `space-y-2`. The h3 uses the base role.

### `/organizations/[login]`

- `scores/scores-view.tsx`: `PageShell width="wide"`. `PageHeader` with `lead={<LinkButton href="/organizations" variant="link">Organizations</LinkButton>}` and the title `` `${login} scores` ``. Statuses stay after the header, in the same order. "Loading scores…" is neutral. The daily and calculation errors use `role="alert"` and tone `error`. The `message` status is neutral with `role="status"`.
- `form/score-form.tsx`: the `form` uses `panelVariants()` with `grid gap-4 md:grid-cols-2`. The submit control is a primary `Button type="submit"` with `md:col-span-2 md:justify-self-start`. Keep the disabled logic, the pending logic, and the labels.
- `form/fields/form-fields.ts`: remove `inputClassName`. `textField` returns `value` and `onChange` only.
- `form/fields/index.ts`: stop exporting `inputClassName`.
- `form/fields/period-fields.tsx`: each label becomes `Field`. The selects become `NativeSelect`. The date inputs become `Input` with `type="datetime-local"` and `required`.
- `form/fields/filter-fields.tsx`: the text and number inputs become `Field` with `Input`. "Hide bots" becomes `CheckboxField`. The `details` element gets `md:col-span-2`, and its summary uses the base rule. The JSON field is `Field` (`mt-2`) with `Textarea rows={6} className="font-mono text-sm"`.
- `presets/presets-panel.tsx`: the `section` uses `panelVariants()` and `space-y-4`. Use `Field` with `NativeSelect` and `Input` from `@/components/ui`. "Save preset" and "Remove preset" become `Button type="button" variant="outline"` inside `flex flex-wrap gap-3`.
- `results/score-results.tsx`: the freshness line is `StatusMessage role="status"`. Its tone is `success` when fresh, `error` when ingestion failed, and `neutral` otherwise. Each `Board` section uses `panelVariants()` and `space-y-4`. Rows use `flex items-baseline justify-between gap-4 border-b border-border py-3 last:border-b-0`. The score `strong` gets `font-semibold tabular-nums`. "No activity in this period." uses `text-muted-foreground`.

### `/motion`

- `motion-demo.tsx`: `PageShell` (narrow) with `className="space-y-24"`. The page gains a `main` landmark.
- `demo-header.tsx`: `<Fade>` wraps `PageHeader` with `lead={<LinkButton href="/" variant="link">Back home</LinkButton>}`, the title "Motion", and the existing description. Remove the `←` glyph, as section 7 requires.
- `section.tsx`: the h2 uses the base role. Remove its size, weight, and tracking utilities. Keep the 14px muted hint.
- `card.tsx`: `Card` applies `cn(panelVariants(), className)`.
- `gesture-examples.tsx`: both examples use `panelVariants()` with `text-center`. Keep `cursor-pointer` on "Hover me", the `m.button` for "Tap me", and both gestures.
- `reveal-examples.tsx`: stagger labels use `text-center text-sm text-muted-foreground tabular-nums`. Remove `font-mono`. Use `md:grid-cols-3`.
- `presence-example.tsx`: no change. It already uses `Button variant="outline"`.

### Unmatched 404

Next.js injects unlayered body styles in its default 404 page. They override the `@layer base` page colors. A custom root `not-found.tsx` replaces that page, so the injection does not appear.

- `src/app/not-found.tsx`: a thin Server Component. It exports `metadata` with the title "Page not found" and no description. It renders `NotFoundView` only.
- `components/not-found/not-found-view.tsx` and `index.ts`: the named `NotFoundView` export, through the barrel.
- Composition: `PageShell` (narrow, default `space-y-12`), then `PageHeader` with `lead={<BrandLink size={28} />}`, the title "Page not found", and the description "No page exists at this address. Check the link, or go to your organizations." Then a `div` with one default primary `LinkButton` to `/organizations`, labeled "Go to your organizations". The brand link goes to `/`.
- One `main` and one `h1`. The page gets fonts and theme from `RootLayout`.
- Add no class, panel, mascot, beam, animation, 404 numeral, status line, secondary button, token, color, or variant.
- Never use `global-not-found`, experimental config, `!important`, raw `body` overrides, `html` or `body` tags, `next/dist` imports, or a client component.
- Add no permanent test for this route.

## 12. Acceptance

### Code gates

Run each search over `web/src`, without `components/react-bits`. Each search must return no match.

- `--home-`, a `.home` selector, and a `home` class name on the homepage root.
- `Geist`, `Inter(`, `--font-geist`, `--font-home`, and the file `components/home/fonts.ts`.
- `\b(zinc|gray|slate|neutral|stone)-[0-9]`, `text-white`, `text-black`, `bg-white`, and `bg-black`.
- `text-xs`, and any `text-[` size below `0.875rem`.
- `rounded`, `rounded-sm`, `rounded-md`, `rounded-lg`, `rounded-xl`, `rounded-2xl`, `rounded-3xl`, and `rounded-4xl`. Only `rounded-control`, `rounded-panel`, and `rounded-full` are allowed.
- `text-primary` without the `-foreground` suffix.
- `buttonVariants` outside `src/components/ui`.
- `ButtonLink`, `HomeBrand`, and `inputClassName`.
- `outline-none`, `focus-visible:ring`, and `outline-ring`.
- Raw `<button`, `<input`, `<select`, or `<textarea` in `components/organizations` and `components/motion/demo`. The `m.button` in `gesture-examples.tsx` is the one exception.
- `var(--ease-out)`, `var(--ease-in-out)`, and `var(--ease-overshoot)`.
- `font-mono` outside the JSON textarea.
- Raw hex in components, other than `icon.svg` and `apple-icon.tsx`.

### Rendered views

Render each view in light and dark. Check 390, 768, 1024, and 1440 wide. Tab through each view and check the focus ring. Also check that a theme change does not animate control colors.

- `/`: compare with the approved screenshots. The fonts, palette, beam, cat, wave, scoreboard, timeline, filter panel, and footer look the same. Reduced motion shows the static greeting image.
- `/organizations`: session check, session error with "Try again", signed out (sign-in panel), signed in with organizations, no organizations, organization load error, signing out, and sign-out error.
- `/organizations/[login]`: loading scores, daily error, default form, custom period with date inputs, open "Custom scoring rules" with the JSON textarea, saved presets, a preset status message, calculation error, fresh results, failed results, incomplete results, and an empty board.
- Unmatched 404 (for example `/no-such-theme-route`): the HTTP status is 404. No default `next-error` body styles apply. The theme class overrides the opposite OS color scheme. The fonts load. One `main` and one `h1` exist. Tab order starts at the brand link, then the organizations button. Both targets are 44px or larger. Widths 390, 768, 1024, and 1440 all fit.
- `/motion`: every section, with and without reduced motion.

Use the local stack (`make up`) for real states. Report every state that you could not render. Do not report a view that you did not see.

### Checklist

Theme and type

- [ ] `globals.css` contains imports only. Tokens, mapping, type, motion, and base rules live in `src/styles`.
- [ ] Every route uses the same `:root` and `.dark` tokens. No route or component defines theme tokens.
- [ ] Headings on every route use Bricolage Grotesque. Body, controls, and tables use Instrument Sans. The JSON textarea uses the system monospace stack.
- [ ] The root layout loads only the two fonts. No Inter, Geist, or Geist Mono request exists.
- [ ] Native controls, date pickers, and scrollbars follow the theme.

Controls and states

- [ ] Every link and button is 44px or taller, with 8px or more between targets.
- [ ] Primary buttons are marigold with navy text. Secondary buttons have a 3:1 border. Text links use `foreground` with an underline. No marigold text exists.
- [ ] Every input, select, textarea, and checkbox has a visible 3:1 boundary and a visible label.
- [ ] The focus ring is the same 2px ring on every control in both themes.
- [ ] Every error and success message has an icon, words, and a status color. Each role stays `status` or `alert` as before.
- [ ] Reduced motion removes control transitions, the press translate, and the mascot wave.

Brand and homepage

- [ ] The mark reads as a cat with a commit-ring tail at 16, 24, and 28px, on light and dark backgrounds.
- [ ] No circle container, no tentacles, no GitHub artwork anywhere.
- [ ] The favicon and apple icon show the marigold tile mark.
- [ ] The SSR HTML contains the mascot in the rest pose with fixed `width` and `height`. CLS from the mascot is 0.
- [ ] The wave runs once when the mascot first becomes 60% visible, then stops in the rest pose.
- [ ] Click, tap, Enter, and Space replay the wave. Repeated presses restart it cleanly.
- [ ] With reduced motion: static greeting pose, `role="img"` with a label, no button, no animation.
- [ ] The homepage copy matches section 9 exactly. Both mock panels say they are illustrative.
- [ ] LightRays and the cat appear only on `/`. SpotlightCard appears only on the scoreboard.

Content and restraint

- [ ] Application copy, labels, roles, heading levels, element order, and routes are unchanged. The only exception is the removed `←` in "Back home".
- [ ] No testimonials, logos, pricing, customer claims, real metrics, or team-management features.
- [ ] No kicker labels, all-caps labels, mono labels, arrow glyphs, or emoji.
- [ ] Marigold appears only in the mark, the mascot, primary buttons, and the light beam.

## 13. Sonnet migration contract

Read this file and `web/AGENTS.md`. Read the Next.js 16 `next/font` docs in `node_modules/next/dist/docs/`. Use Context7 for the Tailwind CSS v4 docs on `@import`, `@theme inline`, and `@utility`.

### New files

- `src/styles/tokens.css`: section 3 raw tokens, mascot tokens, radii, and the panel shadow.
- `src/styles/theme.css`: the section 3 mapping block.
- `src/styles/typography.css`: the section 4 block.
- `src/styles/motion.css`: the section 8 tokens, the reduced-motion override, and `transition-control`.
- `src/styles/base.css`: the section 5 block.
- `src/styles/fonts.ts` and `src/styles/index.ts`: the section 4 font load and the `fontVariables` barrel.
- `src/components/ui/link-button.tsx`, `control.ts`, `input.tsx`, `textarea.tsx`, `native-select.tsx`, `field.tsx`, `checkbox-field.tsx`, `status-message.tsx`, `status-icon.tsx`, and `panel.ts`: section 6.
- `src/components/brand/brand-link.tsx` and `brand-link.module.css`: section 7.
- `src/components/layout/page-shell.tsx`, `page-header.tsx`, and `index.ts`: section 6.

### Changed files

- `src/app/globals.css`: replace the content with the section 2 imports.
- `src/app/layout.tsx`: use `fontVariables` from `@/styles`. Remove Inter, Geist, Geist Mono, and the `font-sans` class.
- `src/components/ui/button.tsx`: the section 6 variants and sizes.
- `src/components/ui/sonner.tsx`: the section 6 Toaster style.
- `src/components/ui/index.ts`: export `Button`, `LinkButton`, `Input`, `Textarea`, `NativeSelect`, `Field`, `CheckboxField`, `StatusMessage`, `panelVariants`, and `Toaster`. Do not export `buttonVariants`, `controlClassName`, or `StatusIcon`.
- `src/components/brand/index.ts`: export `BrandMark` and `BrandLink`.
- `src/components/home/home-view.tsx`: remove the `home` class and the font variable classes. Keep `styles.page`.
- `src/components/home/home-navigation.tsx`, `hero/home-hero.tsx`, `scope/home-scope.tsx`, and `footer/home-footer.tsx`: use `LinkButton` and `BrandLink`, as section 9 specifies.
- `src/components/home/home.module.css`: remove `.brandLarge` and `.brandSmall`. Rename tokens.
- Every homepage `*.module.css`, `effects/sunbeam-field.tsx`, `effects/spotlight-panel.tsx`, and `scope/filter-panel.tsx`: rename tokens with the section 3 table, the section 5 radii, and the section 8 motion tokens. `SunbeamField` reads `--primary`.
- `src/components/organizations/**` and `src/components/motion/demo/**`: section 11.
- `web/README.md`: replace the `.home`-only token, font, and "neutral theme" prose with the global theme. Replace the `ButtonLink` and `HomeBrand` sentence.
- `web/AGENTS.md`: add `src/styles`, `src/components/layout`, and the new `ui` files to the directory tree. Point the Tailwind token rule to `src/styles/tokens.css`.

### Deleted files

- `src/components/home/fonts.ts`
- `src/components/home/button-link.tsx`
- `src/components/home/button-link.module.css`
- `src/components/home/home-brand.tsx`

### Order

1. Build `src/styles`, `globals.css`, and `layout.tsx`. Then build the `ui`, `brand`, and `layout` primitives.
2. Migrate the homepage. Compare it with the approved screenshots before you continue.
3. Migrate `/organizations`, then `/organizations/[login]`, then `/motion`.
4. Update `web/README.md` and `web/AGENTS.md`.
5. Run the code gates. Then run `pnpm lint && pnpm typecheck && pnpm test && pnpm build && pnpm format:check` in `web`. Existing tests must pass without changes to their role or label queries.
6. Add one component test for `ScoreForm`. Pressing "Show scores" calls `onSubmit` once. The button is disabled while `pending` is true. The change to `Button` makes this a real regression risk.
7. Capture the section 12 rendered views. Report each view that you saw and each view that you could not render.

Ask Opus before you add a color, a font, an animation, a section, a variant, or a copy change.

## 14. Deliberate tradeoffs

- `--destructive` and `--success` are the only new color values. The application has real error and success states. Both values pass AA on `background` and `card` in both themes.
- The `useScoreControls` message mixes success and error text in one string. It renders as a neutral status, because the hook state has no kind and the hook stays feature-owned.
- The code role uses a system monospace stack, not a webfont. The JSON rules editor is its only feature consumer.
- The `ghost`, `secondary`, and `destructive` variants and the small sizes are removed. "Check organizations" moves from `ghost` to `outline`, so every control has a visible boundary and a 44px target.
- The raw `--chart-2` to `--chart-5` values are removed. `chart-2` and `chart-3` map to existing neutral tokens. No `chart-4` or `chart-5` exists, so data stays neutral.
- The shadcn radius scale is removed. A shadcn component that you add later must use `rounded-control`, `rounded-panel`, or `rounded-full`.
- Popover, secondary, accent, and sidebar utilities map to the raw `card`, `muted`, `foreground`, `primary`, `border`, and `ring` tokens. The palette has one value per role.
- Application copy stays unchanged. This includes the middle-dot meta text in the organization list, because it is part of the link names. Only the decorative `←` in "Back home" is removed.
- Application pages get no skip link, because no navigation comes before `<main>`. `/motion` gains a `main` landmark from `PageShell`.
