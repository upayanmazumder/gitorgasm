import { screen } from '@testing-library/react';
import axe from 'axe-core';
import { afterEach, describe, expect, it, vi } from 'vitest';

import {
  choose,
  mockApi,
  operation,
  paste,
  receiptId,
  renderPeople,
  review,
} from './people-test-helpers';

afterEach(() => vi.restoreAllMocks());

// jsdom has no layout, so color contrast is checked in the browser flows instead.
async function violations() {
  const results = await axe.run(screen.getByRole('main'), {
    rules: { 'color-contrast': { enabled: false } },
  });
  return results.violations.map(
    violation => `${violation.id}: ${violation.nodes.map(node => node.target.join(' ')).join(', ')}`
  );
}

describe('People workspace accessibility', () => {
  it('has no violations in the editor with every source', async () => {
    mockApi({ 'GET /members': () => ({ data: [{ id: 11, login: 'alice' }], next_cursor: null }) });
    const { user } = renderPeople();
    await paste(user, 'alice not/valid');

    expect(await violations()).toEqual([]);

    await choose(user, 'Change organization role');
    await choose(user, 'Current members');
    await screen.findByRole('checkbox', { name: 'alice' });
    expect(await violations()).toEqual([]);
  });

  it('has no violations in the owner review', async () => {
    mockApi();
    const { user } = renderPeople();
    await choose(user, 'Change organization role');
    await paste(user, 'alice bob');
    await choose(user, 'Organization owner');
    await review(user);
    await user.click(screen.getByRole('button', { name: 'Make 2 people owners' }));

    expect(await screen.findByRole('alert')).toBeInTheDocument();
    expect(await violations()).toEqual([]);
  });

  it('has no violations in a receipt with every result group', async () => {
    mockApi({
      'GET /operations/:id': () => ({
        data: operation({
          status: 'interrupted',
          completed_at: '2026-11-03T10:01:00Z',
          target_count: 4,
          results: [
            { login: 'alice', status: 'invited', code: null, message: null },
            {
              login: 'bob',
              status: 'failed',
              code: 'x',
              message: 'GitHub rejected this invitation.',
            },
            { login: 'carol', status: 'unknown', code: 'y', message: 'Check GitHub.' },
            { login: 'dave', status: 'not_attempted', code: 'z', message: null },
          ],
        }),
      }),
    });
    renderPeople({ search: `?receipt=${receiptId(1)}` });

    expect(await screen.findByText(/Stopped before it finished/)).toBeInTheDocument();
    expect(await violations()).toEqual([]);
  });
});
