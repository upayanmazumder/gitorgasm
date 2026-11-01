import Link from 'next/link';

import { ContributionDither } from './illustrations';

export function HomeHero() {
  return (
    <section
      aria-labelledby="home-heading"
      className="grid items-center gap-12 py-20 sm:py-28 lg:grid-cols-[1.2fr_1fr] lg:gap-16"
    >
      <div>
        <p className="mb-5 text-sm font-medium text-muted-foreground">
          GitHub organization activity, in view.
        </p>
        <h1
          id="home-heading"
          className="max-w-xl text-5xl leading-[1.08] font-semibold tracking-tight sm:text-6xl"
        >
          See the work behind the commits.
        </h1>
        <p className="mt-6 max-w-lg text-lg leading-8 text-muted-foreground">
          Connect your GitHub organization. Explore contributor and repository activity scores, with
          the filters that matter to you.
        </p>
        <Link
          href="/organizations"
          className="mt-8 inline-flex min-h-12 items-center justify-center gap-4 rounded-md bg-foreground px-6 py-3 text-sm font-medium text-background hover:opacity-80 focus-visible:outline-2 focus-visible:outline-offset-4"
        >
          View organizations <span aria-hidden="true">→</span>
        </Link>
        <p className="mt-4 text-sm text-muted-foreground">Sign in with GitHub to get started.</p>
      </div>
      <div className="rounded-lg border p-6 sm:p-8">
        <div className="mb-8 flex items-center justify-between gap-4 text-xs text-muted-foreground">
          <span>Contributions, connected</span>
          <span aria-hidden="true">● ○ ●</span>
        </div>
        <ContributionDither className="h-auto w-full" />
        <p className="mt-6 border-t pt-4 text-xs text-muted-foreground">
          An illustration of activity. Your organization tells its own story.
        </p>
      </div>
    </section>
  );
}
