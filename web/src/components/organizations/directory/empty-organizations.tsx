import { panelVariants } from '@/components/ui';

export function EmptyOrganizations() {
  return (
    <div className={panelVariants({ tone: 'dashed', className: 'space-y-2' })}>
      <h3>No organizations available yet</h3>
      <p className="text-sm text-muted-foreground">
        Install the GitHub App for an organization, then complete the GitHub sign-in when you
        return. If installation needs approval, ask an organization owner to approve it and refresh
        GitHub access afterward.
      </p>
    </div>
  );
}
