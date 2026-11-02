import { describe, expect, it } from 'vitest';

import { blockedReason, mergePeople, togglePerson } from '../people';
import type { Person } from '../types';

const actor = { github_id: 7, login: 'Owner' };

function person(login: string, overrides: Partial<Person> = {}): Person {
  return { login, key: login.toLowerCase(), id: null, member: false, ...overrides };
}

describe('mergePeople', () => {
  it('deduplicates case-insensitively and keeps the first casing and position', () => {
    const result = mergePeople(
      [person('Alice'), person('bob')],
      [
        { login: 'carol', id: null, member: false },
        { login: 'ALICE', id: null, member: false },
        { login: 'Carol', id: null, member: false },
      ],
      50
    );

    expect(result.people.map(p => p.login)).toEqual(['Alice', 'bob', 'carol']);
    expect(result).toMatchObject({ added: 1, duplicates: 2, total: 3, overflow: false });
  });

  it('upgrades an existing person that a directory hit proves is a member', () => {
    const result = mergePeople([person('Alice')], [{ login: 'alice', id: 11, member: true }], 50);

    expect(result.people).toEqual([person('Alice', { id: 11, member: true })]);
  });

  it('adds no one when the list would pass the limit and reports the would-be total', () => {
    const existing = [person('a'), person('b')];
    const result = mergePeople(
      existing,
      [
        { login: 'c', id: null, member: false },
        { login: 'd', id: null, member: false },
      ],
      3
    );

    expect(result).toMatchObject({ overflow: true, total: 4, added: 0 });
    expect(result.people).toEqual(existing);
  });
});

describe('blockedReason', () => {
  it('blocks the actor in a role request by GitHub ID and by login in another case', () => {
    expect(blockedReason(person('someone', { id: 7 }), 'organization_role', actor)).toBe('self');
    expect(blockedReason(person('OWNER'), 'organization_role', actor)).toBe('self');
  });

  it('allows the actor in a team request', () => {
    expect(blockedReason(person('owner', { id: 7 }), 'team_membership', actor)).toBeNull();
  });

  it('blocks inviting a current member', () => {
    expect(blockedReason(person('alice', { member: true }), 'invite', actor)).toBe('member');
    expect(blockedReason(person('alice', { member: true }), 'team_membership', actor)).toBeNull();
  });
});

describe('togglePerson', () => {
  it('removes by key regardless of casing when unchecked', () => {
    const next = togglePerson(
      [person('Alice'), person('bob')],
      { login: 'ALICE', id: null, member: false },
      false,
      50
    );

    expect(next.map(p => p.login)).toEqual(['bob']);
  });
});
