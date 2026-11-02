import { screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { api, ApiError } from '@/api';

import { organization, setup, user } from './organizations-test-helpers';

afterEach(() => vi.restoreAllMocks());

describe('organization authorization', () => {
  it('offers sign-in without fetching organizations for a signed-out visitor', async () => {
    const request = vi
      .spyOn(api, 'request')
      .mockRejectedValue(new ApiError({ message: 'Unauthorized', status: 401 }));
    setup();
    expect(await screen.findByRole('link', { name: 'Sign in with GitHub' })).toHaveAttribute(
      'href',
      '/api/v1/auth/login'
    );
    expect(request).toHaveBeenCalledTimes(1);
    expect(screen.queryByRole('link', { name: 'Install GitHub App' })).not.toBeInTheDocument();
  });

  it('keeps API failures distinct from being signed out and allows retry', async () => {
    vi.spyOn(api, 'request')
      .mockRejectedValueOnce(new ApiError({ message: 'Unavailable', status: 503 }))
      .mockResolvedValue({ data: user });
    setup();
    expect(await screen.findByRole('alert')).toHaveTextContent('Could not check your session');
    expect(screen.queryByRole('link', { name: 'Sign in with GitHub' })).not.toBeInTheDocument();
    await userEvent.setup().click(screen.getByRole('button', { name: 'Try again' }));
    expect(await screen.findByText('@alice')).toBeInTheDocument();
  });

  it('hides cached organizations when the session expires during a list refresh', async () => {
    const request = vi.spyOn(api, 'request').mockImplementation(async config => ({
      data: config.url === '/v1/me' ? user : { data: [organization] },
    }));
    setup();
    await screen.findByRole('link', { name: /acme/ });
    request.mockRejectedValue(new ApiError({ message: 'Unauthorized', status: 401 }));
    await userEvent.setup().click(screen.getByRole('button', { name: 'Check organizations' }));
    expect(await screen.findByRole('link', { name: 'Sign in with GitHub' })).toBeInTheDocument();
    expect(screen.queryByRole('link', { name: /acme/ })).not.toBeInTheDocument();
  });

  it('reports an unreachable service on the session check and recovers on an explicit retry', async () => {
    const retried = Promise.withResolvers<{ data: typeof user }>();
    vi.spyOn(api, 'request')
      .mockRejectedValueOnce(
        new ApiError({
          message: 'Request failed with status code 500',
          status: 500,
          code: 'ERR_BAD_RESPONSE',
        })
      )
      .mockReturnValueOnce(retried.promise)
      .mockImplementation(async config => ({
        data: config.url === '/v1/me' ? user : { data: [organization] },
      }));
    setup();
    expect(await screen.findByRole('alert')).toHaveTextContent(/temporarily unavailable/);
    expect(screen.queryByRole('link', { name: 'Sign in with GitHub' })).not.toBeInTheDocument();
    expect(screen.queryByText(/status code/i)).not.toBeInTheDocument();

    await userEvent.setup().click(screen.getByRole('button', { name: 'Try again' }));
    // A refetch of a query that never had data returns to pending, so the alert and button leave.
    expect(await screen.findByText('Checking your session…')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Try again' })).not.toBeInTheDocument();
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    expect(screen.queryByRole('link', { name: 'Sign in with GitHub' })).not.toBeInTheDocument();

    retried.resolve({ data: user });
    expect(await screen.findByText('@alice')).toBeInTheDocument();
    expect(await screen.findByRole('link', { name: /acme/ })).toBeInTheDocument();
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
  });

  it('keeps the identity visible when the organization list is unreachable and recovers on Check organizations', async () => {
    const request = vi.spyOn(api, 'request').mockImplementation(async config => {
      if (config.url === '/v1/me') return { data: user };
      throw new ApiError({ message: 'Network Error', code: 'ERR_NETWORK' });
    });
    setup();
    expect(await screen.findByText('@alice')).toBeInTheDocument();
    expect(await screen.findByRole('alert')).toHaveTextContent(
      /organizations\. The service may be temporarily unavailable/
    );
    expect(screen.queryByRole('link', { name: 'Sign in with GitHub' })).not.toBeInTheDocument();

    request.mockImplementation(async config => ({
      data: config.url === '/v1/me' ? user : { data: [organization] },
    }));
    await userEvent.setup().click(screen.getByRole('button', { name: 'Check organizations' }));
    expect(await screen.findByRole('link', { name: /acme/ })).toBeInTheDocument();
  });
});
