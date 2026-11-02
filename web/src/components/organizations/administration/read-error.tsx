import { Button, LinkButton, StatusMessage } from '@/components/ui';

import { readFailure } from './data';

interface Props {
  error: unknown;
  subject: string;
  org: string;
  fetching: boolean;
  onRetry: () => void;
  retryLabel?: string;
  rateLimited?: string;
}

export function ReadError({
  error,
  subject,
  org,
  fetching,
  onRetry,
  retryLabel = 'Try again',
  rateLimited,
}: Props) {
  const failure = readFailure(error, subject, org, rateLimited);
  return (
    <div className="space-y-4">
      <StatusMessage role="alert" tone="error">
        {failure.message}
      </StatusMessage>
      <div className="flex flex-wrap gap-3">
        {failure.signIn && (
          <LinkButton href="/api/v1/auth/login" native>
            Sign in with GitHub
          </LinkButton>
        )}
        <Button variant="outline" onClick={onRetry} disabled={fetching}>
          {retryLabel}
        </Button>
      </div>
    </div>
  );
}
