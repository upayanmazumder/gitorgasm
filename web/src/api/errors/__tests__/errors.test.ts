import { AxiosError, AxiosHeaders, CanceledError, type AxiosResponse } from 'axios';
import { describe, expect, it } from 'vitest';
import * as z from 'zod';

import { ApiError, isServiceUnavailable, toApiError } from '..';

function axiosErrorWith(status: number, data: unknown) {
  const response: AxiosResponse = {
    status,
    statusText: '',
    data,
    headers: {},
    config: { headers: new AxiosHeaders() },
  };
  // Axios settle() gives 4xx ERR_BAD_REQUEST and every other rejected status ERR_BAD_RESPONSE.
  const code = status < 500 ? 'ERR_BAD_REQUEST' : 'ERR_BAD_RESPONSE';
  return new AxiosError('Request failed', code, undefined, undefined, response);
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

function envelope(code: string) {
  return { error: { code, message: 'x', request_id: 'req-1' } };
}

function zodFailure() {
  const parsed = z.string().min(10).safeParse('x');
  if (parsed.success) throw new Error('expected parse failure');
  return parsed.error;
}

describe('isServiceUnavailable', () => {
  it.each([
    ['a network failure', new AxiosError('Network Error', 'ERR_NETWORK'), true],
    ['a client timeout', new AxiosError('timeout of 10000ms exceeded', 'ECONNABORTED'), true],
    ['a clarified client timeout', new AxiosError('timeout', 'ETIMEDOUT'), true],
    [
      'the Next.js proxy 500 for a refused connection',
      axiosErrorWith(500, 'Internal Server Error'),
      true,
    ],
    ['a 502 gateway page', axiosErrorWith(502, '<html>Bad Gateway</html>'), true],
    ['a 503 with an empty body', axiosErrorWith(503, ''), true],
    ['a 504 gateway page', axiosErrorWith(504, '<html>Gateway Timeout</html>'), true],
    ['a cancellation', new CanceledError(), false],
    ['a 503 scores_busy envelope', axiosErrorWith(503, envelope('scores_busy')), false],
    ['a 503 scores_pending envelope', axiosErrorWith(503, envelope('scores_pending')), false],
    [
      'a 502 github_unavailable envelope',
      axiosErrorWith(502, envelope('github_unavailable')),
      false,
    ],
    ['a 500 internal envelope', axiosErrorWith(500, envelope('internal')), false],
    ['a 401 unauthenticated envelope', axiosErrorWith(401, envelope('unauthenticated')), false],
    ['a 404 page without an envelope', axiosErrorWith(404, '<html>Not Found</html>'), false],
    ['a response validation failure', zodFailure(), false],
    ['a plain Error', new Error('boom'), false],
  ])('classifies %s', (_name, input, expected) => {
    expect(isServiceUnavailable(toApiError(input))).toBe(expected);
  });
});
