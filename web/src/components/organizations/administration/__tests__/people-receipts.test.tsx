import { focusManager } from '@tanstack/react-query';
import { act, screen, waitFor, within } from '@testing-library/react';
import type { UserEvent } from '@testing-library/user-event';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { ApiError } from '@/api';

import type { Operation } from '../data';
import { mockApi, operation, paste, receiptId, renderPeople, review } from './people-test-helpers';

afterEach(() => {
  vi.restoreAllMocks();
  vi.useRealTimers();
  focusManager.setFocused(undefined);
});

async function confirmInvite(user: UserEvent) {
  await paste(user, 'alice bob');
  await review(user);
  await user.click(screen.getByRole('button', { name: 'Send 2 invitations' }));
}

describe('enqueue outcomes', () => {
  it('keeps the same key and body after an uncertain response and never retries on its own', async () => {
    let attempts = 0;
    const api = mockApi({
      'POST /operations': () => {
        attempts += 1;
        if (attempts === 1) throw new ApiError({ message: 'Network Error', code: 'ERR_NETWORK' });
        return { data: operation() };
      },
      'GET /operations/:id': () => ({ data: operation() }),
    });
    const { user } = renderPeople();
    await confirmInvite(user);

    const alert = await screen.findByRole('alert');
    expect(alert).toHaveTextContent('Request status unknown');
    expect(alert).not.toHaveTextContent(/failed|succeeded/i);
    await waitFor(() => expect(alert).toHaveFocus());
    expect(api.to('POST', '/operations')).toHaveLength(1);

    await user.click(screen.getByRole('button', { name: 'Send the same request again' }));

    await waitFor(() => expect(api.to('POST', '/operations')).toHaveLength(2));
    const [first, second] = api.to('POST', '/operations');
    expect(second.headers['Idempotency-Key']).toBe(first.headers['Idempotency-Key']);
    expect(second.data).toEqual(first.data);
    expect(await screen.findByRole('heading', { name: 'Request progress' })).toBeInTheDocument();
  });

  it('keeps the review after a definitive rejection and uses a new key on the next confirm', async () => {
    const api = mockApi({
      'POST /operations': () => {
        throw new ApiError({ message: 'raw upstream text', status: 400, code: 'invalid_request' });
      },
    });
    const { user } = renderPeople();
    await confirmInvite(user);

    expect(await screen.findByRole('alert')).toHaveTextContent(
      'Gitorgasm did not accept this request.'
    );
    expect(screen.queryByText(/raw upstream text/)).not.toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Review request' })).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Send 2 invitations' }));
    await waitFor(() => expect(api.to('POST', '/operations')).toHaveLength(2));

    const [first, second] = api.to('POST', '/operations');
    expect(second.headers['Idempotency-Key']).not.toBe(first.headers['Idempotency-Key']);
  });

  it('asks for GitHub sign-in again when the authorization expired during the request', async () => {
    mockApi({
      'POST /operations': () => {
        throw new ApiError({
          message: 'raw',
          status: 401,
          code: 'github_reauthentication_required',
        });
      },
    });
    const { user } = renderPeople();
    await confirmInvite(user);

    expect(await screen.findByRole('alert')).toHaveTextContent(
      'Your GitHub authorization expired. Gitorgasm did not send this request.'
    );
    expect(screen.getByRole('link', { name: 'Sign in with GitHub' })).toHaveAttribute(
      'href',
      '/api/v1/auth/login'
    );
  });

  it('puts the accepted receipt in the URL and moves focus to its progress', async () => {
    const onUrlUpdate = vi.fn();
    mockApi({
      'POST /operations': () => ({ data: operation() }),
      'GET /operations/:id': () => ({ data: operation() }),
    });
    const { user } = renderPeople({ onUrlUpdate });
    await confirmInvite(user);

    const heading = await screen.findByRole('heading', { name: 'Request progress' });
    await waitFor(() => expect(heading).toHaveFocus());
    expect(onUrlUpdate.mock.calls.at(-1)?.[0].searchParams.get('receipt')).toBe(receiptId(1));
    expect(await screen.findByText('Waiting to start.')).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'New request' })).toBeInTheDocument();
  });

  it('refetches the first page when Try again follows a failed check of recent requests', async () => {
    let lists = 0;
    const first = { ...operation(), results: undefined };
    const second = {
      ...operation({ id: receiptId(2), kind: 'organization_role', role: 'admin' }),
      results: undefined,
    };
    const api = mockApi({
      'POST /operations': () => {
        throw new ApiError({ message: 'Network Error', code: 'ERR_NETWORK' });
      },
      'GET /operations': () => {
        lists += 1;
        // The first read loads one page. The check that follows the lost response fails.
        if (lists === 2) throw new ApiError({ message: 'raw', status: 502 });
        return { data: lists === 1 ? [first] : [second, first], next_cursor: null };
      },
    });
    const { user } = renderPeople();
    await screen.findByRole('link', { name: /Invite to organization/ });
    await confirmInvite(user);
    await screen.findByText('Request status unknown');

    await user.click(screen.getByRole('button', { name: 'Check recent requests' }));
    const retry = await screen.findByRole('button', { name: 'Try again' });
    expect(screen.getAllByRole('link', { name: /Invite to organization/ })).toHaveLength(1);

    await user.click(retry);

    expect(
      await screen.findByRole('link', { name: /Change role to Organization owner/ })
    ).toBeInTheDocument();
    expect(api.to('GET', '/operations')).toHaveLength(3);
    expect(screen.queryByRole('button', { name: 'Try again' })).not.toBeInTheDocument();
  });
});

describe('receipt progress', () => {
  beforeEach(() => vi.useFakeTimers({ shouldAdvanceTime: true }));

  const running = operation({
    status: 'running',
    results: [
      { login: 'alice', status: 'invited', code: null, message: null },
      { login: 'bob', status: 'running', code: null, message: null },
    ],
  });
  const completed = operation({
    status: 'completed',
    completed_at: '2026-11-03T10:01:00Z',
    results: [
      { login: 'alice', status: 'invited', code: null, message: null },
      {
        login: 'bob',
        status: 'failed',
        code: 'github_error',
        message: 'GitHub rejected this invitation.',
      },
    ],
  });

  it('resumes from the URL, polls while active, and stops once the receipt is terminal', async () => {
    let reads = 0;
    const api = mockApi({
      'GET /operations/:id': () => ({ data: ++reads === 1 ? running : completed }),
    });
    renderPeople({ search: `?receipt=${receiptId(1)}`, advanceTimers: vi.advanceTimersByTime });

    expect(
      await screen.findByText('In progress. Gitorgasm makes one change at a time.')
    ).toBeInTheDocument();
    expect(screen.getByText('1 of 2 done')).toBeInTheDocument();

    await act(() => vi.advanceTimersByTimeAsync(3_000));

    const status = await screen.findByText(/Finished with problems/);
    expect(status).toHaveTextContent('Finished with problems. 1 invited, 1 failed.');
    expect(screen.getByRole('heading', { name: 'Failed (1)' })).toBeInTheDocument();
    expect(screen.getByText('GitHub rejected this invitation.')).toBeInTheDocument();
    expect(screen.queryByText(/^Finished\. /)).not.toBeInTheDocument();

    const settled = api.to('GET', '/operations/:id').length;
    await act(() => vi.advanceTimersByTimeAsync(15_000));
    expect(api.to('GET', '/operations/:id')).toHaveLength(settled);
  });

  it('does not poll while the tab is hidden', async () => {
    focusManager.setFocused(false);
    const api = mockApi({ 'GET /operations/:id': () => ({ data: running }) });
    renderPeople({ search: `?receipt=${receiptId(1)}`, advanceTimers: vi.advanceTimersByTime });
    await screen.findByText('In progress. Gitorgasm makes one change at a time.');
    const before = api.to('GET', '/operations/:id').length;

    await act(() => vi.advanceTimersByTimeAsync(12_000));

    expect(api.to('GET', '/operations/:id')).toHaveLength(before);
  });

  it('keeps a recent request Finished after its progress is closed', async () => {
    let reads = 0;
    // The server settles on the second detail read. Later list reads then report it.
    mockApi({
      'GET /operations': () => ({
        data: [
          { ...operation({ status: reads > 1 ? 'completed' : 'running' }), results: undefined },
        ],
        next_cursor: null,
      }),
      'GET /operations/:id': () => ({ data: ++reads === 1 ? running : completed }),
    });
    const { user } = renderPeople({
      search: `?receipt=${receiptId(1)}`,
      advanceTimers: vi.advanceTimersByTime,
    });
    await screen.findByText('In progress. Gitorgasm makes one change at a time.');
    const row = await screen.findByRole('link', { name: /Invite to organization/ });
    expect(row).toHaveTextContent('In progress');

    await act(() => vi.advanceTimersByTimeAsync(3_000));
    await screen.findByText(/Finished with problems/);
    await user.click(screen.getByRole('button', { name: 'Close progress' }));

    await waitFor(() =>
      expect(screen.getByRole('link', { name: /Invite to organization/ })).toHaveTextContent(
        /Finished/
      )
    );
    expect(screen.queryByText(/In progress/)).not.toBeInTheDocument();
  });

  it('refreshes a request that was still running when its progress was closed', async () => {
    let listed: Operation['status'] = 'running';
    const api = mockApi({
      'POST /operations': () => ({ data: operation({ status: 'running' }) }),
      'GET /operations': () => ({
        data: [{ ...operation({ status: listed }), results: undefined }],
        next_cursor: null,
      }),
      'GET /operations/:id': () => ({ data: running }),
    });
    const { user } = renderPeople({ advanceTimers: vi.advanceTimersByTime });
    await confirmInvite(user);
    await screen.findByText('In progress. Gitorgasm makes one change at a time.');
    await user.click(screen.getByRole('button', { name: 'Close progress' }));
    const row = () => screen.getByRole('link', { name: /Invite to organization/ });
    // Let the refetch that closing starts finish, so only a focus can show the change below.
    await act(() => vi.advanceTimersByTimeAsync(1_000));
    expect(row()).toHaveTextContent('In progress');
    const baseline = api.to('GET', '/operations').length;

    listed = 'completed';
    act(() => {
      focusManager.setFocused(false);
      focusManager.setFocused(true);
    });

    await waitFor(() => expect(row()).toHaveTextContent('Finished'));
    expect(api.to('GET', '/operations')).toHaveLength(baseline + 1);

    // No row is active now, so a later focus reads nothing.
    const settled = api.to('GET', '/operations').length;
    act(() => {
      focusManager.setFocused(false);
      focusManager.setFocused(true);
    });
    await act(() => vi.advanceTimersByTimeAsync(1_000));
    expect(api.to('GET', '/operations')).toHaveLength(settled);
  });

  it('never calls an interrupted receipt a success and retries only what was not attempted', async () => {
    const interrupted: Operation = operation({
      kind: 'team_membership',
      team_slug: 'platform',
      status: 'interrupted',
      completed_at: '2026-11-03T10:01:00Z',
      target_count: 4,
      results: [
        { login: 'alice', status: 'succeeded', code: null, message: null },
        { login: 'bob', status: 'unknown', code: 'membership_changed', message: 'Check GitHub.' },
        { login: 'carol', status: 'not_attempted', code: 'interrupted', message: null },
        { login: 'dave', status: 'not_attempted', code: 'interrupted', message: null },
      ],
    });
    const api = mockApi({
      'GET /members': () => ({ data: [{ id: 11, login: 'zoe' }], next_cursor: null }),
      'GET /operations/:id': () => ({ data: interrupted }),
    });
    const { user } = renderPeople({
      search: `?receipt=${receiptId(1)}`,
      advanceTimers: vi.advanceTimersByTime,
    });

    expect(await screen.findByText(/Stopped before it finished/)).toHaveTextContent(
      '1 changed, 1 unknown, 2 not attempted'
    );
    expect(screen.getByRole('link', { name: 'Open acme people on GitHub' })).toHaveAttribute(
      'href',
      'https://github.com/orgs/acme/people'
    );

    // Review again replaces the whole draft, so it waits until the editor is empty.
    await user.click(await screen.findByRole('radio', { name: 'Add to a team' }));
    await user.click(await screen.findByRole('checkbox', { name: 'zoe' }));
    const again = screen.getByRole('button', { name: 'Review again for 2 people' });
    expect(again).toBeDisabled();
    expect(again).toHaveAccessibleDescription('Clear the selected people first.');
    await user.click(screen.getByRole('button', { name: 'Clear list' }));
    expect(again).toBeEnabled();
    await user.click(again);

    const list = await screen.findByRole('list', { name: 'Selected people' });
    expect(
      within(list)
        .getAllByRole('listitem')
        .map(item => item.textContent)
    ).toEqual([expect.stringMatching(/^carol/), expect.stringMatching(/^dave/)]);
    expect(screen.getByRole('radio', { name: 'Add to a team' })).toBeChecked();
    expect(await screen.findByRole('combobox', { name: 'Team' })).toHaveValue('platform');
    expect(api.to('POST', '/operations')).toHaveLength(0);
  });

  it('offers GitHub sign-in when a progress refresh fails after the authorization expired', async () => {
    let reads = 0;
    mockApi({
      'GET /operations/:id': () => {
        if (++reads === 1) return { data: running };
        throw new ApiError({
          message: 'raw',
          status: 401,
          code: 'github_reauthentication_required',
        });
      },
    });
    renderPeople({ search: `?receipt=${receiptId(1)}`, advanceTimers: vi.advanceTimersByTime });
    await screen.findByText('In progress. Gitorgasm makes one change at a time.');

    await act(() => vi.advanceTimersByTimeAsync(3_000));

    expect(
      await screen.findByText(/Sign in with GitHub again\. Showing the last update\./)
    ).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Sign in with GitHub' })).toHaveAttribute(
      'href',
      '/api/v1/auth/login'
    );
    expect(screen.getByText('In progress. Gitorgasm makes one change at a time.')).toBeVisible();
  });

  it('keeps the last update and offers no sign-in when a progress refresh fails for another reason', async () => {
    let reads = 0;
    mockApi({
      'GET /operations/:id': () => {
        if (++reads === 1) return { data: running };
        throw new ApiError({ message: 'raw', status: 502 });
      },
    });
    renderPeople({ search: `?receipt=${receiptId(1)}`, advanceTimers: vi.advanceTimersByTime });
    await screen.findByText('In progress. Gitorgasm makes one change at a time.');

    await act(() => vi.advanceTimersByTimeAsync(3_000));

    expect(
      await screen.findByText('Could not refresh the progress. Showing the last update.')
    ).toBeInTheDocument();
    expect(screen.queryByRole('link', { name: 'Sign in with GitHub' })).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Check progress' })).toBeInTheDocument();
    expect(screen.getByText('In progress. Gitorgasm makes one change at a time.')).toBeVisible();
  });
});

describe('recent requests', () => {
  it('opens a receipt from the list so a lost response can be recovered', async () => {
    const onUrlUpdate = vi.fn();
    const summary = { ...operation(), results: undefined };
    mockApi({
      'GET /operations': () => ({ data: [summary], next_cursor: null }),
      'GET /operations/:id': () => ({ data: operation() }),
    });
    const { user } = renderPeople({ onUrlUpdate });

    await user.click(await screen.findByRole('link', { name: /Invite to organization/ }));

    expect(await screen.findByRole('heading', { name: 'Request progress' })).toBeInTheDocument();
    expect(onUrlUpdate.mock.calls.at(-1)?.[0].searchParams.get('receipt')).toBe(receiptId(1));
  });

  it('shows the empty state', async () => {
    mockApi();
    renderPeople();

    expect(
      await screen.findByText('No requests yet. Requests that you send appear here.')
    ).toBeInTheDocument();
  });
});
