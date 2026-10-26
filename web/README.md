# web

The Gitorgasm web app. It is a plain Next.js (App Router) frontend in the Gitorgasm monorepo. It has no backend integration yet.

## Requirements

- Node.js 20 or later
- pnpm 10

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

Copy `example.env` to `.env.local` and set the values.

| Variable              | Required | Description                                                                                      |
| --------------------- | -------- | ------------------------------------------------------------------------------------------------ |
| `API_ORIGIN`          | Yes      | Gitorgasm API origin, for example `https://api-gitorgasm.upayan.dev`. `/api/*` is proxied to it. |
| `SKIP_ENV_VALIDATION` | No       | Set to `true` to skip validation in CI or test builds.                                           |

The browser calls `/api/...` on this app's origin. `next.config.ts` rewrites those requests to `API_ORIGIN`, so session cookies stay first-party and no CORS is needed.

The schema is in `src/env.ts`.

## Deployment (Vercel)

1. Import the Gitorgasm repository in Vercel.
2. Set **Root Directory** to `web`.
3. Keep the detected framework preset (Next.js). Vercel uses `pnpm` from `pnpm-lock.yaml`.
4. Add `API_ORIGIN` in the project environment variables.

See `AGENTS.md` for code conventions.
