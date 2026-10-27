import { ScoresView } from './scores-view';

export default async function OrganizationPage({ params }: { params: Promise<{ login: string }> }) {
  const { login } = await params;
  return <ScoresView login={login} />;
}
