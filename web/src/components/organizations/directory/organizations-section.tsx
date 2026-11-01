import { Button } from '@/components/ui';

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
        <h2 id="organizations-heading" className="text-xl font-semibold">
          Connected organizations
        </h2>
        <Button variant="ghost" onClick={onRefresh} disabled={isRefreshDisabled}>
          {isFetching ? 'Checking…' : 'Check organizations'}
        </Button>
      </div>
      {isPending ? (
        <p role="status">Loading organizations…</p>
      ) : hasError ? (
        <p role="alert">
          Could not load your organizations. Check your connection, then choose Check organizations
          to try again.
        </p>
      ) : organizations?.length === 0 ? (
        <EmptyOrganizations />
      ) : (
        <OrganizationList organizations={organizations} />
      )}
    </section>
  );
}
