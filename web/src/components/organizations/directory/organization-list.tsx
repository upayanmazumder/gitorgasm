import Link from 'next/link';

import type { Organization } from '../authorization/schemas';

interface OrganizationListProps {
  organizations: readonly Organization[] | undefined;
}

export function OrganizationList({ organizations }: OrganizationListProps) {
  return (
    <nav aria-label="Connected organizations">
      <ul className="divide-y rounded-lg border">
        {organizations?.map(organization => (
          <li key={organization.login}>
            <Link
              href={`/organizations/${encodeURIComponent(organization.login)}`}
              className="block space-y-1 rounded-lg p-5 hover:bg-muted focus-visible:outline-2 focus-visible:outline-offset-4"
            >
              <span className="font-medium">{organization.login}</span>
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
