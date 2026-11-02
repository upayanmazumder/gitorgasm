import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen } from '@testing-library/react';
import userEvent, { type UserEvent } from '@testing-library/user-event';
import { NuqsTestingAdapter, type OnUrlUpdateFunction } from 'nuqs/adapters/testing';
import { vi } from 'vitest';

import { api } from '@/api';

import type { Capability, Operation } from '../data';
import { PeopleView } from '../people-view';

export const actor = { github_id: 7, login: 'Owner' };

export const capability: Capability = {
  organization: 'acme',
  actor,
  can_manage: true,
  reason: null,
  max_batch_size: 50,
};

// Receipt IDs are 43 URL-safe characters.
export const receiptId = (n: number) => `receipt${n}`.padEnd(43, 'x');

export function operation(overrides: Partial<Operation> = {}): Operation {
  return {
    id: receiptId(1),
    kind: 'invite',
    status: 'queued',
    team_slug: null,
    role: null,
    target_count: 2,
    created_at: '2026-11-03T10:00:00Z',
    completed_at: null,
    results: [
      { login: 'alice', status: 'queued', code: null, message: null },
      { login: 'bob', status: 'queued', code: null, message: null },
    ],
    ...overrides,
  };
}

export interface Call {
  method: string;
  route: string;
  params: Record<string, string | undefined>;
  headers: Record<string, string>;
  data: unknown;
}

type Handler = (call: Call) => unknown;

const defaults: Record<string, Handler> = {
  'GET /': () => ({ data: capability }),
  'GET /operations': () => ({ data: [], next_cursor: null }),
  'GET /teams': () => ({
    data: [{ id: 1, name: 'Platform', slug: 'platform' }],
    next_cursor: null,
  }),
  'GET /members': () => ({ data: [], next_cursor: null }),
};

function describeCall(config: {
  url?: string;
  method?: string;
  params?: unknown;
  headers?: unknown;
  data?: unknown;
}): Call {
  const suffix = (config.url ?? '').split('/administration')[1] ?? '';
  const route = suffix.replace(/^\/operations\/[^/]+$/, '/operations/:id') || '/';
  return {
    method: (config.method ?? 'get').toUpperCase(),
    route,
    params: (config.params ?? {}) as Call['params'],
    headers: (config.headers ?? {}) as Call['headers'],
    data: config.data,
  };
}

// Routes are "METHOD /suffix" under /administration. A handler returns the body or throws an ApiError.
export function mockApi(routes: Record<string, Handler> = {}) {
  const calls: Call[] = [];
  const table = { ...defaults, ...routes };
  vi.spyOn(api, 'request').mockImplementation(async config => {
    const call = describeCall(config);
    calls.push(call);
    const handler = table[`${call.method} ${call.route}`];
    if (!handler) throw new Error(`Unhandled ${call.method} ${call.route}`);
    return { data: await handler(call), status: 200 } as never;
  });
  return {
    calls,
    to: (method: string, route: string) =>
      calls.filter(call => call.method === method && call.route === route),
  };
}

interface Options {
  search?: string;
  onUrlUpdate?: OnUrlUpdateFunction;
  advanceTimers?: (ms: number) => void;
}

export function renderPeople({ search = '', onUrlUpdate, advanceTimers }: Options = {}) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const user = advanceTimers ? userEvent.setup({ advanceTimers }) : userEvent.setup();
  render(
    <NuqsTestingAdapter searchParams={search} onUrlUpdate={onUrlUpdate} hasMemory>
      <QueryClientProvider client={client}>
        <PeopleView login="acme" />
      </QueryClientProvider>
    </NuqsTestingAdapter>
  );
  return { user, client };
}

export async function choose(user: UserEvent, radio: string) {
  await user.click(await screen.findByRole('radio', { name: radio }));
}

export async function paste(user: UserEvent, usernames: string) {
  await choose(user, 'Paste or import');
  await user.type(screen.getByRole('textbox', { name: 'Usernames' }), usernames);
  await user.click(screen.getByRole('button', { name: 'Add usernames' }));
}

export async function review(user: UserEvent) {
  await user.click(screen.getByRole('button', { name: 'Review request' }));
  await screen.findByRole('heading', { name: 'Review request' });
}
