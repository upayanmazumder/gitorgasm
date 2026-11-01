import { isApiError } from '@/api';

export function errorMessage(error: unknown, fallback: string): string {
  if (!isApiError(error)) return error instanceof Error ? error.message : fallback;
  switch (error.code) {
    case 'invalid_query':
      return 'The scoring options are not valid. Check the dates, filters, and rules.';
    case 'invalid_name':
      return 'Preset names need 1 to 80 printable characters.';
    case 'preset_limit':
      return 'This organization has the maximum number of presets. Remove one first.';
    case 'scores_pending':
      return 'Scores are not ready yet. Check again after the next update.';
    case 'scores_busy':
      return 'Scoring is busy. Try again in a moment.';
    case 'organization_not_found':
      return 'You do not have access to this organization.';
  }
  if (error.status === 401) return 'Your session ended. Sign in again.';
  return fallback;
}
