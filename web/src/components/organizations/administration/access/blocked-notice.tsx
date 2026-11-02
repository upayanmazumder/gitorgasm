import { Button, LinkButton } from '@/components/ui';

import { noticeCopy, type BlockedKind } from './access-copy';
import { AccessNotice } from './access-notice';

interface Props {
  kind: BlockedKind;
  login: string;
  actorLogin: string;
  checking: boolean;
  onCheck: () => void;
}

function Actions({ kind, login, checking, onCheck }: Omit<Props, 'actorLogin'>) {
  const scores = `/organizations/${encodeURIComponent(login)}`;
  switch (kind) {
    case 'signed_out':
    case 'reauthorize':
      return (
        <LinkButton href="/api/v1/auth/login" native>
          Sign in with GitHub
        </LinkButton>
      );
    case 'no_access':
      return <LinkButton href="/organizations">Go to your organizations</LinkButton>;
    case 'not_owner':
    case 'unsupported_account':
      return (
        <LinkButton href={scores} variant="outline">
          View scores
        </LinkButton>
      );
    case 'members_permission_required':
    case 'installation_suspended':
      return (
        <Button variant="outline" onClick={onCheck} disabled={checking}>
          Check again
        </Button>
      );
  }
}

export function BlockedNotice({ kind, login, actorLogin, checking, onCheck }: Props) {
  const { heading, body } = noticeCopy(kind, login, actorLogin);
  return (
    <AccessNotice
      heading={heading}
      body={body}
      actions={<Actions kind={kind} login={login} checking={checking} onCheck={onCheck} />}
    />
  );
}
