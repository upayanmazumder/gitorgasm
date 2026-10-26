import { AxiosError, AxiosHeaders, type AxiosResponse } from 'axios';
import { describe, expect, it } from 'vitest';
import * as z from 'zod';

import { ApiError, toApiError } from '../errors';

function axiosErrorWith(status: number, data: unknown) {
  const response: AxiosResponse = {
    status,
    statusText: '',
    data,
    headers: {},
    config: { headers: new AxiosHeaders() },
  };
  return new AxiosError('Request failed', 'ERR_BAD_RESPONSE', undefined, undefined, response);
}

describe('toApiError', () => {
  it('passes ApiError through unchanged', () => {
    const e = new ApiError({ message: 'x' });
    expect(toApiError(e)).toBe(e);
  });

  it('reads the API error envelope', () => {
    const result = toApiError(
      axiosErrorWith(404, {
        error: { code: 'not_found', message: 'organization not found', request_id: 'req-1' },
      })
    );
    expect(result).toBeInstanceOf(ApiError);
    expect(result.status).toBe(404);
    expect(result.code).toBe('not_found');
    expect(result.message).toBe('organization not found');
    expect(result.requestId).toBe('req-1');
  });

  it('keeps the transport message when the body is not an API error', () => {
    const result = toApiError(axiosErrorWith(502, '<html>Bad Gateway</html>'));
    expect(result.status).toBe(502);
    expect(result.message).toBe('Request failed');
    expect(result.code).toBe('ERR_BAD_RESPONSE');
    expect(result.details).toBeUndefined();
  });

  it('converts AxiosError without response', () => {
    const result = toApiError(new AxiosError('Network Error', 'ERR_NETWORK'));
    expect(result.status).toBeUndefined();
    expect(result.message).toBe('Network Error');
    expect(result.code).toBe('ERR_NETWORK');
  });

  it('converts a ZodError produced by schema parsing', () => {
    const parsed = z.string().min(10).safeParse('x');
    if (parsed.success) throw new Error('expected parse failure');
    const result = toApiError(parsed.error);
    expect(result.code).toBe('VALIDATION_ERROR');
    expect(result.details).toEqual(parsed.error.issues);
  });

  it('converts a plain Error', () => {
    expect(toApiError(new Error('boom')).message).toBe('boom');
  });

  it('converts unknown values', () => {
    expect(toApiError('something weird').message).toBe('An unexpected error occurred');
  });
});
