import { isAxiosError } from 'axios';
import * as z from 'zod';

import type { ApiErrorData } from '@/types';

const errorBodySchema = z.object({
  error: z.object({
    code: z.string(),
    message: z.string(),
    request_id: z.string(),
  }),
});

export class ApiError extends Error {
  readonly status?: number;
  readonly code?: string;
  readonly requestId?: string;
  readonly details?: unknown;

  constructor({ message, status, code, requestId, details }: ApiErrorData) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
    this.code = code;
    this.requestId = requestId;
    this.details = details;
  }
}

export function isApiError(error: unknown): error is ApiError {
  return error instanceof ApiError;
}

export function toApiError(error: unknown): ApiError {
  if (isApiError(error)) return error;

  if (isAxiosError(error)) {
    const status = error.response?.status;
    const body = errorBodySchema.safeParse(error.response?.data);
    if (body.success) {
      const { code, message, request_id } = body.data.error;
      return new ApiError({ message, status, code, requestId: request_id });
    }
    // Proxies and network failures may have no API error body.
    return new ApiError({ message: error.message, status, code: error.code });
  }

  if (error instanceof z.ZodError) {
    return new ApiError({
      message: 'Validation error',
      code: 'VALIDATION_ERROR',
      details: error.issues,
    });
  }

  if (error instanceof Error) {
    return new ApiError({ message: error.message });
  }

  return new ApiError({ message: 'An unexpected error occurred' });
}
