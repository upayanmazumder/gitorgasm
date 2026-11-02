import { describe, expect, it } from 'vitest';

import { ApiError } from '@/api';

import { classifyEnqueueError } from '../classify-enqueue-error';

const uncertain = { kind: 'uncertain', rateLimited: false };

function failure(status: number | undefined, code?: string) {
  return new ApiError({ message: 'raw upstream text', status, code });
}

describe('classifyEnqueueError', () => {
  it.each([
    ['a network failure', failure(undefined, 'ERR_NETWORK')],
    ['a timeout', failure(undefined, 'ECONNABORTED')],
    ['an unreadable success body', failure(undefined, 'VALIDATION_ERROR')],
    ['a gateway error without an API envelope', failure(502, 'ERR_BAD_RESPONSE')],
    ['an internal error', failure(500, 'internal')],
    ['a non-API error', new Error('boom')],
  ])('treats %s as uncertain because the request may have committed', (_name, error) => {
    expect(classifyEnqueueError(error)).toEqual(uncertain);
  });

  it('flags an upstream rate limit on an uncertain response', () => {
    expect(classifyEnqueueError(failure(503, 'github_rate_limited'))).toEqual({
      kind: 'uncertain',
      rateLimited: true,
    });
  });

  it('asks for reauthorization before it reads the status', () => {
    expect(classifyEnqueueError(failure(401, 'github_reauthentication_required'))).toEqual({
      kind: 'rejected',
      rejection: 'reauthenticate',
    });
  });

  it.each([
    [400, undefined, 'invalid'],
    [401, 'unauthenticated', 'signed_out'],
    [403, 'origin_rejected', 'origin'],
    [403, 'forbidden', 'access_changed'],
    [404, 'not_found', 'access_changed'],
    [409, 'idempotency_conflict', 'conflict'],
    [429, 'rate_limited', 'too_many'],
  ])('rejects %i %s as %s', (status, code, rejection) => {
    expect(classifyEnqueueError(failure(status, code))).toEqual({ kind: 'rejected', rejection });
  });
});
