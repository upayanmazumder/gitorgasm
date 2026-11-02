import { PeopleView } from '@/components/organizations/administration';

export default async function OrganizationPeoplePage({
  params,
}: {
  params: Promise<{ login: string }>;
}) {
  const { login } = await params;
  return <PeopleView login={login} />;
}
