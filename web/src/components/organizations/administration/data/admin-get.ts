import type { QueryClient } from '@tanstack/react-query';
import type { output as ZodOutput, ZodType } from 'zod';

import { isApiError, request } from '@/api';

import { accessKeys } from './keys';

interface Context {
  client: QueryClient;
  signal: AbortSignal;
}

export function administrationPath(login: string, suffix = '') {
  return `/v1/organizations/${encodeURIComponent(login)}/administration${suffix}`;
}

export async function adminGet<S extends ZodType>(
  { client, signal }: Context,
  login: string,
  suffix: string,
  schema: S,
  params?: Record<string, string | null | undefined>
): Promise<ZodOutput<S>> {
  try {
    return await request({
      url: administrationPath(login, suffix),
      params: { ...params, cursor: params?.cursor ?? undefined },
      signal,
      schema,
    });
  } catch (error) {
    // A denied read means the capability changed, so the gate must check it again.
    if (isApiError(error) && (error.status === 403 || error.status === 404)) {
      void client.invalidateQueries({ queryKey: accessKeys.detail(login) });
    }
    throw error;
  }
}
