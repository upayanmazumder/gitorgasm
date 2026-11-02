'use client';

import { StatusMessage } from '@/components/ui';

import { AccountSection } from '../account';
import { OrganizationsSection } from '../directory';
import { SessionError } from './session-error';
import { SignInSection } from './sign-in-section';
import { useOrganizationAuthorization } from './use-organization-authorization';

export function OrganizationAuthorization() {
  const { session, organizations, logout, signedOut } = useOrganizationAuthorization();

  if (session.isPending) return <StatusMessage role="status">Checking your session…</StatusMessage>;
  if (session.isError) return <SessionError onRetry={() => void session.refetch()} />;
  if (signedOut) return <SignInSection />;
  if (!session.data) return null;

  return (
    <>
      <AccountSection
        user={session.data}
        isSigningOut={logout.isPending}
        hasSignOutError={logout.isError}
        onSignOut={() => logout.mutate()}
      />
      <OrganizationsSection
        organizations={organizations.data}
        isPending={organizations.isPending}
        isFetching={organizations.isFetching}
        hasError={organizations.isError}
        isRefreshDisabled={organizations.isFetching || logout.isPending}
        onRefresh={() => void organizations.refetch()}
      />
    </>
  );
}
