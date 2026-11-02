'use client';

import { PageHeader, PageShell } from '@/components/layout';
import { LinkButton } from '@/components/ui';

import { OrganizationNav } from '../navigation';
import { AccessGate } from './access';
import { PeopleWorkspace } from './workspace';

interface Props {
  login: string;
}

export function PeopleView({ login }: Props) {
  return (
    <PageShell width="wide">
      <div className="space-y-6">
        <PageHeader
          lead={
            <LinkButton href="/organizations" variant="link">
              Organizations
            </LinkButton>
          }
          title={`${login} people`}
          description="Invite people, add them to teams, and change organization roles. You review each request before Gitorgasm sends it to GitHub."
        />
        <OrganizationNav login={login} current="people" />
      </div>
      <AccessGate login={login}>
        {capability => <PeopleWorkspace login={login} capability={capability} />}
      </AccessGate>
    </PageShell>
  );
}
