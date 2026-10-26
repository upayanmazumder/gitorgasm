import { AxiosError, type AxiosAdapter } from 'axios';
import { describe, expect, it } from 'vitest';
import * as z from 'zod';

import { createApiClient } from '../client';
import { isApiError } from '../errors';
import { request } from '../request';

function clientReturning(status: number, data: unknown) {
  const client = createApiClient('/api');
  const adapter: AxiosAdapter = async config => {
    const response = { status, statusText: '', data, headers: {}, config };
    if (status >= 400) {
      throw new AxiosError('Request failed', 'ERR_BAD_RESPONSE', config, undefined, response);
    }
    return response;
  };
  client.defaults.adapter = adapter;
  return client;
}

const userSchema = z.object({ login: z.string() });

describe('request', () => {
  it('returns the parsed body when it matches the schema', async () => {
    const client = clientReturning(200, { login: 'octocat', extra: true });
    await expect(request({ url: '/v1/me', client, schema: userSchema })).resolves.toEqual({
      login: 'octocat',
    });
  });

  it('rejects with a validation ApiError when the body does not match the schema', async () => {
    const client = clientReturning(200, { login: 42 });
    const error = await request({ url: '/v1/me', client, schema: userSchema }).catch(e => e);
    expect(isApiError(error)).toBe(true);
    expect(error.code).toBe('VALIDATION_ERROR');
  });

  it('normalises error responses to ApiError', async () => {
    const client = clientReturning(401, {
      error: { code: 'unauthenticated', message: 'sign in required', request_id: 'r1' },
    });
    const error = await request({ url: '/v1/me', client }).catch(e => e);
    if (!isApiError(error)) throw new Error('Expected ApiError');
    expect(error.status).toBe(401);
    expect(error.code).toBe('unauthenticated');
  });
});
