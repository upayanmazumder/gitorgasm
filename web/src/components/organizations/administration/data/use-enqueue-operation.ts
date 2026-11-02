import { useMutation } from '@tanstack/react-query';

import { request } from '@/api';

import { administrationPath } from './admin-get';
import type { OperationRequest } from './request-schema';
import { operationSchema } from './schemas';

// The API checks the session, grant, and live owner state on GitHub first, so a timeout is uncertain.
const ENQUEUE_TIMEOUT_MS = 20_000;

export interface EnqueueVariables {
  idempotencyKey: string;
  body: OperationRequest;
}

// No signal and no retry: unmounting must not abort a write that may commit, and a resend is the user's call.
export function useEnqueueOperation(login: string) {
  return useMutation({
    mutationFn: async ({ idempotencyKey, body }: EnqueueVariables) =>
      (
        await request({
          method: 'POST',
          url: administrationPath(login, '/operations'),
          data: body,
          headers: { 'Idempotency-Key': idempotencyKey },
          timeout: ENQUEUE_TIMEOUT_MS,
          schema: operationSchema,
        })
      ).data,
    retry: 0,
  });
}
