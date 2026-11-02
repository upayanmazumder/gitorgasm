# web

Gitorgasm shows organization activity and scores. The website connects to the app server.

## Home page

The home page uses the **Sunbeam** design system in `design-system.md`.
Claude Opus 5.5 wrote the design direction. Claude Sonnet 5.5 wrote the frontend code.
The design system owns colors, type, spacing, controls, motion, and copy. Read it before you change the page.

A marigold cat waves from the top of an illustrative scoreboard, in a soft beam of light.
Below the hero, a commit-line timeline explains three steps, and a filter picture explains scoping.
The page introduces organization connection, contributor and repository scores, activity filters, UTC periods, saved presets, and daily refreshes.
Scores describe GitHub activity, not developer productivity.
Visible labels identify illustrative figures and settings.
App actions lead to `/organizations`. Navigation uses page anchors and the project repository.

Tokens for color, radius, shadow, motion, and mascot colors apply only to `.home` in `src/app/globals.css`.
The fonts Bricolage Grotesque and Instrument Sans load in `src/components/home/fonts.ts` and apply to the `.home` root only.
Application screens keep their neutral theme and root fonts.

Components live in `src/components/home`, with the public export in `index.ts`.
The `hero`, `mascot`, `scoreboard`, `features`, `scope`, `effects`, and `footer` folders separate the composition.
`ButtonLink` and `HomeBrand` are shared by the navigation, hero, scope, and footer.

### Brand mark and icon

The mark is an original seated cat with a tail that rises as a git line and ends in a commit ring.
It is not an Octocat, not a trace of any GitHub artwork, and does not imply a GitHub affiliation.
The cat is never placed in a circle, and it has no tentacles.

- `src/components/brand` holds the shared geometry (`mark-paths.ts`) and the `BrandMark` component.
  `BrandMark` draws a marigold tile with an ink cat on a 32 by 32 grid.
  It is hidden from assistive technology next to the wordmark. Pass `title` when it stands alone.
- `src/app/icon.svg` is the favicon for browsers that support SVG. It repeats the same geometry with literal colors, because the file is standalone.
- `src/app/favicon.ico` is the fallback icon. It holds 16, 32, and 48 pixel frames that were rendered from `icon.svg` with `rsvg-convert` and packed with ImageMagick. Render it again when `icon.svg` changes.
- `src/app/apple-icon.tsx` renders the 180 pixel touch icon from the same geometry.
- The cat and tail span x 5 to 29.7, so every tile draws them inside a `translate(-1.4 0)` group (`MARK_GLYPH_TRANSFORM`). The tile rectangle does not move.

### Mascot animation

`src/components/home/mascot` draws the hero cat. The art is server HTML, so it shows in the rest pose before scripts run.
The SVG has parts named `tail`, `body`, `leg-static`, `head`, `eyes-open`, `eyes-happy`, and `arm-wave`.
The wave moves only `transform` and `opacity`. The SVG box never changes size.

- One wave takes 1600 ms. The arm lifts to 150 degrees with a small overshoot, swings between 132 and 165 degrees, then returns.
  The head tilts by -4 degrees, and the eyes become happy arcs for the middle of the wave.
- The wave runs once when 60 percent of the cat is visible. It does not loop and nothing else on the mascot moves.
- The cat is a button named "Make the Gitorgasm cat wave". Click, tap, Enter, and Space restart the wave. A press during a wave restarts it.
- With `prefers-reduced-motion: reduce`, the cat shows a static greeting pose with the arm raised and happy eyes.
  It is then an image named "The Gitorgasm cat waving hello", with no button and no animation.
  The CSS applies this pose before hydration, and the component swaps the button for the image after it.

### Effects and motion budget

Content and illustrations render in server HTML before effects load.
At most two things animate in a view: the mascot wave (once) and the LightRays beam.
Fine-pointer screens without a reduced-motion preference load React Bits effects only at widths of 768 pixels or more.
LightRays provides the sunbeam behind the hero. SpotlightCard lights only the scoreboard panel.
The scoreboard keeps its full content in the loading fallback, so nothing flashes blank.
Touch, mobile, and reduced-motion views keep static surfaces and a CSS light effect.
DotGrid and Dither are removed.

### React Bits provenance

Vendored source lives in `src/components/react-bits`, with exports through its barrel.
LightRays and SpotlightCard are official React Bits components from the TypeScript and Tailwind registry.
These sources need no external dependencies.

| Component     | Official source                                                              | Documentation                                                     |
| ------------- | ---------------------------------------------------------------------------- | ----------------------------------------------------------------- |
| LightRays     | [LightRays-TS-TW.json](https://reactbits.dev/r/LightRays-TS-TW.json)         | [Light Rays](https://reactbits.dev/backgrounds/light-rays)        |
| SpotlightCard | [SpotlightCard-TS-TW.json](https://reactbits.dev/r/SpotlightCard-TS-TW.json) | [Spotlight Card](https://reactbits.dev/components/spotlight-card) |

No local vendor patches apply.
The upstream [MIT + Commons Clause license](https://github.com/DavidHDev/react-bits/blob/main/LICENSE.md) belongs to David Haz.
Its notice stays in `src/components/react-bits/LICENSE.txt`.
Retain source attribution and license notices when you update these components.
Mark necessary vendor changes with a one-line `Local patch` annotation.

### Review checklist

Check the skip link, section navigation, organization actions, keyboard focus, and project link.
Review widths of 390, 768, 1024, and 1440 pixels in both themes.
Check the wave with a keyboard, with touch, and with reduced motion on.
The scoreboard must remain readable while effects load or remain unavailable.

## Requirements

- Node.js 20 or later
- pnpm 11 (the pinned version is in `package.json`)

## Commands

Run these commands in `web`.

| Command          | Action                         |
| ---------------- | ------------------------------ |
| `pnpm install`   | Install dependencies           |
| `pnpm dev`       | Start the development server   |
| `pnpm build`     | Make a production build        |
| `pnpm start`     | Serve the production build     |
| `pnpm lint`      | Run ESLint                     |
| `pnpm typecheck` | Run `tsc --noEmit`             |
| `pnpm test`      | Run the Vitest unit tests once |
| `pnpm format`    | Format the files with Prettier |

## Environment variables

Create `web/.env.local` with `API_ORIGIN=http://localhost:8080` for local development. Start the API from the repository root with `make up`, then run `pnpm dev` in `web`. The local frontend at `http://localhost:3000` rewrites `/api/v1/*` to `http://localhost:8080/v1/*`; for example, `/api/v1/me` reaches `/v1/me` (which returns 401 until signed in). The API's `APP_ORIGIN` must match the browser's frontend origin for state-changing requests, and local HTTP cookies require `COOKIE_SECURE=false`.

| Variable              | Required | Description                                                                                  |
| --------------------- | -------- | -------------------------------------------------------------------------------------------- |
| `API_ORIGIN`          | Yes      | API origin only (no path or trailing slash), for example `https://api-gitorgasm.upayan.dev`. |
| `SKIP_ENV_VALIDATION` | No       | Set to `true` to skip validation in CI or test builds.                                       |

Only `/api/v1/*` is proxied. The browser calls this app's origin, so session cookies remain first-party and CORS is unnecessary. GitHub's OAuth callback must also target `/api/v1/auth/callback` on the frontend origin. The schema is in `src/env.ts`, the proxy and security headers are in `next.config.ts`, and formatter configuration is in `web/.prettierrc`.

## Sign-in and organization access

Open `/organizations` to sign in with GitHub. The page checks `/api/v1/me` before loading accessible organizations, shows the signed-in identity, and links each organization to its activity scores. A failed session check is distinct from being signed out and can be retried.

- **Install GitHub App** opens `/api/v1/auth/install`. Choose an organization on GitHub; an organization owner may need to approve the installation.
- **Refresh GitHub access** opens `/api/v1/auth/login` to refresh installation grants through OAuth. Use it after an installation is approved or your GitHub access changes.
- **Check organizations** reloads the accessible organization list without starting OAuth. A new installation may still be waiting for its first sync.
- **Sign out** posts to `/api/v1/auth/logout`. On success, the website cancels in-flight queries and removes cached organization, score, and mutation data. A failed sign-out keeps the current screen available and shows an error.

Configure the GitHub App's **Setup URL** as `/api/v1/auth/login` on the frontend origin and its OAuth callback as `/api/v1/auth/callback`. Disable **Request user authorization (OAuth) during installation**: the setup return must begin the server's state- and PKCE-protected OAuth flow. OAuth returns to `/organizations`; installation query parameters are never treated as proof of access by the UI.

Session and organization data use TanStack Query, with no automatic error retries. Returning to the browser tab rechecks both, and organization caches are keyed by signed-in user. Organization access is always enforced by the API, not the presence of a navigation link.

## Component structure

Keep route files in `src/app/` thin. They import and compose components rather than implement full screens.
Place UI in feature folders under `src/components/`, with subfolders for each part of the screen.
Pass data and actions through typed props. Export public components through `index.ts` barrels for page imports.

Keep components small and focused. About 80 lines is a target, not a minimum.
Usually keep implementation files below 100 lines after formatting. Smaller files are welcome.
Do not compress code or add unnecessary abstractions to meet a line count.

The organization feature separates account actions, authorization, directory views, and shared models.
Scores use nested `data`, `form/fields`, `presets`, `results`, and `state` folders.
Each folder exports its public API through `index.ts`. Internal siblings use direct imports to avoid cycles.
Form values stay in `form/types.ts`. The `data` folder contains API models and query hooks.

The HTTP layer exposes `api`, `createApiClient`, `request`, and normalized errors through `src/api/index.ts`.
Its `transport` folder owns Axios client configuration and typed, schema-validated requests.
Its `errors` folder owns the error shape, class, and response normalization.
HTTP models no longer live in a global types folder.
Hooks, schemas, and types specific to an organization screen remain inside that feature.
Cross-cutting hooks and utilities expose explicit barrels in `src/hooks` and `src/lib`.

## Deployment (Vercel)

1. Import the Gitorgasm repository in Vercel.
2. Set **Root Directory** to `web`.
3. Keep the detected framework preset (Next.js). Vercel uses `pnpm` from `pnpm-lock.yaml`.
4. Add `API_ORIGIN` in the project environment variables.

See `AGENTS.md` for code conventions.
