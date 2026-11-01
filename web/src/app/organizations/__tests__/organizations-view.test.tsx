import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { api, ApiError } from '@/api';

import { OrganizationsView } from '../organizations-view';

const user = { github_id: 7, login: 'alice', name: 'Alice', avatar_url: null };
const organization = { login: 'acme', repository_count: 2, suspended: false, last_synced_at: null };

function setup() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={client}>
      <OrganizationsView />
    </QueryClientProvider>
  );
  return client;
}

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

  it('exposes real installation and refresh links and removes private cache after logout', async () => {
    vi.spyOn(api, 'request').mockImplementation(async config => ({
      data:
        config.url === '/v1/me'
          ? user
          : config.url === '/v1/organizations'
            ? { data: [organization] }
            : undefined,
    }));
    const client = setup();
    expect(await screen.findByRole('link', { name: /acme/ })).toHaveAttribute(
      'href',
      '/organizations/acme'
    );
    expect(screen.getByRole('link', { name: 'Install GitHub App' })).toHaveAttribute(
      'href',
      '/api/v1/auth/install'
    );
    expect(screen.getByRole('link', { name: 'Refresh GitHub access' })).toHaveAttribute(
      'href',
      '/api/v1/auth/login'
    );
    client.setQueryData(['scores', 'acme'], { private: true });
    await userEvent.setup().click(screen.getByRole('button', { name: 'Sign out' }));
    expect(await screen.findByRole('link', { name: 'Sign in with GitHub' })).toBeInTheDocument();
    expect(screen.queryByRole('link', { name: /acme/ })).not.toBeInTheDocument();
    expect(client.getQueryData(['scores', 'acme'])).toBeUndefined();
  });

  it('keeps identity and organizations visible when logout fails', async () => {
    vi.spyOn(api, 'request').mockImplementation(async config => {
      if (config.url === '/v1/auth/logout')
        throw new ApiError({ message: 'Unavailable', status: 503 });
      return { data: config.url === '/v1/me' ? user : { data: [organization] } };
    });
    setup();
    await screen.findByRole('link', { name: /acme/ });
    await userEvent.setup().click(screen.getByRole('button', { name: 'Sign out' }));
    expect(await screen.findByRole('alert')).toHaveTextContent('Could not sign out');
    await waitFor(() => expect(screen.getByRole('button', { name: 'Sign out' })).toBeEnabled());
    expect(screen.getByRole('link', { name: /acme/ })).toBeInTheDocument();
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
});
