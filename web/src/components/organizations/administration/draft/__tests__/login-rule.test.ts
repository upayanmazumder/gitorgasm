import { describe, expect, it } from 'vitest';

import { normalizeLogin } from '../login-rule';

describe('normalizeLogin', () => {
  it.each([
    ['Octocat', 'Octocat'],
    ['@octocat', 'octocat'],
    ['  alice  ', 'alice'],
    ['mona_acme', 'mona_acme'],
    ['a-b', 'a-b'],
  ])('accepts %j as %j', (raw, expected) => {
    expect(normalizeLogin(raw)).toBe(expected);
  });

  it.each([
    'a/b',
    '../x',
    'x?y',
    'x#y',
    'x%2Fy',
    'a.b',
    'a b',
    'https://github.com/x',
    '',
    '@',
    '@@alice',
    '-alice',
    'a'.repeat(101),
  ])('rejects %j so it cannot alter an API path', raw => {
    expect(normalizeLogin(raw)).toBeNull();
  });

  it('accepts the longest login the API accepts', () => {
    expect(normalizeLogin('a'.repeat(100))).toBe('a'.repeat(100));
  });
});
