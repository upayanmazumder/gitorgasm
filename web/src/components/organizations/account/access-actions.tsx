import { buttonVariants } from '@/components/ui/button';

export function AccessActions() {
  return (
    <>
      <div className="flex flex-wrap gap-3">
        <a href="/api/v1/auth/install" className={buttonVariants()}>
          Install GitHub App
        </a>
        <a href="/api/v1/auth/login" className={buttonVariants({ variant: 'outline' })}>
          Refresh GitHub access
        </a>
      </div>
      <p className="max-w-xl text-sm text-muted-foreground">
        Choose an organization on GitHub to install the app. Already installed it or changed your
        access? Refresh GitHub access to update the organizations available here.
      </p>
    </>
  );
}
