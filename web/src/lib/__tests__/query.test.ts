import { partialMatchKey } from '@tanstack/react-query';
import { describe, expect, it } from 'vitest';

import { createQueryKeys, getQueryClient } from '../query';

describe('getQueryClient', () => {
  it('reuses one client in the browser', () => {
    expect(getQueryClient()).toBe(getQueryClient());
  });
});

describe('createQueryKeys', () => {
  const keys = createQueryKeys('orgs');

  it('lets a prefix key match every key below it', () => {
    expect(partialMatchKey(keys.list({ cursor: 'c' }), keys.lists())).toBe(true);
    expect(partialMatchKey(keys.detail('acme'), keys.details())).toBe(true);
    expect(partialMatchKey(keys.detail('acme'), keys.all())).toBe(true);
  });

  it('keeps list keys apart from a detail whose id is `list`', () => {
    expect(partialMatchKey(keys.detail('list'), keys.lists())).toBe(false);
  });
});
