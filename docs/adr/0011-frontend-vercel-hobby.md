# ADR 0011: Frontend on Vercel Hobby from the template

- Status: Accepted
- Date: 2026-10-26

## Context

The budget allows no paid resources beyond the existing VM. The cluster is memory-constrained. The legacy frontend ran on Vercel. A template `next-frontend-plus-template` exists (Next.js, TypeScript strict, TanStack Query, shadcn/ui, Tailwind, pnpm).

## Decision

- Bootstrap `web` from the template (**Verified** `web/package.json`: Next.js 16.2.11, pnpm 11.28.6 pinned through `packageManager`). CI job `web` runs lint, typecheck, format check, tests, and build.
- Deploy on Vercel Hobby (free) at `https://gitorgasm.upayan.dev` (custom domain: DNS-only Cloudflare CNAME to Vercel; `gitorgasm.vercel.app` also serves the app). The production overlay sets this value as `APP_ORIGIN` (**Verified** `deploy/kustomize/overlays/production/config.yaml`).
- The frontend holds no privileged tokens and no Supabase client. It calls the backend through the same-origin rewrite `/api/:path*` (ADR 0008). **Planned** Stage 1.5: the rewrite does not exist yet in `web`.

## Consequences

- No cluster memory used for the frontend.
- Vercel Hobby is for non-commercial personal use only ([Vercel fair use guidelines](https://vercel.com/docs/limits/fair-use-guidelines)). Commercial use needs a paid plan or another host (risk R7).
- The frontend is stateless, so moving it to the cluster or Cloudflare Pages later is a deployment change only.
- No pages consume the API yet and there is no API integration (**Planned** Stage 2).

## Alternatives

- Serve the frontend from the cluster: rejected now. Costs memory requests on a node at 80%.
- Cloudflare Pages: viable fallback. Not chosen because the template targets Vercel and the legacy app runs there.
