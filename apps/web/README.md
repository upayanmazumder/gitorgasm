# apps/web

The Gitorgasm web app. It is a plain Next.js (App Router) frontend in the Gitorgasm monorepo. It has no backend integration yet.

## Requirements

- Node.js 20 or later
- pnpm 10

## Commands

Run these commands in `apps/web`.

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

| Variable              | Required | Description                                            |
| --------------------- | -------- | ------------------------------------------------------ |
| `NEXT_PUBLIC_API_URL` | Yes      | Base URL for the HTTP client. Must be a valid URL.     |
| `SKIP_ENV_VALIDATION` | No       | Set to `true` to skip validation in CI or test builds. |

The schema is in `src/env.ts`.

## Deployment (Vercel)

1. Import the Gitorgasm repository in Vercel.
2. Set **Root Directory** to `apps/web`.
3. Keep the detected framework preset (Next.js). Vercel uses `pnpm` from `pnpm-lock.yaml`.
4. Add `NEXT_PUBLIC_API_URL` in the project environment variables.

See `AGENTS.md` for code conventions.
