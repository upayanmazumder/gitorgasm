import { isApiError } from '@/api';

export const REAUTHENTICATION_CODE = 'github_reauthentication_required';

export type Rejection =
  | 'invalid'
  | 'conflict'
  | 'too_many'
  | 'origin'
  | 'access_changed'
  | 'signed_out'
  | 'reauthenticate';

export type EnqueueFailure =
  { kind: 'uncertain'; rateLimited: boolean } | { kind: 'rejected'; rejection: Rejection };

function rejected(rejection: Rejection): EnqueueFailure {
  return { kind: 'rejected', rejection };
}

// Only a definitive 4xx proves that the API did not accept the request. Anything else may have committed.
export function classifyEnqueueError(error: unknown): EnqueueFailure {
  if (!isApiError(error) || error.status === undefined) {
    return { kind: 'uncertain', rateLimited: false };
  }
  if (error.code === REAUTHENTICATION_CODE) return rejected('reauthenticate');
  if (error.status >= 500) {
    return { kind: 'uncertain', rateLimited: error.code === 'github_rate_limited' };
  }
  switch (error.status) {
    case 401:
      return rejected('signed_out');
    case 403:
      return rejected(error.code === 'origin_rejected' ? 'origin' : 'access_changed');
    case 404:
      return rejected('access_changed');
    case 409:
      return rejected('conflict');
    case 429:
      return rejected('too_many');
    default:
      return rejected('invalid');
  }
}
