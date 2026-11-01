import { ScoresView } from '@/components/organizations/scores';

export default async function OrganizationPage({ params }: { params: Promise<{ login: string }> }) {
  const { login } = await params;
  return <ScoresView login={login} />;
}
