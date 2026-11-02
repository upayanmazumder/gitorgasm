import Link from 'next/link';

import { panelVariants } from '@/components/ui';

import type { Organization } from '../model';

interface OrganizationListProps {
  organizations: readonly Organization[] | undefined;
}

export function OrganizationList({ organizations }: OrganizationListProps) {
  return (
    <nav aria-label="Connected organizations">
      <ul className={panelVariants({ padding: 'none', className: 'divide-y divide-border' })}>
        {organizations?.map(organization => (
          <li key={organization.login}>
            <Link
              href={`/organizations/${encodeURIComponent(organization.login)}`}
              className="block space-y-1 p-4 transition-control hover:bg-muted focus-visible:-outline-offset-4 md:p-6"
            >
              <span className="font-semibold">{organization.login}</span>
              <span className="block text-sm text-muted-foreground">
                {organization.repository_count}{' '}
                {organization.repository_count === 1 ? 'repository' : 'repositories'}
                {organization.suspended
                  ? ' · Installation suspended'
                  : organization.last_synced_at === null
                    ? ' · First sync pending'
                    : null}
              </span>
            </Link>
          </li>
        ))}
      </ul>
    </nav>
  );
}
