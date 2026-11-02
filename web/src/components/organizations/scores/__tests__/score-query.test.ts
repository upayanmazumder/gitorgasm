import { describe, expect, it } from 'vitest';

import { ApiError } from '@/api';

import { defaultForm, fromQuery, toQuery } from '../form';
import { errorMessage } from '../state';

describe('toQuery', () => {
  it('leaves named periods unanchored so the server uses the latest ingest', () => {
    expect(toQuery({ ...defaultForm, period: 'last7_days' }).period).toEqual({
      kind: 'preset',
      preset: 'last7_days',
    });
  });

  it('rejects an empty or reversed custom range', () => {
    const custom = { ...defaultForm, period: 'custom' };
    expect(() => toQuery({ ...custom, start: '', end: '' })).toThrow(/start time/);
    expect(() =>
      toQuery({ ...custom, start: '2026-02-01T00:00', end: '2026-01-01T00:00' })
    ).toThrow(/start time/);
  });

  it('rejects invalid user IDs and rules', () => {
    expect(() => toQuery({ ...defaultForm, userId: '1.5' })).toThrow(/user ID/);
    expect(() => toQuery({ ...defaultForm, rules: '{' })).toThrow(/valid JSON/);
  });

  it('round-trips through a saved preset', () => {
    const form = {
      ...defaultForm,
      period: 'custom',
      start: '2026-01-01T09:30',
      end: '2026-01-02T09:30',
      category: 'reviews',
      search: 'alice',
      repository: 'acme/api',
      userId: '7',
      kinds: 'review, commit',
      excludeBots: false,
      rules: '{"spamPenalty":0}',
    };
    expect(fromQuery(toQuery(form))).toEqual({
      ...form,
      rules: JSON.stringify({ spamPenalty: 0 }, null, 2),
    });
  });
});

describe('errorMessage', () => {
  it('maps API error codes and statuses', () => {
    expect(
      errorMessage(new ApiError({ message: 'x', status: 409, code: 'preset_limit' }), 'f')
    ).toMatch(/maximum number of presets/);
    expect(errorMessage(new ApiError({ message: 'x', status: 401 }), 'f')).toMatch(/Sign in/);
    expect(
      errorMessage(new ApiError({ message: 'x', status: 500, code: 'internal' }), 'fallback')
    ).toBe('fallback');
  });

  it('reports an unreachable service without leaking transport details', () => {
    const gateway = errorMessage(
      new ApiError({
        message: 'Request failed with status code 502',
        status: 502,
        code: 'ERR_BAD_RESPONSE',
      }),
      'f'
    );
    expect(gateway).toMatch(/temporarily unavailable/);
    expect(gateway).not.toContain('status code');
    const offline = errorMessage(
      new ApiError({ message: 'Network Error', code: 'ERR_NETWORK' }),
      'f'
    );
    expect(offline).toMatch(/temporarily unavailable/);
  });

  it('keeps structured score errors ahead of the unavailable message', () => {
    expect(
      errorMessage(
        new ApiError({ message: 'x', status: 503, code: 'scores_busy', requestId: 'r' }),
        'f'
      )
    ).toMatch(/Scoring is busy/);
    expect(
      errorMessage(
        new ApiError({ message: 'x', status: 503, code: 'scores_pending', requestId: 'r' }),
        'f'
      )
    ).toMatch(/not ready yet/);
  });
});
