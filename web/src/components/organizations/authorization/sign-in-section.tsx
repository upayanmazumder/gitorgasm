import { LinkButton, panelVariants } from '@/components/ui';

export function SignInSection() {
  return (
    <section
      aria-labelledby="sign-in-heading"
      className={panelVariants({ className: 'space-y-4' })}
    >
      <h2 id="sign-in-heading">Sign in to get started</h2>
      <p className="text-muted-foreground">
        Use your GitHub account to see connected organizations. You can install the GitHub App after
        signing in.
      </p>
      <LinkButton native href="/api/v1/auth/login">
        Sign in with GitHub
      </LinkButton>
    </section>
  );
}
