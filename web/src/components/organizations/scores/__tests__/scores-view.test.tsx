import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { api, ApiError } from '@/api';

import { ScoresView } from '../scores-view';

afterEach(() => vi.restoreAllMocks());

const daily = {
  data: { contributors: [], repositories: [], teams: [] },
  state: { generation: 1, last_ingested_at: null, ingestion_failed: false },
  fresh: true,
};

describe('ScoresView', () => {
  it('offers an explicit retry when daily scores are unreachable', async () => {
    const retried = Promise.withResolvers<{ data: typeof daily }>();
    let scoreCalls = 0;
    vi.spyOn(api, 'get').mockImplementation(async url => {
      if (url.endsWith('/presets')) return { data: { data: [] } };
      scoreCalls += 1;
      if (scoreCalls === 1) {
        throw new ApiError({
          message: 'Request failed with status code 502',
          status: 502,
          code: 'ERR_BAD_RESPONSE',
        });
      }
      return retried.promise;
    });
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(
      <QueryClientProvider client={client}>
        <ScoresView login="acme" />
      </QueryClientProvider>
    );

    expect(await screen.findByRole('alert')).toHaveTextContent(/temporarily unavailable/);
    expect(screen.queryByText(/status code/i)).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Show scores' })).toBeDisabled();

    await userEvent.setup().click(screen.getByRole('button', { name: 'Try again' }));
    // A refetch of a query that never had data returns to pending, so the alert and button leave.
    expect(await screen.findByText('Loading scores…')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Try again' })).not.toBeInTheDocument();
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Show scores' })).toBeDisabled();

    retried.resolve({ data: daily });
    await waitFor(() => expect(screen.getByRole('button', { name: 'Show scores' })).toBeEnabled());
    expect(screen.queryByText('Loading scores…')).not.toBeInTheDocument();
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
  });
});
