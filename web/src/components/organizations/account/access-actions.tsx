import { LinkButton } from '@/components/ui';

export function AccessActions() {
  return (
    <>
      <div className="flex flex-wrap gap-3">
        <LinkButton native href="/api/v1/auth/install">
          Install GitHub App
        </LinkButton>
        <LinkButton native href="/api/v1/auth/login" variant="outline">
          Refresh GitHub access
        </LinkButton>
      </div>
      <p className="max-w-[60ch] text-sm text-muted-foreground">
        Choose an organization on GitHub to install the app. Already installed it or changed your
        access? Refresh GitHub access to update the organizations available here.
      </p>
    </>
  );
}
