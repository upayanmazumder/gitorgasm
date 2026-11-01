import Link from 'next/link';

import { Fade } from '..';

export function DemoHeader() {
  return (
    <Fade className="space-y-3">
      <Link
        href="/"
        className="text-sm text-muted-foreground transition-colors hover:text-foreground"
      >
        &larr; Back home
      </Link>
      <h1 className="text-4xl font-semibold tracking-tight">Motion</h1>
      <p className="max-w-prose text-muted-foreground">
        Reusable Framer Motion primitives — LazyMotion-powered, tree-shakeable, and reduced-motion
        aware.
      </p>
    </Fade>
  );
}
