import { Button, StatusMessage } from '@/components/ui';

import type { Organization } from '../model';
import { EmptyOrganizations } from './empty-organizations';
import { OrganizationList } from './organization-list';

interface OrganizationsSectionProps {
  organizations: readonly Organization[] | undefined;
  isPending: boolean;
  isFetching: boolean;
  hasError: boolean;
  isRefreshDisabled: boolean;
  onRefresh: () => void;
}

export function OrganizationsSection({
  organizations,
  isPending,
  isFetching,
  hasError,
  isRefreshDisabled,
  onRefresh,
}: OrganizationsSectionProps) {
  return (
    <section aria-labelledby="organizations-heading" className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h2 id="organizations-heading">Connected organizations</h2>
        <Button variant="outline" onClick={onRefresh} disabled={isRefreshDisabled}>
          {isFetching ? 'Checking…' : 'Check organizations'}
        </Button>
      </div>
      {isPending ? (
        <StatusMessage role="status">Loading organizations…</StatusMessage>
      ) : hasError ? (
        <StatusMessage role="alert" tone="error">
          Could not load your organizations. Check your connection, then choose Check organizations
          to try again.
        </StatusMessage>
      ) : organizations?.length === 0 ? (
        <EmptyOrganizations />
      ) : (
        <OrganizationList organizations={organizations} />
      )}
    </section>
  );
}
