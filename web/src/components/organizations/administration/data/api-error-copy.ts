import { isApiError, isServiceUnavailable } from '@/api';

import { REAUTHENTICATION_CODE } from './classify-enqueue-error';

export interface ReadFailure {
  message: string;
  signIn: boolean;
  // True when the cause is only a failed or unreachable read, with no specific advice.
  generic?: boolean;
}

// Never returns ApiError.message: the text can carry upstream detail.
export function readFailure(
  error: unknown,
  subject: string,
  org: string,
  rateLimited = 'GitHub limited requests. Wait a minute, then try again.'
): ReadFailure {
  const status = isApiError(error) ? error.status : undefined;
  const code = isApiError(error) ? error.code : undefined;
  if (code === REAUTHENTICATION_CODE) {
    return {
      message: 'Your GitHub authorization expired. Sign in with GitHub again.',
      signIn: true,
    };
  }
  if (status === 401) return { message: 'Your session ended. Sign in again.', signIn: true };
  if (code === 'github_rate_limited' || status === 429) {
    return { message: rateLimited, signIn: false };
  }
  if (status === 404 || status === 403) {
    return { message: `Your access to ${org} changed.`, signIn: false };
  }
  if (isServiceUnavailable(error)) {
    return {
      message: `Could not load ${subject}. The service may be temporarily unavailable. Try again in a moment.`,
      signIn: false,
      generic: true,
    };
  }
  if (code === 'github_unavailable') {
    return {
      message: `Could not load ${subject}. GitHub did not answer. Try again in a moment.`,
      signIn: false,
      generic: true,
    };
  }
  return { message: `Could not load ${subject}. Try again.`, signIn: false, generic: true };
}
