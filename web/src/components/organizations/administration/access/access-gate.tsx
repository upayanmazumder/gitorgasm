import type { ReactNode } from 'react';

import { Button, StatusMessage } from '@/components/ui';

import type { Capability } from '../data';
import { accessErrorMessage, definitiveBlock } from './access-copy';
import { BlockedNotice } from './blocked-notice';
import { useAdministrationAccess } from './use-administration-access';

interface Props {
  login: string;
  children: (capability: Capability) => ReactNode;
}

export function AccessGate({ login, children }: Props) {
  const access = useAdministrationAccess(login);
  const { data, error } = access;
  const check = () => void access.refetch();
  const blocked =
    definitiveBlock(error) ?? (data && !data.can_manage ? (data.reason ?? 'not_owner') : null);

  if (blocked) {
    return (
      <BlockedNotice
        kind={blocked}
        login={login}
        actorLogin={data?.actor.login ?? ''}
        checking={access.isFetching}
        onCheck={check}
      />
    );
  }
  if (!data) {
    if (access.isPending) return <StatusMessage role="status">Checking your access…</StatusMessage>;
    return (
      <section className="space-y-4">
        <StatusMessage role="alert" tone="error">
          {accessErrorMessage(error)}
        </StatusMessage>
        <Button variant="outline" onClick={check} disabled={access.isFetching}>
          Try again
        </Button>
      </section>
    );
  }
  return (
    <>
      {access.isError && (
        <StatusMessage role="status">
          Could not check your access again. Gitorgasm checks it again before it makes any change.
        </StatusMessage>
      )}
      {children(data)}
    </>
  );
}
