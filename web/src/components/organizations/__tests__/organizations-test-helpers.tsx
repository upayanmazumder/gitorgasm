import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render } from '@testing-library/react';

import { OrganizationsView } from '../organizations-view';

export const user = { github_id: 7, login: 'alice', name: 'Alice', avatar_url: null };
export const organization = {
  login: 'acme',
  repository_count: 2,
  suspended: false,
  last_synced_at: null,
};

export function setup() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={client}>
      <OrganizationsView />
    </QueryClientProvider>
  );
  return client;
}
