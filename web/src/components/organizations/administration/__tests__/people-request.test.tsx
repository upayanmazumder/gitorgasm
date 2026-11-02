import { screen, waitFor, within } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { choose, mockApi, operation, paste, renderPeople, review } from './people-test-helpers';

afterEach(() => vi.restoreAllMocks());

const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

function listed(name: string) {
  return within(screen.getByRole('list', { name }))
    .getAllByRole('listitem')
    .map(item => item.textContent);
}

describe('People request review', () => {
  it('sends nothing until the exact review is confirmed, then sends it once with a key', async () => {
    const api = mockApi({
      'POST /operations': () => ({ data: operation() }),
      'GET /operations/:id': () => ({ data: operation() }),
    });
    const { user } = renderPeople();
    await paste(user, 'alice @Bob alice');
    await review(user);

    expect(listed('People in this request')).toEqual(['alice', 'Bob']);
    expect(screen.getByText('Invite 2 people to acme as members.')).toBeInTheDocument();
    expect(api.to('POST', '/operations')).toHaveLength(0);

    await user.click(screen.getByRole('button', { name: 'Send 2 invitations' }));

    await waitFor(() => expect(api.to('POST', '/operations')).toHaveLength(1));
    const [post] = api.to('POST', '/operations');
    expect(post.data).toEqual({ kind: 'invite', logins: ['alice', 'Bob'] });
    expect(post.headers['Idempotency-Key']).toMatch(UUID);
  });

  it('discards the confirmation on edit and reviews the new exact role', async () => {
    const api = mockApi();
    const { user } = renderPeople();
    await choose(user, 'Change organization role');
    await paste(user, 'alice bob');
    await choose(user, 'Organization owner');
    await review(user);

    expect(
      screen.getByText(/Organization owners get complete administrative access to acme/)
    ).toBeInTheDocument();
    expect(screen.getByText(/GitHub may send them an invitation/)).toBeInTheDocument();
    const acknowledgment = screen.getByRole('checkbox', {
      name: 'I understand that 2 people will get complete administrative access to acme.',
    });

    await user.click(screen.getByRole('button', { name: 'Make 2 people owners' }));
    expect(await screen.findByRole('alert')).toHaveTextContent(
      'Select the confirmation box to continue.'
    );
    await waitFor(() => expect(acknowledgment).toHaveFocus());
    expect(api.to('POST', '/operations')).toHaveLength(0);

    await user.click(acknowledgment);
    await user.click(screen.getByRole('button', { name: 'Edit request' }));
    await choose(user, 'Member');
    await review(user);

    expect(screen.queryByRole('checkbox')).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Set 2 people to Member' })).toBeInTheDocument();
    expect(
      screen.getByText(/Owners in this request lose owner access to acme/)
    ).toBeInTheDocument();
    expect(screen.getByText(/GitHub may send them an invitation/)).toBeInTheDocument();
    expect(api.to('POST', '/operations')).toHaveLength(0);
  });

  it('confirms a team request that may invite people outside the organization', async () => {
    const api = mockApi({
      'GET /members': () => ({ data: [{ id: 11, login: 'alice' }], next_cursor: null }),
      'POST /operations': () => ({
        data: operation({ kind: 'team_membership', team_slug: 'platform' }),
      }),
      'GET /operations/:id': () => ({ data: operation({ kind: 'team_membership' }) }),
    });
    const { user } = renderPeople();
    await choose(user, 'Add to a team');
    await user.selectOptions(await screen.findByRole('combobox', { name: 'Team' }), 'Platform');
    await user.click(await screen.findByRole('checkbox', { name: 'alice' }));
    await paste(user, 'carol');

    expect(
      screen.getByText(/Not confirmed as a member\. GitHub may invite this person/)
    ).toBeInTheDocument();
    await review(user);

    expect(screen.getByText('Add 2 people to the Platform team in acme.')).toBeInTheDocument();
    expect(screen.getByText(/1 person is not confirmed as members of acme/)).toBeInTheDocument();
    // The note names only the person who is not a confirmed member.
    const [alice, carol] = within(
      screen.getByRole('list', { name: 'People in this request' })
    ).getAllByRole('listitem');
    expect(alice).toHaveTextContent(/^alice$/);
    expect(carol).toHaveTextContent(
      'carolNot confirmed as a member. GitHub may invite this person to acme.'
    );
    await user.click(screen.getByRole('button', { name: 'Add 2 people to Platform' }));
    expect(await screen.findByRole('alert')).toHaveTextContent('Select the confirmation box');
    expect(api.to('POST', '/operations')).toHaveLength(0);

    await user.click(
      screen.getByRole('checkbox', {
        name: 'I understand that GitHub may invite 1 person to acme.',
      })
    );
    await user.click(screen.getByRole('button', { name: 'Add 2 people to Platform' }));

    await waitFor(() => expect(api.to('POST', '/operations')).toHaveLength(1));
    expect(api.to('POST', '/operations')[0].data).toEqual({
      kind: 'team_membership',
      logins: ['alice', 'carol'],
      team_slug: 'platform',
    });
  });

  it('blocks a pasted copy of the actor login from a role request until it is removed', async () => {
    const api = mockApi();
    const { user } = renderPeople();
    await choose(user, 'Change organization role');
    await paste(user, 'OWNER bob');

    expect(
      screen.getByText(/Cannot include: this is you\. You cannot change your own role\./)
    ).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Review request' }));

    expect(await screen.findByText('Fix these problems, then review again.')).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'New request' })).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Remove 1 person' }));

    expect(listed('Selected people')).toHaveLength(1);
    expect(screen.queryByText(/Cannot include/)).not.toBeInTheDocument();
    expect(api.to('POST', '/operations')).toHaveLength(0);
  });

  it('refuses to invite a current member', async () => {
    mockApi({
      'GET /members': () => ({ data: [{ id: 11, login: 'alice' }], next_cursor: null }),
    });
    const { user } = renderPeople();
    await choose(user, 'Add to a team');
    await user.click(await screen.findByRole('checkbox', { name: 'alice' }));
    await choose(user, 'Invite to the organization');

    expect(screen.getByText('Cannot invite: already a member of acme.')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Review request' }));
    expect(await screen.findByText('Fix these problems, then review again.')).toBeInTheDocument();
  });
});
