# web

Gitorgasm shows organization activity and scores. The website connects to the app server.

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

## Deployment (Vercel)

1. Import the Gitorgasm repository in Vercel.
2. Set **Root Directory** to `web`.
3. Keep the detected framework preset (Next.js). Vercel uses `pnpm` from `pnpm-lock.yaml`.
4. Add `API_ORIGIN` in the project environment variables.

See `AGENTS.md` for code conventions.
