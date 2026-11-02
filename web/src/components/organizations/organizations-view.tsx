import { PageShell } from '@/components/layout';

import { OrganizationAuthorization } from './authorization';
import { OrganizationHeader } from './organization-header';

export function OrganizationsView() {
  return (
    <PageShell>
      <OrganizationHeader />
      <OrganizationAuthorization />
    </PageShell>
  );
}
