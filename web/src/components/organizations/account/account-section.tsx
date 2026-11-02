import { Button, StatusMessage } from '@/components/ui';

import type { OrganizationUser } from '../model';
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
    <section aria-labelledby="account-heading" className="space-y-6 border-y border-border py-6">
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
      {isSigningOut ? <StatusMessage role="status">Signing out…</StatusMessage> : null}
      {hasSignOutError ? (
        <StatusMessage role="alert" tone="error">
          Could not sign out. Your session may still be active. Try signing out again.
        </StatusMessage>
      ) : null}
    </section>
  );
}
