import { screen, within } from '@testing-library/react';
import type { UserEvent } from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { ApiError } from '@/api';

import { actor, choose, mockApi, renderPeople, type Call } from './people-test-helpers';

afterEach(() => vi.restoreAllMocks());

function selected() {
  return within(screen.getByRole('list', { name: 'Selected people' }))
    .getAllByRole('listitem')
    .map(item => item.textContent);
}

describe('current member directory', () => {
  const members = {
    'GET /members': ({ params }: Call) => {
      if (params.role === 'admin') return { data: [{ id: 21, login: 'zed' }], next_cursor: null };
      if (params.cursor === 'c2') return { data: [{ id: 12, login: 'carol' }], next_cursor: null };
      return {
        data: [
          { id: actor.github_id, login: actor.login },
          { id: 11, login: 'alice' },
        ],
        next_cursor: 'c2',
      };
    },
  };

  it('loads the next page only on request and keeps the selection across pages and filters', async () => {
    const api = mockApi(members);
    const { user } = renderPeople();
    await choose(user, 'Add to a team');
    await user.click(await screen.findByRole('checkbox', { name: 'alice' }));

    expect(api.to('GET', '/members')).toHaveLength(1);
    await user.click(screen.getByRole('button', { name: 'Show more members' }));
    await user.click(await screen.findByRole('checkbox', { name: 'carol' }));

    expect(api.to('GET', '/members')[1].params.cursor).toBe('c2');
    expect(screen.getByText('2 of 50 selected')).toBeInTheDocument();

    await user.selectOptions(screen.getByRole('combobox', { name: 'Show' }), 'Owners');
    await screen.findByRole('checkbox', { name: 'zed' });

    expect(api.to('GET', '/members')[2].params.role).toBe('admin');
    expect(screen.getByText('2 of 50 selected')).toBeInTheDocument();
    expect(selected().join(' ')).toMatch(/alice.*carol/);
  });

  it('disables the actor row in a role request and explains why', async () => {
    mockApi(members);
    const { user } = renderPeople();
    await choose(user, 'Change organization role');

    const self = await screen.findByRole('checkbox', { name: 'Owner (you)' });
    expect(self).toBeDisabled();
    expect(self).toHaveAccessibleDescription('You cannot change your own role.');
    expect(screen.getByRole('checkbox', { name: 'alice' })).toBeEnabled();
  });
});

describe('public search', () => {
  async function search(user: UserEvent, query: string) {
    await user.type(await screen.findByRole('textbox', { name: 'Username or name' }), query);
    await user.click(screen.getByRole('button', { name: 'Search' }));
  }

  it('sends a request only when the form is submitted and pages only on request', async () => {
    const api = mockApi({
      'GET /users': ({ params }) => ({
        data: [params.cursor ? { id: 2, login: 'octodog' } : { id: 1, login: 'octocat' }],
        next_cursor: params.cursor ? null : 'n2',
        incomplete_results: !params.cursor,
      }),
    });
    const { user } = renderPeople();
    await user.type(await screen.findByRole('textbox', { name: 'Username or name' }), 'octo');

    expect(api.to('GET', '/users')).toHaveLength(0);
    await user.click(screen.getByRole('button', { name: 'Search' }));

    expect(await screen.findByRole('checkbox', { name: 'octocat' })).toBeInTheDocument();
    expect(screen.getByText(/GitHub returned partial results/)).toBeInTheDocument();
    expect(api.to('GET', '/users')).toHaveLength(1);
    expect(api.to('GET', '/users')[0].params).toMatchObject({ q: 'octo' });

    await user.click(screen.getByRole('button', { name: 'Show more results' }));
    await screen.findByRole('checkbox', { name: 'octodog' });
    expect(api.to('GET', '/users')[1].params.cursor).toBe('n2');

    await user.click(screen.getByRole('checkbox', { name: 'octocat' }));
    expect(selected()[0]).toMatch(/^octocat/);
  });

  it('rejects a query the API would refuse without sending it', async () => {
    const api = mockApi();
    const { user } = renderPeople();
    await search(user, 'a');

    expect(await screen.findByText('Enter at least 2 characters.')).toBeInTheDocument();
    expect(api.to('GET', '/users')).toHaveLength(0);
  });

  it('explains the search quota and points to pasting instead', async () => {
    mockApi({
      'GET /users': () => {
        throw new ApiError({
          message: 'raw upstream text',
          status: 503,
          code: 'github_rate_limited',
        });
      },
    });
    const { user } = renderPeople();
    await search(user, 'octo');

    expect(await screen.findByRole('alert')).toHaveTextContent(
      'GitHub search limit reached. Wait a minute, then search again. You can paste usernames instead.'
    );
    expect(screen.queryByText(/raw upstream text/)).not.toBeInTheDocument();
  });
});

describe('imports', () => {
  async function upload(user: UserEvent, text: string, name: string) {
    await choose(user, 'Paste or import');
    const type = name.endsWith('.json') ? 'application/json' : 'text/csv';
    await user.upload(screen.getByLabelText('CSV or JSON file'), new File([text], name, { type }));
  }

  it('adds valid CSV rows, lists the invalid ones, and dedupes by case', async () => {
    mockApi();
    const { user } = renderPeople();
    await upload(user, '\uFEFFusername,name\nalice,A\nbad/name,B\nBOB,C\nbob,D\n', 'people.csv');

    expect(
      await screen.findByText('Added 2 people from people.csv. 1 were already in the list.')
    ).toBeInTheDocument();
    expect(
      screen.getByText(/1 entries in people\.csv are not valid GitHub usernames/)
    ).toBeInTheDocument();
    expect(screen.getByText('Row 3: “bad/name”')).toBeInTheDocument();
    expect(selected()).toEqual(['aliceRemove', 'BOBRemove']);
  });

  it('adds no one when a file would pass the limit of 50', async () => {
    mockApi();
    const { user } = renderPeople();
    const rows = Array.from({ length: 51 }, (_, index) => `user${index}`).join('\n');
    await upload(user, `login\n${rows}`, 'big.csv');

    expect(await screen.findByRole('alert')).toHaveTextContent(
      'This would make 51 people. A request can include at most 50.'
    );
    expect(screen.getByText('0 of 50 selected')).toBeInTheDocument();
  });

  it('reads JSON records and leaves invalid pasted tokens in the box to correct', async () => {
    mockApi();
    const { user } = renderPeople();
    await upload(user, JSON.stringify([{ login: 'carol' }, 'dave']), 'people.json');
    await screen.findByText('Added 2 people from people.json.');

    const box = screen.getByRole('textbox', { name: 'Usernames' });
    await user.type(box, 'erin not/valid');
    await user.click(screen.getByRole('button', { name: 'Add usernames' }));

    expect(await screen.findByText('Added 1 person from the pasted text.')).toBeInTheDocument();
    expect(screen.getByText('3 of 50 selected')).toBeInTheDocument();
    expect(box).toHaveValue('not/valid');
  });
});
