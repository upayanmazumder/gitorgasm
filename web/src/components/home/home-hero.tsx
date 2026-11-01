import Link from 'next/link';

import { HeroDither } from './hero-dither';

export function HomeHero() {
  return (
    <section
      aria-labelledby="home-heading"
      className="relative isolate overflow-hidden py-24 sm:py-36"
    >
      <HeroDither />
      <h1
        id="home-heading"
        className="max-w-2xl text-5xl leading-[1.08] font-semibold tracking-tight sm:text-6xl"
      >
        See the work behind the commits.
      </h1>
      <p className="mt-6 max-w-lg text-lg leading-8 text-muted-foreground">
        Connect your GitHub organization. Explore contributor and repository activity scores, with
        the filters that matter to you.
      </p>
      <Link
        href="/organizations"
        className="mt-8 inline-flex min-h-12 items-center rounded-md bg-foreground px-6 py-3 text-sm font-medium text-background hover:opacity-80 focus-visible:outline-2 focus-visible:outline-offset-4"
      >
        View organizations
      </Link>
      <p className="mt-4 text-sm text-muted-foreground">Sign in with GitHub to get started.</p>
    </section>
  );
}
