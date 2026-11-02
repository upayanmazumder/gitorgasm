import { screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { ApiError } from '@/api';

import { capability, mockApi, renderPeople } from './people-test-helpers';

afterEach(() => vi.restoreAllMocks());

function fail(status: number | undefined, code: string) {
  return () => {
    throw new ApiError({ message: 'SECRET upstream detail', status, code });
  };
}

describe('People access gate', () => {
  it.each([
    ['a signed-out session', 401, 'unauthenticated', 'Sign in to manage people'],
    [
      'an expired GitHub authorization',
      401,
      'github_reauthentication_required',
      'Sign in with GitHub again',
    ],
    ['an organization the caller cannot see', 404, 'not_found', 'Organization not available'],
  ])('explains %s and offers the next step', async (_name, status, code, heading) => {
    mockApi({ 'GET /': fail(status, code) });
    renderPeople();

    expect(await screen.findByRole('heading', { name: heading })).toBeInTheDocument();
    expect(screen.queryByText(/SECRET/)).not.toBeInTheDocument();
    expect(screen.queryByRole('heading', { name: 'New request' })).not.toBeInTheDocument();
  });

  it('sends a reauthorization through the full-page sign-in route', async () => {
    mockApi({ 'GET /': fail(401, 'github_reauthentication_required') });
    renderPeople();

    expect(await screen.findByRole('link', { name: 'Sign in with GitHub' })).toHaveAttribute(
      'href',
      '/api/v1/auth/login'
    );
  });

  it.each([
    ['not_owner', 'Only owners can manage people'],
    ['members_permission_required', 'The GitHub App needs member access'],
    ['installation_suspended', 'The GitHub App is suspended'],
    ['unsupported_account', 'People management is not available'],
  ] as const)('blocks the workspace for %s', async (reason, heading) => {
    mockApi({ 'GET /': () => ({ data: { ...capability, can_manage: false, reason } }) });
    renderPeople();

    expect(await screen.findByRole('heading', { name: heading })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Review request' })).not.toBeInTheDocument();
  });

  it('shows an outage as retryable and never as a signed-out page', async () => {
    mockApi({ 'GET /': fail(502, 'ERR_BAD_RESPONSE') });
    renderPeople();

    expect(await screen.findByRole('alert')).toHaveTextContent(/temporarily unavailable/);
    expect(screen.getByRole('button', { name: 'Try again' })).toBeEnabled();
    expect(screen.queryByRole('link', { name: 'Sign in with GitHub' })).not.toBeInTheDocument();
    expect(screen.queryByText(/SECRET/)).not.toBeInTheDocument();
  });

  it('marks the People link as the current section', async () => {
    mockApi();
    renderPeople();

    expect(await screen.findByRole('heading', { name: 'New request' })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'People' })).toHaveAttribute('aria-current', 'page');
    expect(screen.getByRole('link', { name: 'Scores' })).not.toHaveAttribute('aria-current');
    expect(screen.getByRole('link', { name: 'Scores' })).toHaveAttribute(
      'href',
      '/organizations/acme'
    );
  });
});
