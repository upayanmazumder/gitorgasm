import { isAxiosError } from 'axios';
import * as z from 'zod';

import { ApiError, isApiError } from './api-error';

const errorBodySchema = z.object({
  error: z.object({
    code: z.string(),
    message: z.string(),
    request_id: z.string(),
  }),
});

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
