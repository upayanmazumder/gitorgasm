import Link from 'next/link';

export function OrganizationHeader() {
  return (
    <header className="space-y-4">
      <Link
        href="/"
        className="text-sm underline underline-offset-4 focus-visible:outline-2 focus-visible:outline-offset-4"
      >
        Gitorgasm
      </Link>
      <div className="space-y-2">
        <h1 className="text-3xl font-semibold tracking-tight">Your organizations</h1>
        <p className="max-w-xl text-muted-foreground">
          Connect GitHub to explore activity scores for the organizations you can access.
        </p>
      </div>
    </header>
  );
}
