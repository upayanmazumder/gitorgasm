import Link from 'next/link';

export function HomeNavigation() {
  return (
    <header className="border-b">
      <nav
        aria-label="Main navigation"
        className="mx-auto flex max-w-6xl items-center justify-between gap-4 px-6 py-5 sm:px-10"
      >
        <Link
          href="/"
          className="rounded-sm text-lg font-semibold tracking-tight focus-visible:outline-2 focus-visible:outline-offset-4"
        >
          Gitorgasm
        </Link>
        <div className="flex items-center gap-5 text-sm sm:gap-8">
          <a
            href="#features"
            className="rounded-sm text-muted-foreground hover:text-foreground focus-visible:outline-2 focus-visible:outline-offset-4"
          >
            Features
          </a>
          <Link
            href="/organizations"
            className="rounded-sm font-medium underline-offset-4 hover:underline focus-visible:outline-2 focus-visible:outline-offset-4"
          >
            Open app <span aria-hidden="true">↗</span>
          </Link>
        </div>
      </nav>
    </header>
  );
}
