import { buttonVariants } from '@/components/ui';

export function SignInSection() {
  return (
    <section aria-labelledby="sign-in-heading" className="space-y-4 rounded-lg border p-6">
      <h2 id="sign-in-heading" className="text-xl font-semibold">
        Sign in to get started
      </h2>
      <p className="text-muted-foreground">
        Use your GitHub account to see connected organizations. You can install the GitHub App after
        signing in.
      </p>
      <a href="/api/v1/auth/login" className={buttonVariants()}>
        Sign in with GitHub
      </a>
    </section>
  );
}
