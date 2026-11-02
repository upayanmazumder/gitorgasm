import type { MouseEvent } from 'react';
import Link from 'next/link';

import { Button, panelVariants, StatusMessage } from '@/components/ui';
import { cn } from '@/lib';
import { formatDateTime } from '@/utils';

import { useRecentOperations, type Operation, type OperationSummary } from '../data';
import { people } from '../draft';
import { ReadError } from '../read-error';

const STATUS_LABELS: Record<OperationSummary['status'], string> = {
  queued: 'Waiting',
  running: 'In progress',
  completed: 'Finished',
  interrupted: 'Stopped',
};

function title({ kind, team_slug, role }: OperationSummary) {
  if (kind === 'invite') return 'Invite to organization';
  if (kind === 'team_membership') return `Add to team ${team_slug}`;
  return role === 'admin' ? 'Change role to Organization owner' : 'Change role to Member';
}

interface Props {
  login: string;
  receipt: string | null;
  active: Operation | undefined;
  onOpen: (id: string) => void;
  register: (target: string) => (element: HTMLElement | null) => void;
}

export function RecentRequests({ login, receipt, active, onOpen, register }: Props) {
  const list = useRecentOperations(login);
  const rows = list.data?.pages.flatMap(page => page.data) ?? [];

  // Only an unmodified primary click opens the receipt in place. Other clicks keep link behavior.
  function open(event: MouseEvent<HTMLAnchorElement>, id: string) {
    if (event.button !== 0 || event.ctrlKey || event.metaKey || event.shiftKey || event.altKey)
      return;
    event.preventDefault();
    onOpen(id);
  }

  return (
    <section aria-labelledby="people-history-heading" className="space-y-4">
      <h2 id="people-history-heading" tabIndex={-1} ref={register('history')}>
        Recent requests
      </h2>
      <p className="max-w-[60ch] text-muted-foreground">
        Your latest requests in {login} appear first. Open one to see its results.
      </p>
      {list.isPending && <StatusMessage role="status">Loading recent requests…</StatusMessage>}
      {list.isSuccess && rows.length === 0 && (
        <p>No requests yet. Requests that you send appear here.</p>
      )}
      {rows.length > 0 && (
        <ul className={panelVariants({ padding: 'none', className: 'divide-y divide-border' })}>
          {rows.map(row => {
            const current = receipt === row.id;
            const status = active && active.id === row.id ? active.status : row.status;
            return (
              <li key={row.id}>
                <Link
                  href={`?receipt=${row.id}`}
                  aria-current={current ? 'true' : undefined}
                  onClick={event => open(event, row.id)}
                  className={cn(
                    'block space-y-1 p-4 transition-control hover:bg-muted focus-visible:-outline-offset-4 md:p-6',
                    current && 'bg-muted'
                  )}
                >
                  <span className="font-semibold">{title(row)}</span>
                  <span className="block text-sm text-muted-foreground">
                    {people(row.target_count)}, {STATUS_LABELS[status]}, sent{' '}
                    {formatDateTime(row.created_at)}
                  </span>
                </Link>
              </li>
            );
          })}
        </ul>
      )}
      {list.isError && (
        <ReadError
          error={list.error}
          subject="recent requests"
          org={login}
          fetching={list.isFetching}
          onRetry={() => void (list.isFetchNextPageError ? list.fetchNextPage() : list.refetch())}
        />
      )}
      {list.hasNextPage && (
        <Button
          variant="outline"
          onClick={() => void list.fetchNextPage()}
          disabled={list.isFetchingNextPage}
        >
          Show older requests
        </Button>
      )}
    </section>
  );
}
