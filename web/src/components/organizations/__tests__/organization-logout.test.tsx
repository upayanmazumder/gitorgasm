import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { api, ApiError } from '@/api';

import { organization, setup, user } from './organizations-test-helpers';

afterEach(() => vi.restoreAllMocks());

describe('organization logout', () => {
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
});
