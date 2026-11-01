import { Button } from '@/components/ui/button';

import type { OrganizationUser } from '../authorization/schemas';
import { AccessActions } from './access-actions';

interface AccountSectionProps {
  user: Pick<OrganizationUser, 'name' | 'login'>;
  isSigningOut: boolean;
  hasSignOutError: boolean;
  onSignOut: () => void;
}

export function AccountSection({
  user,
  isSigningOut,
  hasSignOutError,
  onSignOut,
}: AccountSectionProps) {
  return (
    <section aria-labelledby="account-heading" className="space-y-5 border-y py-6">
      <div className="flex flex-wrap items-center justify-between gap-4">
        <div>
          <h2 id="account-heading" className="text-sm text-muted-foreground">
            Signed in with GitHub
          </h2>
          <p className="font-medium">
            {user.name ? `${user.name} ` : null}
            <span className="text-muted-foreground">@{user.login}</span>
          </p>
        </div>
        <Button variant="outline" onClick={onSignOut} disabled={isSigningOut}>
          {isSigningOut ? 'Signing out…' : 'Sign out'}
        </Button>
      </div>
      <AccessActions />
      {isSigningOut ? <p role="status">Signing out…</p> : null}
      {hasSignOutError ? (
        <p role="alert">
          Could not sign out. Your session may still be active. Try signing out again.
        </p>
      ) : null}
    </section>
  );
}
