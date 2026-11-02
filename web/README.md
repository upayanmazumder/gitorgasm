# web

Gitorgasm shows organization activity and scores. The website connects to the app server.

## Home page

The home page at `/` introduces GitHub organization connection, contributor and repository activity scores, date/time filters and search, saved query presets, and daily score refreshes. Its navigation and primary action lead to `/organizations`. It does not present team membership as a supported feature.

Components live in `src/components/home`, with public exports in `index.ts`.
The `hero` folder owns the heading, primary action, and decorative effect.
The `features` folder owns feature cards and their content.
Navigation stays beside the home composition.
Small wrappers compose vendored React Bits effects from `src/components/react-bits`.
Decorative effects stay hidden from assistive technology.

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
