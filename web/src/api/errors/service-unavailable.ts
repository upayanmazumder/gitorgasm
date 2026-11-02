import { isApiError } from './api-error';

// The browser got no usable answer. ERR_CANCELED is a cancellation, not an outage.
const transportFailureCodes = new Set(['ERR_NETWORK', 'ECONNABORTED', 'ETIMEDOUT']);
// The Next.js proxy answers a refused API connection with a plain-text 500.
const gatewayStatuses = new Set([500, 502, 503, 504]);

export function isServiceUnavailable(error: unknown): boolean {
  if (!isApiError(error) || error.code === undefined) return false;
  if (transportFailureCodes.has(error.code)) return true;
  // toApiError keeps the axios code only when the body was not an API error envelope.
  return (
    error.code === 'ERR_BAD_RESPONSE' &&
    error.status !== undefined &&
    gatewayStatuses.has(error.status)
  );
}
