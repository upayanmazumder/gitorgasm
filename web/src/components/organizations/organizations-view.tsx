import { OrganizationAuthorization } from './authorization';
import { OrganizationHeader } from './organization-header';

export function OrganizationsView() {
  return (
    <main className="mx-auto w-full max-w-3xl space-y-10 px-6 py-12">
      <OrganizationHeader />
      <OrganizationAuthorization />
    </main>
  );
}
