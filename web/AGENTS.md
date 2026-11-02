# web — Agent Guide

> Coding-agent reference for `web`, the Next.js web app in the Gitorgasm monorepo. Keep it current when adding tools, scripts, or conventions.

<!-- BEGIN:nextjs-agent-rules -->

## This is NOT the Next.js you know

This version has breaking changes — APIs, conventions, and file structure may all differ from your training data. Read the relevant guide in `node_modules/next/dist/docs/` (resolved from this file's directory; in monorepos the `next` package may not be visible from the repo root) before writing any code. Heed deprecation notices.

This block is written and re-added by `next dev` — verify at `node_modules/next/dist/server/lib/generate-agent-files.js`. Removing it from a diff only re-creates the uncommitted change; committing it with your work keeps the tree clean.

<!-- END:nextjs-agent-rules -->

---

## Project Overview

Next.js 16 (App Router) production-ready starter with:

- **Tailwind CSS v4** — utility-first styling, `@custom-variant dark` for class-based dark mode
- **Framer Motion** — animation primitives via `LazyMotion + domAnimation`
- **shadcn/ui** — component primitives via `@base-ui/react`
- **next-themes** — system/light/dark theme switching
- **TanStack Query v5** — async server state with SSR prefetch support
- **Axios** — HTTP client with typed error normalisation
- **Zustand v5** — client state with auto-selector helpers
- **t3-env + Zod v4** — build-time environment validation
- **React Hook Form + Zod v4** — type-safe forms via `useZodForm`
- **Sonner** — themed toast notifications
- **nuqs** — type-safe URL search params (requires `NuqsAdapter` in layout)
- **date-fns v4 + nanoid** — date formatting and ID generation
- **Vitest + Testing Library** — unit and component tests
- **ESLint + Prettier** — linting and formatting enforced by `pnpm lint` and `pnpm format:check`

---

## Directory Structure

```
src/
  env.ts                # t3-env schema — import here, never read process.env directly
  app/                  # Next.js App Router pages and layouts
    layout.tsx          # Root layout — providers live here
    globals.css         # Imports only — theme modules live in src/styles
    page.tsx            # Home page
    not-found.tsx       # Custom 404. Replaces the Next.js default, which injects unlayered body styles
    motion/             # /motion demo route
  styles/               # The single Sunbeam theme (one :root and .dark for every route)
    tokens.css          # Raw color tokens, radii, panel shadow, mascot colors
    theme.css           # Tailwind bridge: @custom-variant dark and @theme inline
    typography.css      # Typeface stacks and base heading, body, and code roles
    motion.css          # Motion tokens, reduced-motion override, transition-control
    base.css            # Page colors, focus ring, native control rules
    fonts.ts            # The only next/font load
    index.ts            # Barrel — exports fontVariables
  components/
    brand/              # BrandMark, BrandLink, and shared geometry (also drives the app icons)
    home/               # Home screen components (hero, mascot, scoreboard, …). Homepage-only CSS modules
    organizations/      # Organization screen components and barrel
      model/            # Shared identity and organization schemas
      account/          # Identity, installation, and access actions
      authorization/    # Session state and authorization views
      directory/        # Organization list and empty states
      navigation/       # OrganizationNav: Scores and People links
      administration/   # People workspace at /organizations/[login]/people
        access/         # Owner capability gate and blocked states
        data/           # Zod schemas, query keys, query and mutation hooks, error copy
        draft/          # Draft model, login rule, import parsers, workspace reducer
        editor/         # Action, team, role, member directory, search, import, selection
        review/         # Exact-target review, acknowledgments, enqueue outcomes
        progress/       # Live receipt status and itemized results
        history/        # Recent receipts
        workspace/      # Composition, receipt URL state, focus requests
      scores/
        data/           # API types and query state
        form/
          fields/       # Date and filter controls
        presets/        # Saved queries
        results/        # Score tables
        state/          # Form controller and error messages
    layout/             # PageShell and PageHeader for application pages
    not-found/          # NotFoundView for src/app/not-found.tsx (unmatched routes)
    providers/          # Client-side React context providers
      query-provider.tsx
      theme-provider.tsx
      index.ts          # Barrel export
    motion/             # Framer Motion wrapper components
      demo/             # Motion demo components and barrel
      index.ts
    ui/                 # Shared primitives. Barrel exports Button, LinkButton, Input, Textarea,
                        # NativeSelect, Field, CheckboxField, RadioField, StatusMessage, panelVariants, Toaster
      button.tsx        # cva variants (default, outline, link) and sizes (default, lg)
      link-button.tsx   # Link styled as a button. `native` renders <a> for /api/v1/auth/*
      control.ts        # controlClassName shared by Input, Textarea, NativeSelect
      input.tsx, textarea.tsx, native-select.tsx, field.tsx, checkbox-field.tsx, radio-field.tsx
      status-message.tsx, status-icon.tsx  # Icon + text messages, tone neutral|success|error
      panel.ts          # panelVariants: raised or dashed surface
      sonner.tsx        # Themed <Toaster>
  api/                  # HTTP layer
    client.ts           # axios instance `api` + createApiClient()
    errors.ts           # ApiError class, isApiError(), toApiError()
    request.ts          # request<S>() — typed, optionally schema-validated
    index.ts            # Barrel export
  lib/                  # Cross-cutting utilities (no side effects, tree-shakeable)
    utils.ts            # cn() — clsx + tailwind-merge
    motion.ts           # Shared animation variants and constants
    query.ts            # makeQueryClient(), getQueryClient(), createQueryKeys()
  hooks/                # Cross-cutting use* hooks (≥3 consumers or truly shared)
    use-mounted.ts
    use-zod-form.ts     # useZodForm(schema, options) — RHF + zodResolver
  stores/               # Zustand stores
    create-selectors.ts # createSelectors(store) — auto-generates use.* hooks
    ui-store.ts         # useUiStore — sidebar state example (persist)
    index.ts
  schemas/              # Shared Zod schemas
    common.ts           # emailSchema, paginationSchema, paginatedResponseSchema, …
    index.ts
  utils/                # Pure utility functions
    date.ts             # formatDate, formatDateTime, formatRelativeDate, timeAgo
    id.ts               # nanoid re-export
    index.ts
  constants/            # App-wide constants (no logic)
    api.ts              # API_TIMEOUT_MS, QUERY_STALE_TIME_MS, QUERY_GC_TIME_MS
    env.ts              # IS_PRODUCTION, IS_DEVELOPMENT, IS_TEST
    index.ts
  types/                # Cross-cutting TypeScript interfaces
    index.ts            # ApiResponse<T>, PaginatedResponse<T>, ApiErrorData
  test/
    setup.ts            # Vitest global setup (@testing-library/jest-dom)
```

Feature-specific hooks, types, and helpers should be colocated with their feature. Use `src/hooks/`, `src/types/`, and `src/schemas/` only for cross-cutting concerns used in three or more places.

---

## Commands

| Script              | What it does                 |
| ------------------- | ---------------------------- |
| `pnpm dev`          | Start dev server (Turbopack) |
| `pnpm build`        | Production build             |
| `pnpm start`        | Serve production build       |
| `pnpm lint`         | ESLint                       |
| `pnpm lint:fix`     | ESLint with auto-fix         |
| `pnpm format`       | Prettier write               |
| `pnpm format:check` | Prettier check (used in CI)  |
| `pnpm typecheck`    | `tsc --noEmit`               |
| `pnpm test`         | Vitest run (single pass)     |
| `pnpm test:watch`   | Vitest in watch mode         |

---

## Conventions

### Modular Code

Keep `src/app/` for route entry points, layouts, metadata, and route-specific framework files.

- Route pages import feature components from `src/components/` and compose them. Do not implement full screens inside `app/`.
- Organize components into feature folders and subfolders that reflect the UI hierarchy.
- Export public components through `index.ts` barrels. Pages import those barrels. Internal files use direct imports to avoid cycles.
- Split screens into small components with explicit, typed props. Pass data and actions to presentation components.
- Target about 80 lines per implementation file. Usually keep files below 100 lines after formatting.
- Smaller components are welcome. Do not add code to reach 80 lines, or compress formatting to meet the limit.
- Keep one concern per file. Extract a cohesive component or helper when a file grows.
- Keep route-specific state near its feature. Put shared hooks in `src/hooks/` and shared pure functions in `src/lib/`.
- Reuse existing components and variants instead of creating duplicates.
- `src/components/react-bits/` holds [React Bits](https://reactbits.dev) components installed with `npx shadcn@latest add @react-bits/<Name>-TS-TW`. This is vendored upstream source. Keep changes minimal and mark each one `Local patch`, so it is easy to re-apply after an update. The folder is excluded from ESLint and exempt from the line target. Import it through the barrel, and wrap it in small feature components.
- Homepage React Bits source URLs and local patches are recorded in `README.md`. The upstream license stays beside the vendors.

### Comments

- Comment **why**, not what. Code shows what; the comment explains the non-obvious decision behind it.
- No commented-out code — delete it, git has history.
- No multiline comments. Inline single-line comments only when the intent cannot be expressed in the code itself.

### TypeScript

- Use `interface` for object shapes and component props. Reserve `type` for unions and aliases.
- Always use `import type` for type-only imports. Enforced by ESLint (`consistent-type-imports`).
- `any` is banned. Use `unknown` and narrow.
- No explicit return types on components — let inference handle it. Explicit return types on exported `lib/` functions only.
- Use Zod v4 top-level APIs: `z.email()`, `z.url()`, `z.uuid()`. Avoid deprecated `ZodTypeAny` — use `ZodType`.

### Imports

Prettier auto-sorts on every format pass. The enforced order is:

```
react / react-dom
next and next/*

<third-party packages>

@/* (internal absolute)

./relative
```

Blank lines between each group are auto-inserted — do not add them by hand.

### Tailwind

- `prettier-plugin-tailwindcss` auto-sorts utility classes on format. Never sort by hand.
- Use `cn()` for conditional or merged class strings.
- Use `cva()` for components with variant axes.
- Raw design tokens live in `src/styles/tokens.css`. Do not inline raw colours, and do not define tokens in routes, layouts, or components.
- Use mapped utilities (`bg-card`, `text-muted-foreground`, `border-input`, `rounded-control`, `rounded-panel`, `shadow-panel`). Use only breakpoints `md:` and `lg:`.
- Never use `text-primary` or `text-xs`. Text below 14px is not allowed. Marigold is a fill only.

### Components

- Named exports only — no default exports for components.
- Barrel `index.ts` in every component folder. Consumers import from the folder, not the file.
- Props via `interface Props extends …` — extend a base type where applicable.
- `'use client'` at the top only when the component genuinely needs interactivity. Default to Server Components.

### Environment Variables

- Never read `process.env` directly. Import from `@/env` to get type-safe, validated values.
- Set `SKIP_ENV_VALIDATION=true` in CI or Docker builds to bypass validation without supplying real secrets.

### HTTP / API

- Use `api` from `@/api` for browser HTTP calls (`/api/v1/*` is rewritten to `API_ORIGIN/v1/*`). Use `createApiClient(url)` for server-side internal URLs.
- Use `request({ ..., schema })` for zod-validated typed responses.
- All rejections from `api` are normalised to `ApiError`. Check with `isApiError(err)`.

### Data Fetching (TanStack Query)

- Define query keys with `createQueryKeys(entity)` from `@/lib/query`. Colocate the result with the feature's API module.
- Server-side prefetch: `getQueryClient()` → `prefetchQuery` → `<HydrationBoundary state={dehydrate(qc)}>`.
- Client components use `useQuery`, `useMutation`, `useSuspenseQuery` from `@tanstack/react-query`.
- The TanStack Query ESLint plugin (`@tanstack/eslint-plugin-query`) enforces exhaustive query keys.

### State Management (Zustand)

- One store per file in `src/stores/`. Use curried `create<State>()` form.
- Wrap with `createSelectors` to get `store.use.field()` auto-hooks.
- For persisted stores, add `persist()` middleware and cast the result as `UseBoundStore<StoreApi<State>>` before passing to `createSelectors`.
- Server-side hydration: guard with `useMounted()` or call `store.persist.rehydrate()` as needed.

### Forms

- Use `useZodForm(schema, options?)` from `@/hooks/use-zod-form` — wires RHF + `zodResolver`.
- Colocate feature form schemas with the feature. Only cross-cutting primitives go in `src/schemas/`.
- Submit handler: validated data → `api`/`request` call → `toast.success/error`.

### Notifications (Sonner)

- Import `toast` from `sonner` and call directly. No wrapper needed.
- `<Toaster />` in `layout.tsx` handles positioning, theming, and stacking automatically.

### URL State (nuqs)

- Use `useQueryState` and `parseAs*` parsers from `nuqs` in client components.
- `NuqsAdapter` in `layout.tsx` is required — it is already wired.

### Adding a Provider

1. Create `src/components/providers/<name>.tsx` with `'use client'` at the top.
2. Export from `src/components/providers/index.ts`.
3. Compose it inside the hierarchy in `src/app/layout.tsx`.

---

## Theming

Dark mode is class-based (`@custom-variant dark (&:is(.dark *))`). `next-themes` stamps `.dark` on `<html>` before hydration — no flash.

- Default theme: `system` — follows `prefers-color-scheme` automatically.
- Use `useMounted()` from `src/hooks/use-mounted.ts` to guard any client-only rendering (e.g. reading `useTheme()` before hydration).

### Global theme (Sunbeam)

`web/design-system.md` is the contract. Read it before you change any screen.

- One theme applies to every route. `src/app/globals.css` contains imports only. Tokens, the Tailwind mapping, type, motion, and base rules live in `src/styles`.
- Fonts load once in `src/styles/fonts.ts`. `RootLayout` applies `fontVariables` to `<html>`. Do not add another `next/font` load.
- CSS modules use raw tokens (`var(--card)`, `var(--motion-hover)`). Tailwind code uses mapped utilities. Names such as `--home-*` and `.home` do not exist.
- Application screens use `PageShell` and `PageHeader` from `@/components/layout`, and the primitives in `@/components/ui`.
  Never style a raw `button`, `input`, `select`, or `textarea` with theme classes. Never apply `buttonVariants` outside `src/components/ui`.
- Use `LinkButton` for button-like links. Pass `native` for `/api/v1/auth/*`.
- Use `StatusMessage` with an explicit `role` for status text. The tone adds the icon and color, never the role.
- Use `panelVariants()` on the semantic element for panels. Do not add a `Panel` wrapper.
- The cat, the beam, and decorative motion appear only on `/`. `/motion` keeps its Framer Motion examples only.
- Controls are 44px high with a 3:1 border (`border-input`). The focus ring is one `:focus-visible` rule in `base.css`. Do not add `outline-none` or `focus-visible:ring-*`.
- Ask the design owner before you add a color, a font, an animation, a variant, or a copy change.

---

## Motion

All animation components use `LazyMotion + domAnimation` (tree-shaken, ~16 kB). Use the `m.*` namespace; never import from `motion/*` directly in components.

Shared variants and transition presets live in `src/lib/motion.ts` — add new ones there, not inline in components.

---

## Testing

- Tests live next to their source in `__tests__/` subdirectories.
- Setup file: `src/test/setup.ts` — imports jest-dom matchers and registers `afterEach(cleanup)`.
- Import `describe`, `it`, `expect`, etc. explicitly from `vitest`.
- Use `@testing-library/react` for component tests; `@testing-library/user-event` for interactions.
- **What to test:** unit tests for all `lib/`, `api/`, `schemas/`, `utils/`, and `stores/` utilities; component tests for interactive components (state, events, ARIA). Skip pure layout/presentational components.
- Test behaviour, not implementation. Assert roles, labels, and states — not class strings or style values.
- Set `SKIP_ENV_VALIDATION=true` when running tests that transitively import `@/env` or `@/api/client`.

---

## Monorepo Notes

- This app lives in `web`. Repository-level tooling (CI, commit hooks, licence) lives at the monorepo root.
- The frontend proxies only `/api/v1/*` to the API origin via `next.config.ts`. Keep webhook and health endpoints off the frontend origin.
- Before you open a PR, run `pnpm lint && pnpm typecheck && pnpm test && pnpm build` in `web`.
