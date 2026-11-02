import { Button, LinkButton, StatusMessage } from '@/components/ui';

import type { Rejection } from '../data';

type Register = (target: string) => (element: HTMLElement | null) => void;

function rejectionCopy(rejection: Rejection, org: string) {
  switch (rejection) {
    case 'invalid':
      return 'Gitorgasm did not accept this request. Check the people and the action, then review again.';
    case 'conflict':
      return 'This request does not match its first attempt. Gitorgasm did not send it. Confirm again to send it as a new request.';
    case 'too_many':
      return 'Too many requests. Gitorgasm did not send this request. Wait a minute, then confirm again.';
    case 'origin':
      return 'Gitorgasm did not send this request. Reload the page, then try again.';
    case 'access_changed':
      return `Gitorgasm did not send this request. Your access to ${org} changed.`;
    case 'signed_out':
      return 'Your session ended. Gitorgasm did not send this request. Sign in again, then create the request again.';
    case 'reauthenticate':
      return 'Your GitHub authorization expired. Gitorgasm did not send this request. Sign in with GitHub again, then create the request again.';
  }
}

interface RejectedProps {
  rejection: Rejection;
  org: string;
  register: Register;
}

export function RejectedAlert({ rejection, org, register }: RejectedProps) {
  const signIn = rejection === 'signed_out' || rejection === 'reauthenticate';
  return (
    <div tabIndex={-1} ref={register('enqueue-alert')} className="space-y-4">
      <StatusMessage role="alert" tone="error">
        {rejectionCopy(rejection, org)}
      </StatusMessage>
      {signIn && (
        <LinkButton href="/api/v1/auth/login" native>
          Sign in with GitHub
        </LinkButton>
      )}
    </div>
  );
}

interface UncertainProps {
  rateLimited: boolean;
  register: Register;
  onResend: () => void;
  onCheckRecent: () => void;
  onDiscard: () => void;
}

export function UncertainAlert({
  rateLimited,
  register,
  onResend,
  onCheckRecent,
  onDiscard,
}: UncertainProps) {
  return (
    <div role="alert" tabIndex={-1} ref={register('enqueue-alert')} className="space-y-4">
      <h3>Request status unknown</h3>
      <p>Gitorgasm did not confirm that it received this request. It may have started.</p>
      <p>
        Send the same request again. Gitorgasm recognizes a repeat and does not apply the changes
        twice.
      </p>
      {rateLimited && <p>GitHub limited requests. Wait a few minutes before you send it again.</p>}
      <p>
        If you discard it and create a new request, the changes can run twice. Check recent requests
        first.
      </p>
      <div className="flex flex-wrap gap-3">
        <Button onClick={onResend}>Send the same request again</Button>
        <Button variant="outline" onClick={onCheckRecent}>
          Check recent requests
        </Button>
        <Button variant="outline" onClick={onDiscard}>
          Discard and edit
        </Button>
      </div>
    </div>
  );
}
