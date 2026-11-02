import type { UseQueryResult } from '@tanstack/react-query';

import { isApiError } from '@/api';
import { Button, LinkButton, panelVariants, StatusMessage } from '@/components/ui';
import { formatDateTime } from '@/utils';

import { isReceiptId, readFailure, type Operation } from '../data';
import { describeRequest } from '../draft';
import { ReadError } from '../read-error';
import { ResultNotes, StatusLine } from './progress-status';
import { ResultGroups } from './result-groups';

interface Props {
  login: string;
  receipt: string;
  query: UseQueryResult<Operation>;
  // Why "Review again" is unavailable, or null when it is available.
  reviewAgainHelp: string | null;
  onReviewAgain: (operation: Operation) => void;
  onClose: () => void;
  register: (target: string) => (element: HTMLElement | null) => void;
}

function Body({
  login,
  query,
  reviewAgainHelp,
  onReviewAgain,
}: Omit<Props, 'receipt' | 'onClose' | 'register'>) {
  const { data, error } = query;
  if (!data) {
    if (query.isPending) return <StatusMessage role="status">Loading the request…</StatusMessage>;
    if (isApiError(error) && error.status === 404) {
      return (
        <StatusMessage role="alert" tone="error">
          Gitorgasm cannot find this request for your account in {login}.
        </StatusMessage>
      );
    }
    return (
      <ReadError
        error={error}
        subject="the request"
        org={login}
        fetching={query.isFetching}
        onRetry={() => void query.refetch()}
        retryLabel="Check progress"
      />
    );
  }

  const failure = query.isError ? readFailure(error, 'the progress', login) : null;
  return (
    <>
      <p className="font-semibold">
        {describeRequest(
          { kind: data.kind, count: data.target_count, role: data.role, teamName: data.team_slug },
          login
        )}
      </p>
      <p className="text-sm text-muted-foreground">
        Sent <time dateTime={data.created_at}>{formatDateTime(data.created_at)}</time>
        {data.completed_at && (
          <>
            . Finished <time dateTime={data.completed_at}>{formatDateTime(data.completed_at)}</time>
          </>
        )}
      </p>
      {failure && (
        <div className="space-y-4">
          <StatusMessage role="status">
            {failure.generic ? 'Could not refresh the progress.' : failure.message} Showing the last
            update.
          </StatusMessage>
          <div className="flex flex-wrap gap-3">
            {failure.signIn && (
              <LinkButton href="/api/v1/auth/login" native>
                Sign in with GitHub
              </LinkButton>
            )}
            <Button
              variant="outline"
              onClick={() => void query.refetch()}
              disabled={query.isFetching}
            >
              Check progress
            </Button>
          </div>
        </div>
      )}
      <StatusLine operation={data} />
      <ResultNotes
        operation={data}
        org={login}
        reviewAgainHelp={reviewAgainHelp}
        onReviewAgain={() => onReviewAgain(data)}
      />
      <ResultGroups operation={data} />
    </>
  );
}

export function OperationProgress({ receipt, onClose, register, ...body }: Props) {
  return (
    <section
      aria-labelledby="people-progress-heading"
      className={panelVariants({ className: 'space-y-6' })}
    >
      <h2 id="people-progress-heading" tabIndex={-1} ref={register('progress')}>
        Request progress
      </h2>
      {isReceiptId(receipt) ? (
        <Body {...body} />
      ) : (
        <StatusMessage role="alert" tone="error">
          This request link is not valid.
        </StatusMessage>
      )}
      <Button variant="outline" onClick={onClose}>
        Close progress
      </Button>
    </section>
  );
}
